// Package music looks up YouTube videos for the music player's playlist.
package music

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalidURL     = errors.New("not a YouTube video link")
	ErrVideoNotFound  = errors.New("video not found (it may be private or deleted)")
	ErrNotEmbeddable  = errors.New("the owner doesn't allow this video to be played outside YouTube")
	videoIDPattern    = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	pathVideoPrefixes = []string{"/shorts/", "/embed/", "/live/", "/v/"}
)

// ParseVideoID extracts the 11-character video ID from a YouTube link, or accepts a bare ID.
// Handles watch, youtu.be, shorts, embed, live, and music.youtube.com links, with any extra
// parameters (playlist, timestamp, share tracking).
func ParseVideoID(input string) (string, error) {
	input = strings.TrimSpace(input)
	if videoIDPattern.MatchString(input) {
		return input, nil
	}
	if !strings.Contains(input, "://") {
		input = "https://" + input
	}
	u, err := url.Parse(input)
	if err != nil {
		return "", ErrInvalidURL
	}

	var id string
	switch strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.") {
	case "youtu.be":
		id = strings.Trim(u.Path, "/")
	case "youtube.com", "m.youtube.com", "music.youtube.com", "youtube-nocookie.com":
		if u.Path == "/watch" {
			id = u.Query().Get("v")
			break
		}
		for _, prefix := range pathVideoPrefixes {
			if rest, ok := strings.CutPrefix(u.Path, prefix); ok {
				id, _, _ = strings.Cut(rest, "/")
			}
		}
	}
	if !videoIDPattern.MatchString(id) {
		return "", ErrInvalidURL
	}
	return id, nil
}

// Video is what the playlist stores about a song.
type Video struct {
	Title        string
	ChannelName  string
	ThumbnailURL string
}

// ThumbnailURL returns a video's 320x180 (16:9) thumbnail.
func ThumbnailURL(videoID string) string {
	return "https://i.ytimg.com/vi/" + videoID + "/mqdefault.jpg"
}

// OEmbedClient looks videos up through YouTube's oEmbed endpoint, which needs no API key and
// costs no quota. It also reports whether a video may be embedded, which the player needs.
type OEmbedClient struct {
	HTTP    *http.Client
	BaseURL string
}

func NewOEmbedClient() *OEmbedClient {
	return &OEmbedClient{
		HTTP:    &http.Client{Timeout: 10 * time.Second},
		BaseURL: "https://www.youtube.com/oembed",
	}
}

func (c *OEmbedClient) Lookup(ctx context.Context, videoID string) (*Video, error) {
	q := url.Values{
		"format": {"json"},
		"url":    {"https://www.youtube.com/watch?v=" + videoID},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("youtube lookup: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("youtube lookup: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrNotEmbeddable
	case http.StatusBadRequest, http.StatusNotFound:
		return nil, ErrVideoNotFound
	default:
		return nil, fmt.Errorf("youtube lookup: status %d", resp.StatusCode)
	}

	var r struct {
		Title      string `json:"title"`
		AuthorName string `json:"author_name"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode youtube lookup: %w", err)
	}
	return &Video{Title: r.Title, ChannelName: r.AuthorName, ThumbnailURL: ThumbnailURL(videoID)}, nil
}
