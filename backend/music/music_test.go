package music

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVideoID(t *testing.T) {
	const id = "wb_E3HnLwG4"
	for _, in := range []string{
		id,
		"  " + id + "  ",
		"https://www.youtube.com/watch?v=" + id,
		"https://www.youtube.com/watch?v=" + id + "&list=PLx&index=3&t=42s",
		"youtube.com/watch?v=" + id,
		"https://m.youtube.com/watch?v=" + id,
		"https://music.youtube.com/watch?v=" + id + "&si=abc",
		"https://youtu.be/" + id,
		"https://youtu.be/" + id + "?si=share-tracking&t=10",
		"https://www.youtube.com/shorts/" + id,
		"https://www.youtube.com/embed/" + id + "?autoplay=1",
		"https://www.youtube.com/live/" + id,
		"https://www.youtube-nocookie.com/embed/" + id,
	} {
		got, err := ParseVideoID(in)
		require.NoError(t, err, in)
		assert.Equal(t, id, got, in)
	}

	for _, bad := range []string{
		"",
		"not a link",
		"https://www.youtube.com/@mkbhd",
		"https://www.youtube.com/playlist?list=PLx",
		"https://www.youtube.com/watch?v=tooshort",
		"https://vimeo.com/123456789",
		"https://evil.example/watch?v=" + id,
	} {
		_, err := ParseVideoID(bad)
		assert.ErrorIs(t, err, ErrInvalidURL, bad)
	}
}

func TestOEmbedLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("url") {
		case "https://www.youtube.com/watch?v=wb_E3HnLwG4":
			w.Write([]byte(`{"title":"1 Hour of Relaxing and Beautiful Zelda Music","author_name":"Ralph L. Tanaka","thumbnail_url":"https://i.ytimg.com/vi/wb_E3HnLwG4/hqdefault.jpg"}`))
		case "https://www.youtube.com/watch?v=noembed0000":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	c := NewOEmbedClient()
	c.BaseURL = srv.URL

	v, err := c.Lookup(context.Background(), "wb_E3HnLwG4")
	require.NoError(t, err)
	assert.Equal(t, &Video{
		Title:        "1 Hour of Relaxing and Beautiful Zelda Music",
		ChannelName:  "Ralph L. Tanaka",
		ThumbnailURL: "https://i.ytimg.com/vi/wb_E3HnLwG4/mqdefault.jpg",
	}, v, "the 16:9 thumbnail is used instead of oEmbed's letterboxed one")

	_, err = c.Lookup(context.Background(), "noembed0000")
	assert.ErrorIs(t, err, ErrNotEmbeddable)
	_, err = c.Lookup(context.Background(), "missing0000")
	assert.ErrorIs(t, err, ErrVideoNotFound)
}
