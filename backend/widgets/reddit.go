package widgets

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	RedditWidgetType = "reddit"
	RedditTTL        = 20 * time.Minute
	redditPostLimit  = 10
	// Used when Reddit returns 429 without saying when the limit resets.
	redditDefaultBackoff = time.Minute
	// Added to Reddit's reset time: requests sent exactly at the reset still get a 429.
	redditResetMargin = 2 * time.Second
	// Reddit rejects requests without a descriptive User-Agent.
	redditUserAgent = "linux:personal-dashboard:v0.1 (self-hosted dashboard)"
)

// RedditPost is bound to the GraphQL RedditPost type. Score and CommentCount are nil
// when posts come from the RSS feed, which does not include them.
type RedditPost struct {
	Title        string `json:"title"`
	Score        *int   `json:"score"`
	CommentCount *int   `json:"commentCount"`
	URL          string `json:"url"`
	Author       string `json:"author"`
	PublishedAt  string `json:"publishedAt"`
}

// SubredditFeed is bound to the GraphQL SubredditFeed type.
type SubredditFeed struct {
	Subreddit string        `json:"subreddit"`
	Posts     []*RedditPost `json:"posts"`
}

// ErrRateLimited is returned while Reddit's rate limit is exhausted. ReadyAt says when it resets.
var ErrRateLimited = errors.New("rate limited by reddit")

var subredditPattern = regexp.MustCompile(`^[A-Za-z0-9_]{2,21}$`)

// NormalizeSubreddit strips an optional "r/" prefix, lowercases the name, and validates it.
func NormalizeSubreddit(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(strings.TrimPrefix(name, "/"), "r/")
	if !subredditPattern.MatchString(name) {
		return "", fmt.Errorf("invalid subreddit name %q", name)
	}
	return strings.ToLower(name), nil
}

// RedditCacheKey identifies one subreddit's top posts, e.g. "reddit_golang".
func RedditCacheKey(subreddit string) string {
	return "reddit_" + subreddit
}

// ParseRedditConfig returns the normalized subreddits from a reddit widget's config.
func ParseRedditConfig(cfg map[string]any) ([]string, error) {
	raw, _ := cfg["subreddits"].([]any)
	subs := make([]string, 0, len(raw))
	for _, v := range raw {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("subreddits must be strings")
		}
		name, err := NormalizeSubreddit(s)
		if err != nil {
			return nil, err
		}
		subs = append(subs, name)
	}
	return subs, nil
}

// RedditClient fetches a subreddit's top posts of the day. With OAuth credentials it uses
// the official JSON API; without them it falls back to the public RSS feed, because Reddit
// blocks anonymous requests to its .json endpoints.
type RedditClient struct {
	HTTP         *http.Client
	ClientID     string
	ClientSecret string
	// Base URLs are fields so tests can point them at an httptest server.
	WebURL   string
	OAuthURL string
	// readyAt is when Reddit's rate limit window resets, taken from its x-ratelimit-* headers.
	// Anonymous RSS access allows roughly one request per window.
	limitMu sync.Mutex
	readyAt time.Time

	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
}

func NewRedditClient(clientID, clientSecret string) *RedditClient {
	return &RedditClient{
		HTTP:         &http.Client{Timeout: 10 * time.Second},
		ClientID:     clientID,
		ClientSecret: clientSecret,
		WebURL:       "https://www.reddit.com",
		OAuthURL:     "https://oauth.reddit.com",
	}
}

func (c *RedditClient) UsesOAuth() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}

// Fetch returns the top posts of the day for one subreddit. subreddit must already be normalized.
func (c *RedditClient) Fetch(ctx context.Context, subreddit string) (*SubredditFeed, error) {
	var posts []*RedditPost
	var err error
	if c.UsesOAuth() {
		posts, err = c.fetchJSON(ctx, subreddit)
	} else {
		posts, err = c.fetchRSS(ctx, subreddit)
	}
	if err != nil {
		return nil, fmt.Errorf("r/%s: %w", subreddit, err)
	}
	return &SubredditFeed{Subreddit: subreddit, Posts: posts}, nil
}

func (c *RedditClient) fetchRSS(ctx context.Context, subreddit string) ([]*RedditPost, error) {
	u := fmt.Sprintf("%s/r/%s/top.rss?t=day&limit=%d", c.WebURL, subreddit, redditPostLimit)
	body, err := c.get(ctx, u, "")
	if err != nil {
		return nil, err
	}
	return ParseRedditRSS(body)
}

func (c *RedditClient) fetchJSON(ctx context.Context, subreddit string) ([]*RedditPost, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/r/%s/top?t=day&limit=%d&raw_json=1", c.OAuthURL, subreddit, redditPostLimit)
	body, err := c.get(ctx, u, token)
	if err != nil {
		return nil, err
	}
	return ParseRedditListing(body)
}

func (c *RedditClient) get(ctx context.Context, u, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", redditUserAgent)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return c.do(req)
}

// ReadyAt returns when the next request is allowed. It is in the past when requests can go out now.
func (c *RedditClient) ReadyAt() time.Time {
	c.limitMu.Lock()
	defer c.limitMu.Unlock()
	return c.readyAt
}

// recordRateLimit updates readyAt from a response. Requests fail fast with ErrRateLimited
// until then, so a page load never blocks waiting on Reddit.
func (c *RedditClient) recordRateLimit(resp *http.Response) {
	remaining, errRemaining := strconv.ParseFloat(resp.Header.Get("x-ratelimit-remaining"), 64)
	reset, errReset := strconv.Atoi(resp.Header.Get("x-ratelimit-reset"))

	var wait time.Duration
	switch {
	case errReset == nil && (resp.StatusCode == http.StatusTooManyRequests || errRemaining == nil && remaining < 1):
		wait = time.Duration(reset)*time.Second + redditResetMargin
	case resp.StatusCode == http.StatusTooManyRequests:
		wait = redditDefaultBackoff
	default:
		return
	}

	c.limitMu.Lock()
	defer c.limitMu.Unlock()
	if until := time.Now().Add(wait); until.After(c.readyAt) {
		c.readyAt = until
	}
}

func (c *RedditClient) do(req *http.Request) ([]byte, error) {
	if time.Now().Before(c.ReadyAt()) {
		return nil, ErrRateLimited
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	c.recordRateLimit(resp)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, ErrRateLimited
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("subreddit not found")
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("reddit: status %d", resp.StatusCode)
	}
	return body, nil
}

// accessToken returns a cached application-only OAuth token, requesting a new one when it expires.
func (c *RedditClient) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExpiry) {
		return c.token, nil
	}

	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.WebURL+"/api/v1/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.ClientID, c.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", redditUserAgent)
	body, err := c.do(req)
	if err != nil {
		return "", fmt.Errorf("oauth token: %w", err)
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("oauth token: unexpected response")
	}
	c.token = tok.AccessToken
	// Refresh a minute early so a token never expires mid-request.
	c.tokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn)*time.Second - time.Minute)
	return c.token, nil
}

type redditAtomFeed struct {
	Entries []struct {
		Title  string `xml:"title"`
		Author struct {
			Name string `xml:"name"`
		} `xml:"author"`
		Link struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Published string `xml:"published"`
	} `xml:"entry"`
}

// ParseRedditRSS converts a subreddit's Atom feed into posts.
func ParseRedditRSS(body []byte) ([]*RedditPost, error) {
	var feed redditAtomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("decode reddit feed: %w", err)
	}
	posts := make([]*RedditPost, 0, len(feed.Entries))
	for _, e := range feed.Entries {
		posts = append(posts, &RedditPost{
			Title:       e.Title,
			URL:         e.Link.Href,
			Author:      strings.TrimPrefix(e.Author.Name, "/u/"),
			PublishedAt: e.Published,
		})
	}
	return posts, nil
}

type redditListing struct {
	Data struct {
		Children []struct {
			Data struct {
				Title       string  `json:"title"`
				Author      string  `json:"author"`
				Score       int     `json:"score"`
				NumComments int     `json:"num_comments"`
				Permalink   string  `json:"permalink"`
				CreatedUTC  float64 `json:"created_utc"`
				Stickied    bool    `json:"stickied"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

// ParseRedditListing converts a Reddit JSON API listing into posts, skipping pinned posts.
func ParseRedditListing(body []byte) ([]*RedditPost, error) {
	var listing redditListing
	if err := json.Unmarshal(body, &listing); err != nil {
		return nil, fmt.Errorf("decode reddit listing: %w", err)
	}
	posts := make([]*RedditPost, 0, len(listing.Data.Children))
	for _, child := range listing.Data.Children {
		d := child.Data
		if d.Stickied {
			continue
		}
		score, comments := d.Score, d.NumComments
		posts = append(posts, &RedditPost{
			Title:        d.Title,
			Score:        &score,
			CommentCount: &comments,
			URL:          "https://www.reddit.com" + d.Permalink,
			Author:       d.Author,
			PublishedAt:  time.Unix(int64(d.CreatedUTC), 0).UTC().Format(time.RFC3339),
		})
	}
	return posts, nil
}
