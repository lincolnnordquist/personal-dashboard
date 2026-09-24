package widgets

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRedditRSS(t *testing.T) {
	body, err := os.ReadFile("testdata/reddit_top.rss")
	require.NoError(t, err)

	posts, err := ParseRedditRSS(body)
	require.NoError(t, err)
	require.Len(t, posts, 2)

	assert.Equal(t, &RedditPost{
		Title:       "Always Active: Faking Microsoft Teams Green Status in Go",
		URL:         "https://www.reddit.com/r/golang/comments/1wo3y4m/always_active_faking_microsoft_teams_green_status/",
		Author:      "derjanni",
		PublishedAt: "2026-09-23T11:57:31+00:00",
	}, posts[0])
	assert.Equal(t, "Generics vs interfaces & when to use which?", posts[1].Title, "XML entities are decoded")
	assert.Nil(t, posts[1].Score, "RSS has no score")
	assert.Nil(t, posts[1].CommentCount, "RSS has no comment count")
}

func TestParseRedditRSSInvalid(t *testing.T) {
	_, err := ParseRedditRSS([]byte("<html>whoa there, pardner!"))
	assert.Error(t, err)
}

func TestParseRedditListing(t *testing.T) {
	body, err := os.ReadFile("testdata/reddit_top.json")
	require.NoError(t, err)

	posts, err := ParseRedditListing(body)
	require.NoError(t, err)
	require.Len(t, posts, 2)

	first := posts[0]
	assert.Equal(t, "Always Active: Faking Microsoft Teams Green Status in Go", first.Title)
	assert.Equal(t, "derjanni", first.Author)
	assert.Equal(t, "https://www.reddit.com/r/golang/comments/1wo3y4m/always_active_faking_microsoft_teams_green_status/", first.URL)
	assert.Equal(t, "2026-09-23T11:57:31Z", first.PublishedAt)
	require.NotNil(t, first.Score)
	assert.Equal(t, 214, *first.Score)
	require.NotNil(t, first.CommentCount)
	assert.Equal(t, 37, *first.CommentCount)

	// A score of zero is real data, not missing data.
	require.NotNil(t, posts[1].Score)
	assert.Equal(t, 0, *posts[1].Score)
}

func TestParseRedditListingSkipsStickied(t *testing.T) {
	body := []byte(`{"data":{"children":[
		{"data":{"title":"Weekly thread","stickied":true}},
		{"data":{"title":"Real post","stickied":false}}
	]}}`)
	posts, err := ParseRedditListing(body)
	require.NoError(t, err)
	require.Len(t, posts, 1)
	assert.Equal(t, "Real post", posts[0].Title)
}

func TestNormalizeSubreddit(t *testing.T) {
	for in, want := range map[string]string{
		"golang":       "golang",
		"r/SelfHosted": "selfhosted",
		"/r/Go_Lang ":  "go_lang",
	} {
		got, err := NormalizeSubreddit(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, bad := range []string{"", "a", "golang/../x", "has space", "waytoolongsubredditnamehere"} {
		_, err := NormalizeSubreddit(bad)
		assert.Error(t, err, bad)
	}
}

func TestRedditClientFetchUsesRSSWithoutCredentials(t *testing.T) {
	fixture, err := os.ReadFile("testdata/reddit_top.rss")
	require.NoError(t, err)

	var gotPath, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotUA = r.URL.Path, r.UserAgent()
		w.Write(fixture)
	}))
	defer srv.Close()

	c := NewRedditClient("", "")
	c.WebURL = srv.URL
	feed, err := c.Fetch(context.Background(), "golang")
	require.NoError(t, err)

	assert.Equal(t, "/r/golang/top.rss", gotPath)
	assert.Equal(t, redditUserAgent, gotUA)
	assert.Equal(t, "golang", feed.Subreddit)
	assert.Len(t, feed.Posts, 2)
}

func TestRedditClientFetchUsesOAuthWithCredentials(t *testing.T) {
	fixture, err := os.ReadFile("testdata/reddit_top.json")
	require.NoError(t, err)

	tokenRequests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/access_token":
			tokenRequests++
			user, pass, ok := r.BasicAuth()
			assert.True(t, ok)
			assert.Equal(t, "id", user)
			assert.Equal(t, "secret", pass)
			w.Write([]byte(`{"access_token":"tok123","token_type":"bearer","expires_in":86400}`))
		case "/r/golang/top":
			assert.Equal(t, "Bearer tok123", r.Header.Get("Authorization"))
			w.Write(fixture)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewRedditClient("id", "secret")
	c.WebURL, c.OAuthURL = srv.URL, srv.URL

	for range 2 {
		feed, err := c.Fetch(context.Background(), "golang")
		require.NoError(t, err)
		require.Len(t, feed.Posts, 2)
		assert.Equal(t, 214, *feed.Posts[0].Score)
	}
	assert.Equal(t, 1, tokenRequests, "token is cached between requests")
}

func TestRedditClientFailsFastWhenLimitExhausted(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("x-ratelimit-remaining", "0.0")
		w.Header().Set("x-ratelimit-reset", "35")
		w.Write([]byte(`<feed xmlns="http://www.w3.org/2005/Atom"></feed>`))
	}))
	defer srv.Close()

	c := NewRedditClient("", "")
	c.WebURL = srv.URL

	_, err := c.Fetch(context.Background(), "golang")
	require.NoError(t, err, "the last request in a window still succeeds")
	assert.WithinDuration(t, time.Now().Add(35*time.Second+redditResetMargin), c.ReadyAt(), time.Second)

	_, err = c.Fetch(context.Background(), "selfhosted")
	assert.ErrorIs(t, err, ErrRateLimited)
	assert.Equal(t, 1, requests, "no request is sent while the limit is exhausted")
}

func TestRedditClientBacksOffAfter429AtWindowEdge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-reset", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := NewRedditClient("", "")
	c.WebURL = srv.URL
	_, err := c.Fetch(context.Background(), "golang")
	assert.ErrorIs(t, err, ErrRateLimited)
	assert.True(t, c.ReadyAt().After(time.Now().Add(time.Second)), "a reset of 0 still backs off by the margin")
}

func TestRedditClientRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := NewRedditClient("", "")
	c.WebURL = srv.URL
	_, err := c.Fetch(context.Background(), "golang")
	assert.ErrorIs(t, err, ErrRateLimited)
	assert.WithinDuration(t, time.Now().Add(redditDefaultBackoff), c.ReadyAt(), 2*time.Second,
		"a 429 without headers backs off for the default duration")
}
