package widgets

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseISODuration(t *testing.T) {
	valid := map[string]int{
		"PT15M6S":   906,
		"PT58S":     58,
		"PT1H2M3S":  3723,
		"PT2H":      7200,
		"P1DT1S":    86401,
		"P0D":       0,
		"PT10M":     600,
		"PT1H0M30S": 3630,
	}
	for in, want := range valid {
		got, err := ParseISODuration(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, bad := range []string{"", "P", "PT", "15:06", "PT1.5S"} {
		_, err := ParseISODuration(bad)
		assert.Error(t, err, bad)
	}
}

func TestNormalizeYouTubeChannel(t *testing.T) {
	for in, want := range map[string]string{
		"@mkbhd":                   "@mkbhd",
		"MKBHD":                    "@mkbhd",
		" @LinusTechTips ":         "@linustechtips",
		"UCBJycsmduvYEL83R_U4JriQ": "UCBJycsmduvYEL83R_U4JriQ",
	} {
		got, err := NormalizeYouTubeChannel(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, bad := range []string{"", "@a", "has space", "@../../etc", "https://youtube.com/@mkbhd"} {
		_, err := NormalizeYouTubeChannel(bad)
		assert.Error(t, err, bad)
	}
}

func TestParseYouTubePlaylistItems(t *testing.T) {
	videos, err := ParseYouTubePlaylistItems(readFixture(t, "youtube_playlist_items.json"))
	require.NoError(t, err)
	require.Len(t, videos, 4)
	assert.Equal(t, &YoutubeVideo{
		Title:        "The Apple Watch Has a Problem",
		VideoID:      "pOX1l1edBME",
		PublishedAt:  "2026-09-22T17:49:42Z",
		ThumbnailURL: "https://i.ytimg.com/vi/pOX1l1edBME/mqdefault.jpg",
	}, videos[0])
}

func TestParseYouTubePlaylistItemsSkipsDeletedVideos(t *testing.T) {
	body := []byte(`{"items":[
		{"snippet":{"title":"Deleted video"},"contentDetails":{"videoId":"abc"}},
		{"snippet":{"title":"Real"},"contentDetails":{"videoId":"def","videoPublishedAt":"2026-09-01T00:00:00Z"}}
	]}`)
	videos, err := ParseYouTubePlaylistItems(body)
	require.NoError(t, err)
	require.Len(t, videos, 1)
	assert.Equal(t, "Real", videos[0].Title)
	assert.Empty(t, videos[0].ThumbnailURL, "missing thumbnails leave the URL empty")
}

func TestApplyYouTubeVideoDetailsFiltersShorts(t *testing.T) {
	videos, err := ParseYouTubePlaylistItems(readFixture(t, "youtube_playlist_items.json"))
	require.NoError(t, err)
	details, err := ParseYouTubeVideoDetails(readFixture(t, "youtube_videos.json"))
	require.NoError(t, err)

	kept := ApplyYouTubeVideoDetails(videos, details, 10)
	var ids []string
	for _, v := range kept {
		ids = append(ids, v.VideoID)
	}
	// The fixture's second video is 58 seconds long, so it is a Short.
	assert.Equal(t, []string{"pOX1l1edBME", "6D__H_DO2Xk", "Od6M0AXpcxQ"}, ids)
	assert.Equal(t, 906, kept[0].DurationSeconds)
	require.NotNil(t, kept[0].ViewCount)
	assert.Equal(t, 3780126.0, *kept[0].ViewCount)

	assert.Len(t, ApplyYouTubeVideoDetails(videos, details, 2), 2, "limit caps the result")
}

func TestApplyYouTubeVideoDetailsDropsLiveAndUnknown(t *testing.T) {
	videos := []*YoutubeVideo{{VideoID: "live"}, {VideoID: "missing"}, {VideoID: "ok"}}
	details := map[string]YouTubeVideoDetails{
		"live": {DurationSeconds: 3600, Live: true},
		"ok":   {DurationSeconds: 600},
	}
	kept := ApplyYouTubeVideoDetails(videos, details, 10)
	require.Len(t, kept, 1)
	assert.Equal(t, "ok", kept[0].VideoID)
	assert.Nil(t, kept[0].ViewCount, "hidden view counts stay null")
}

func TestYouTubeClientFetch(t *testing.T) {
	channels := readFixture(t, "youtube_channels.json")
	playlist := readFixture(t, "youtube_playlist_items.json")
	videos := readFixture(t, "youtube_videos.json")
	calls := map[string]int{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-key", r.Header.Get("X-Goog-Api-Key"))
		assert.Empty(t, r.URL.Query().Get("key"), "the key must not be in the URL")
		calls[r.URL.Path]++
		switch r.URL.Path {
		case "/channels":
			assert.Equal(t, "@mkbhd", r.URL.Query().Get("forHandle"))
			w.Write(channels)
		case "/playlistItems":
			assert.Equal(t, "UUBJycsmduvYEL83R_U4JriQ", r.URL.Query().Get("playlistId"))
			w.Write(playlist)
		case "/videos":
			assert.Equal(t, "pOX1l1edBME,ohqxP8EEumo,6D__H_DO2Xk,Od6M0AXpcxQ", r.URL.Query().Get("id"))
			w.Write(videos)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewYouTubeClient("test-key")
	c.BaseURL = srv.URL
	for range 2 {
		ch, err := c.Fetch(context.Background(), "@mkbhd")
		require.NoError(t, err)
		assert.Equal(t, "UCBJycsmduvYEL83R_U4JriQ", ch.ChannelID)
		assert.Equal(t, "Marques Brownlee", ch.ChannelName)
		assert.Equal(t, "@mkbhd", *ch.Handle)
		assert.Len(t, ch.Videos, 3)
	}
	assert.Equal(t, 1, calls["/channels"], "the channel lookup is remembered")
	assert.Equal(t, 2, calls["/playlistItems"])
}

func TestYouTubeClientErrors(t *testing.T) {
	_, err := NewYouTubeClient("").Fetch(context.Background(), "@mkbhd")
	assert.ErrorIs(t, err, ErrYouTubeNoKey)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/channels" && r.URL.Query().Get("forHandle") == "@nobody" {
			w.Write([]byte(`{"pageInfo":{"totalResults":0}}`))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":{"code":403,"message":"The request cannot be completed because you have exceeded your <a href=\"/youtube/v3/getting-started#quota\">quota</a>."}}`))
	}))
	defer srv.Close()

	c := NewYouTubeClient("test-key")
	c.BaseURL = srv.URL
	_, err = c.Fetch(context.Background(), "@nobody")
	assert.ErrorContains(t, err, "@nobody: channel not found")
	_, err = c.Fetch(context.Background(), "@mkbhd")
	assert.ErrorContains(t, err, "exceeded your quota.", "HTML tags are stripped from API messages")
	assert.NotContains(t, err.Error(), "test-key")
}
