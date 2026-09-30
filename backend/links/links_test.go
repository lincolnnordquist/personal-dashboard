package links

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dashboard/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	groups, problems := Parse(strings.NewReader(`# comment
loose.example.com

[dev]
GitHub | https://github.com
Go docs | pkg.go.dev | 🐹
| https://www.example.org/path
Broken | ftp://nope

[]
[Dev]
MDN | https://developer.mozilla.org/en-US/
`))
	require.Len(t, groups, 2)
	assert.Equal(t, "links", groups[0].Name)
	assert.Equal(t, []*QuickLink{{Title: "loose.example.com", URL: "https://loose.example.com"}}, groups[0].Links)

	assert.Equal(t, "dev", groups[1].Name, "[Dev] joins [dev]")
	assert.Equal(t, []*QuickLink{
		{Title: "GitHub", URL: "https://github.com"},
		{Title: "Go docs", URL: "https://pkg.go.dev", Icon: "🐹"},
		{Title: "example.org", URL: "https://www.example.org/path"},
		{Title: "MDN", URL: "https://developer.mozilla.org/en-US/"},
	}, groups[1].Links)
	assert.Len(t, problems, 2, "ftp link and empty group name")
}

func TestFormatRoundTrip(t *testing.T) {
	groups := []*QuickLinkGroup{
		{Name: "dev", Links: []*QuickLink{{Title: "GitHub", URL: "https://github.com"}, {Title: "Go", URL: "https://go.dev", Icon: "🐹"}}},
		{Name: "empty", Links: []*QuickLink{}},
	}
	parsed, problems := Parse(strings.NewReader(string(Format(groups))))
	assert.Empty(t, problems)
	assert.Equal(t, groups, parsed, "empty groups are kept too")
}

func TestFromInput(t *testing.T) {
	icon := " 🎮 "
	groups, err := FromInput([]*QuickLinkGroupInput{{
		Name: " games | fun ",
		Links: []*QuickLinkInput{
			{Title: "", URL: "store.steampowered.com", Icon: &icon},
			{Title: "Zelda\nWiki", URL: "https://zeldawiki.wiki/"},
		},
	}})
	require.NoError(t, err)
	assert.Equal(t, "games / fun", groups[0].Name)
	assert.Equal(t, []*QuickLink{
		{Title: "store.steampowered.com", URL: "https://store.steampowered.com", Icon: "🎮"},
		{Title: "Zelda Wiki", URL: "https://zeldawiki.wiki/"},
	}, groups[0].Links)

	_, err = FromInput([]*QuickLinkGroupInput{{Name: "a"}, {Name: "A"}})
	assert.ErrorContains(t, err, "two groups")
	_, err = FromInput([]*QuickLinkGroupInput{{Name: " "}})
	assert.Error(t, err)
	_, err = FromInput([]*QuickLinkGroupInput{{Name: "a", Links: []*QuickLinkInput{{URL: "javascript:alert(1)"}}}})
	assert.Error(t, err)
}

func TestFaviconURL(t *testing.T) {
	l := &QuickLink{URL: "https://GitHub.com/user/repo?tab=1"}
	assert.Equal(t, "/icons?site=https%3A%2F%2Fgithub.com", l.FaviconURL())
}

func TestStoreSaveLoadWatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "links", "links.txt")
	s := NewStore(path)
	require.NoError(t, s.Load(), "a missing file is fine")
	assert.Empty(t, s.Links().Groups)

	groups := []*QuickLinkGroup{{Name: "dev", Links: []*QuickLink{{Title: "GitHub", URL: "https://github.com/x"}}}}
	loaded := s.Links().Version
	require.NoError(t, s.Save(loaded, groups))
	assert.True(t, s.HasOrigin("https://github.com"))
	assert.False(t, s.HasOrigin("https://example.com"))

	// A save based on links that have since changed is refused.
	assert.ErrorIs(t, s.Save(loaded, nil), ErrStale)
	assert.Equal(t, groups, s.Links().Groups)

	reloaded := NewStore(path)
	require.NoError(t, reloaded.Load())
	assert.Equal(t, groups, reloaded.Links().Groups)
	assert.Equal(t, s.Links().Version, reloaded.Links().Version)

	// An outside edit (e.g. git pull) is picked up by Watch.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Watch(ctx, 10*time.Millisecond)
	require.NoError(t, os.WriteFile(path, []byte("[news]\nhttps://example.com\n"), 0o644))
	assert.Eventually(t, func() bool { return s.HasOrigin("https://example.com") }, time.Second, 10*time.Millisecond)
	assert.Equal(t, "news", s.Links().Groups[0].Name)
}

func TestIconCandidates(t *testing.T) {
	page, err := os.ReadFile("testdata/github_head.html")
	require.NoError(t, err)
	base, _ := url.Parse("https://github.com/")
	assert.Equal(t, []string{
		"https://github.githubassets.com/favicons/favicon.svg",
		"https://github.githubassets.com/favicons/favicon.png",
	}, IconCandidates(page, base), "SVG first; mask-icon and fluid-icon skipped")

	page = []byte(`<link rel="shortcut icon" href="/favicon.ico">
<LINK REL='apple-touch-icon' HREF='/touch.png'>
<link href="icons/big.png" sizes="192x192" rel="icon">`)
	base, _ = url.Parse("https://example.com/home/")
	assert.Equal(t, []string{
		"https://example.com/home/icons/big.png",
		"https://example.com/touch.png",
		"https://example.com/favicon.ico",
	}, IconCandidates(page, base))
}

// fakeIcons is an in-memory IconStore.
type fakeIcons map[string]*db.Icon

func (f fakeIcons) GetIcon(_ context.Context, origin string) (*db.Icon, error) { return f[origin], nil }
func (f fakeIcons) SetIcon(_ context.Context, origin string, icon *db.Icon) error {
	cp := *icon
	f[origin] = &cp
	return nil
}

var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func TestFavicons(t *testing.T) {
	var hits int
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<html><head><link rel="icon" href="/missing.png"><link rel="apple-touch-icon" href="/touch.png"></head></html>`))
		case "/touch.png":
			w.Write(pngBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer site.Close()
	bare := httptest.NewServer(http.NotFoundHandler())
	defer bare.Close()

	store := NewStore("")
	require.NoError(t, store.Save(store.Links().Version, []*QuickLinkGroup{{Name: "x", Links: []*QuickLink{{URL: site.URL + "/a"}, {URL: bare.URL}}}}))
	icons := fakeIcons{}
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	f := NewFavicons(store, icons)
	f.now = func() time.Time { return now }

	get := func(site string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		f.ServeHTTP(rec, httptest.NewRequest("GET", "/icons?site="+url.QueryEscape(site), nil))
		return rec
	}

	rec := get(site.URL)
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	assert.Equal(t, pngBytes, rec.Body.Bytes())

	// Cached: no more requests to the site until it expires.
	before := hits
	assert.Equal(t, 200, get(site.URL).Code)
	assert.Equal(t, before, hits)

	// A site with no icon is a cached 404.
	assert.Equal(t, 404, get(bare.URL).Code)
	assert.Empty(t, icons[strings.ToLower(bare.URL)].Data)

	// Sites that aren't in the links are never fetched.
	assert.Equal(t, 404, get("http://169.254.169.254").Code)
	_, fetched := icons["http://169.254.169.254"]
	assert.False(t, fetched)

	// After expiry a failed refetch keeps the old icon.
	site.Close()
	now = now.Add(IconTTL + time.Hour)
	rec = get(site.URL)
	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, pngBytes, rec.Body.Bytes())
}
