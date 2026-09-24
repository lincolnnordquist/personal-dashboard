package widgets

import (
	"context"
	"encoding/json"
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
	YouTubeWidgetType = "youtube"
	YouTubeTTL        = 60 * time.Minute
	// Fetch extra uploads per channel so some remain after Shorts are filtered out.
	youtubeUploadsFetched = 15
	youtubeVideosKept     = 10
	// Videos this short or shorter are treated as Shorts and hidden.
	youtubeShortsMaxSeconds = 180
)

var ErrYouTubeNoKey = errors.New("youtube: YOUTUBE_API_KEY is not set")

// YoutubeChannel is bound to the GraphQL YoutubeChannel type.
type YoutubeChannel struct {
	ChannelID   string          `json:"channelId"`
	ChannelName string          `json:"channelName"`
	Handle      *string         `json:"handle"`
	Videos      []*YoutubeVideo `json:"videos"`
}

// YoutubeVideo is bound to the GraphQL YoutubeVideo type.
type YoutubeVideo struct {
	Title           string   `json:"title"`
	VideoID         string   `json:"videoId"`
	PublishedAt     string   `json:"publishedAt"`
	ThumbnailURL    string   `json:"thumbnailUrl"`
	DurationSeconds int      `json:"durationSeconds"`
	ViewCount       *float64 `json:"viewCount"`
}

var (
	youtubeHandlePattern    = regexp.MustCompile(`^@[A-Za-z0-9._-]{3,30}$`)
	youtubeChannelIDPattern = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)
)

// NormalizeYouTubeChannel accepts an @handle (the @ is optional) or a channel ID ("UC…").
// Handles are case-insensitive, so they are lowercased; channel IDs are case-sensitive.
func NormalizeYouTubeChannel(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if youtubeChannelIDPattern.MatchString(ref) {
		return ref, nil
	}
	if !strings.HasPrefix(ref, "@") {
		ref = "@" + ref
	}
	if !youtubeHandlePattern.MatchString(ref) {
		return "", fmt.Errorf("invalid YouTube channel %q: use an @handle or a channel ID", ref)
	}
	return strings.ToLower(ref), nil
}

// YouTubeCacheKey identifies one channel's latest uploads, e.g. "youtube_@mkbhd".
func YouTubeCacheKey(ref string) string {
	return "youtube_" + ref
}

// ParseYouTubeConfig returns the normalized channels from a youtube widget's config.
func ParseYouTubeConfig(cfg map[string]any) ([]string, error) {
	raw, _ := cfg["channelIds"].([]any)
	refs := make([]string, 0, len(raw))
	for _, v := range raw {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("channelIds must be strings")
		}
		ref, err := NormalizeYouTubeChannel(s)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// YouTubeClient reads channel uploads from the YouTube Data API v3.
type YouTubeClient struct {
	HTTP    *http.Client
	APIKey  string
	BaseURL string

	// Channel lookups never change, so they are remembered for the life of the process.
	mu       sync.Mutex
	channels map[string]youtubeChannelInfo
}

type youtubeChannelInfo struct {
	id, name, handle, uploads string
}

func NewYouTubeClient(apiKey string) *YouTubeClient {
	return &YouTubeClient{
		HTTP:     &http.Client{Timeout: 10 * time.Second},
		APIKey:   apiKey,
		BaseURL:  "https://www.googleapis.com/youtube/v3",
		channels: map[string]youtubeChannelInfo{},
	}
}

// Fetch returns a channel's latest uploads, newest first, without Shorts or live streams.
// ref must already be normalized.
func (c *YouTubeClient) Fetch(ctx context.Context, ref string) (*YoutubeChannel, error) {
	if c.APIKey == "" {
		return nil, ErrYouTubeNoKey
	}
	info, err := c.resolve(ctx, ref)
	if err != nil {
		return nil, err
	}

	body, err := c.get(ctx, "playlistItems", url.Values{
		"part":       {"snippet,contentDetails"},
		"playlistId": {info.uploads},
		"maxResults": {strconv.Itoa(youtubeUploadsFetched)},
	})
	if err != nil {
		return nil, fmt.Errorf("%s uploads: %w", ref, err)
	}
	videos, err := ParseYouTubePlaylistItems(body)
	if err != nil {
		return nil, err
	}

	channel := &YoutubeChannel{ChannelID: info.id, ChannelName: info.name, Videos: []*YoutubeVideo{}}
	if info.handle != "" {
		channel.Handle = &info.handle
	}
	if len(videos) == 0 {
		return channel, nil
	}

	ids := make([]string, len(videos))
	for i, v := range videos {
		ids[i] = v.VideoID
	}
	body, err = c.get(ctx, "videos", url.Values{
		"part": {"contentDetails,statistics,snippet"},
		"id":   {strings.Join(ids, ",")},
	})
	if err != nil {
		return nil, fmt.Errorf("%s video details: %w", ref, err)
	}
	details, err := ParseYouTubeVideoDetails(body)
	if err != nil {
		return nil, err
	}
	channel.Videos = ApplyYouTubeVideoDetails(videos, details, youtubeVideosKept)
	return channel, nil
}

// resolve looks up a channel's ID, name, and uploads playlist, once per process.
func (c *YouTubeClient) resolve(ctx context.Context, ref string) (youtubeChannelInfo, error) {
	c.mu.Lock()
	info, ok := c.channels[ref]
	c.mu.Unlock()
	if ok {
		return info, nil
	}

	params := url.Values{"part": {"snippet,contentDetails"}}
	if strings.HasPrefix(ref, "@") {
		params.Set("forHandle", ref)
	} else {
		params.Set("id", ref)
	}
	body, err := c.get(ctx, "channels", params)
	if err != nil {
		return info, fmt.Errorf("%s: %w", ref, err)
	}
	info, err = parseYouTubeChannel(body)
	if err != nil {
		return info, fmt.Errorf("%s: %w", ref, err)
	}

	c.mu.Lock()
	c.channels[ref] = info
	c.mu.Unlock()
	return info, nil
}

// get calls one API endpoint. The key goes in a header rather than the URL so it can never
// appear in a logged URL or an error message.
func (c *YouTubeClient) get(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/"+endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Goog-Api-Key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("youtube request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("youtube read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
			// Messages sometimes contain HTML, e.g. "<code>playlistId</code>".
			return nil, fmt.Errorf("youtube: %s", htmlTagPattern.ReplaceAllString(apiErr.Error.Message, ""))
		}
		return nil, fmt.Errorf("youtube: status %d", resp.StatusCode)
	}
	return body, nil
}

func parseYouTubeChannel(body []byte) (youtubeChannelInfo, error) {
	var r struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title     string `json:"title"`
				CustomURL string `json:"customUrl"`
			} `json:"snippet"`
			ContentDetails struct {
				RelatedPlaylists struct {
					Uploads string `json:"uploads"`
				} `json:"relatedPlaylists"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return youtubeChannelInfo{}, fmt.Errorf("decode channel: %w", err)
	}
	if len(r.Items) == 0 {
		return youtubeChannelInfo{}, fmt.Errorf("channel not found")
	}
	ch := r.Items[0]
	if ch.ContentDetails.RelatedPlaylists.Uploads == "" {
		return youtubeChannelInfo{}, fmt.Errorf("channel has no uploads playlist")
	}
	return youtubeChannelInfo{
		id:      ch.ID,
		name:    ch.Snippet.Title,
		handle:  ch.Snippet.CustomURL,
		uploads: ch.ContentDetails.RelatedPlaylists.Uploads,
	}, nil
}

// ParseYouTubePlaylistItems converts an uploads playlist page into videos, newest first.
// Durations and view counts come later from the videos endpoint.
func ParseYouTubePlaylistItems(body []byte) ([]*YoutubeVideo, error) {
	var r struct {
		Items []struct {
			Snippet struct {
				PublishedAt string `json:"publishedAt"`
				Title       string `json:"title"`
				Thumbnails  map[string]struct {
					URL string `json:"url"`
				} `json:"thumbnails"`
			} `json:"snippet"`
			ContentDetails struct {
				VideoID          string `json:"videoId"`
				VideoPublishedAt string `json:"videoPublishedAt"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode playlist items: %w", err)
	}
	videos := make([]*YoutubeVideo, 0, len(r.Items))
	for _, it := range r.Items {
		// Deleted and private videos stay in the playlist but have no publish date.
		if it.ContentDetails.VideoID == "" || it.ContentDetails.VideoPublishedAt == "" {
			continue
		}
		thumb := ""
		// medium is 320x180 (16:9); high is 480x360 with letterboxing.
		for _, size := range []string{"medium", "high", "default"} {
			if t, ok := it.Snippet.Thumbnails[size]; ok && t.URL != "" {
				thumb = t.URL
				break
			}
		}
		videos = append(videos, &YoutubeVideo{
			Title:        it.Snippet.Title,
			VideoID:      it.ContentDetails.VideoID,
			PublishedAt:  it.ContentDetails.VideoPublishedAt,
			ThumbnailURL: thumb,
		})
	}
	return videos, nil
}

// YouTubeVideoDetails is what the videos endpoint adds to a playlist item.
type YouTubeVideoDetails struct {
	DurationSeconds int
	ViewCount       *float64
	// Live is true for live streams and upcoming premieres, which have no fixed length yet.
	Live bool
}

// ParseYouTubeVideoDetails maps video IDs to their length, views, and live status.
func ParseYouTubeVideoDetails(body []byte) (map[string]YouTubeVideoDetails, error) {
	var r struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				LiveBroadcastContent string `json:"liveBroadcastContent"`
			} `json:"snippet"`
			ContentDetails struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
			Statistics struct {
				// View counts are strings because they can exceed 32-bit integers.
				ViewCount string `json:"viewCount"`
			} `json:"statistics"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode video details: %w", err)
	}
	details := make(map[string]YouTubeVideoDetails, len(r.Items))
	for _, it := range r.Items {
		d := YouTubeVideoDetails{Live: it.Snippet.LiveBroadcastContent != "" && it.Snippet.LiveBroadcastContent != "none"}
		if secs, err := ParseISODuration(it.ContentDetails.Duration); err == nil {
			d.DurationSeconds = secs
		}
		if views, err := strconv.ParseFloat(it.Statistics.ViewCount, 64); err == nil {
			d.ViewCount = &views
		}
		details[it.ID] = d
	}
	return details, nil
}

// ApplyYouTubeVideoDetails fills in lengths and view counts, drops Shorts, live streams, and
// videos the details lookup did not return, and keeps at most limit videos.
func ApplyYouTubeVideoDetails(videos []*YoutubeVideo, details map[string]YouTubeVideoDetails, limit int) []*YoutubeVideo {
	kept := make([]*YoutubeVideo, 0, min(limit, len(videos)))
	for _, v := range videos {
		d, ok := details[v.VideoID]
		if !ok || d.Live || d.DurationSeconds <= youtubeShortsMaxSeconds {
			continue
		}
		v.DurationSeconds = d.DurationSeconds
		v.ViewCount = d.ViewCount
		kept = append(kept, v)
		if len(kept) == limit {
			break
		}
	}
	return kept
}

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

var isoDurationPattern = regexp.MustCompile(`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)

// ParseISODuration converts an ISO 8601 duration such as "PT1H2M3S" to seconds.
func ParseISODuration(s string) (int, error) {
	m := isoDurationPattern.FindStringSubmatch(s)
	if m == nil || s == "P" || s == "PT" {
		return 0, fmt.Errorf("invalid ISO 8601 duration %q", s)
	}
	total := 0
	for i, unit := range []int{86400, 3600, 60, 1} {
		if m[i+1] != "" {
			n, _ := strconv.Atoi(m[i+1])
			total += n * unit
		}
	}
	return total, nil
}
