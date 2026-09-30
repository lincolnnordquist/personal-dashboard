package links

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"dashboard/db"
)

const (
	// IconTTL is how long a fetched favicon is kept before it's fetched again.
	IconTTL = 7 * 24 * time.Hour
	// missingIconTTL is how long to wait before retrying a site whose icon couldn't be found.
	missingIconTTL = 24 * time.Hour

	maxPageBytes = 1 << 20
	maxIconBytes = 512 << 10
)

// IconStore caches favicons (db.IconRepo in production).
type IconStore interface {
	GetIcon(ctx context.Context, origin string) (*db.Icon, error)
	SetIcon(ctx context.Context, origin string, icon *db.Icon) error
}

// Favicons serves each quick link's site icon at /icons?site=<origin>. The dashboard fetches
// icons itself, so no third-party icon service sees the links, and caches them in Postgres.
// Only origins that appear in the links are fetched, so the endpoint can't be used to make
// the backend request arbitrary URLs.
type Favicons struct {
	links  *Store
	store  IconStore
	client *http.Client
	now    func() time.Time

	locks sync.Map // origin -> *sync.Mutex, so one site is fetched once at a time
}

func NewFavicons(links *Store, store IconStore) *Favicons {
	return &Favicons{
		links: links,
		store: store,
		// Generous: some icon CDNs are slow, and the result is cached for a week.
		client: &http.Client{Timeout: 30 * time.Second},
		now:    time.Now,
	}
}

func (f *Favicons) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin, err := Origin(r.URL.Query().Get("site"))
	if err != nil || !f.links.HasOrigin(origin) {
		http.NotFound(w, r)
		return
	}
	icon, err := f.icon(r.Context(), origin)
	if err != nil {
		log.Printf("favicon %s: %v", origin, err)
	}
	if icon == nil || len(icon.Data) == 0 {
		// The widget shows the link's first letter instead.
		w.Header().Set("Cache-Control", "public, max-age=3600")
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", icon.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Icons come from other sites; an SVG opened directly must not run scripts on this origin.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	w.Write(icon.Data)
}

// icon returns the cached icon for origin, fetching it when missing or expired. When a
// refetch fails, the old icon is kept.
func (f *Favicons) icon(ctx context.Context, origin string) (*db.Icon, error) {
	lock, _ := f.locks.LoadOrStore(origin, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	cached, err := f.store.GetIcon(ctx, origin)
	if err != nil {
		return nil, err
	}
	now := f.now()
	if cached != nil {
		ttl := IconTTL
		if len(cached.Data) == 0 {
			ttl = missingIconTTL
		}
		if now.Sub(cached.FetchedAt) < ttl {
			return cached, nil
		}
	}

	icon, fetchErr := f.fetch(ctx, origin)
	if fetchErr != nil {
		icon = &db.Icon{}
		if cached != nil {
			icon = cached
		}
	}
	icon.FetchedAt = now
	if err := f.store.SetIcon(ctx, origin, icon); err != nil {
		return icon, err
	}
	return icon, fetchErr
}

// fetch finds a site's icon: the best <link rel="icon"> on its home page, falling back to
// /favicon.ico.
func (f *Favicons) fetch(ctx context.Context, origin string) (*db.Icon, error) {
	var candidates []string
	page, base, err := f.get(ctx, origin+"/", maxPageBytes)
	if err == nil {
		candidates = IconCandidates(page, base)
	}
	candidates = append(candidates, origin+"/favicon.ico")
	if base != nil {
		// The home page may have redirected to another host, e.g. a login page.
		candidates = append(candidates, base.Scheme+"://"+base.Host+"/favicon.ico")
	}

	var lastErr error
	tried := map[string]bool{}
	for _, c := range candidates {
		if tried[c] {
			continue
		}
		tried[c] = true
		icon, err := f.fetchImage(ctx, c)
		if err == nil {
			return icon, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("no icon found: %w", lastErr)
}

func (f *Favicons) fetchImage(ctx context.Context, src string) (*db.Icon, error) {
	if strings.HasPrefix(src, "data:") {
		return decodeDataURL(src)
	}
	body, _, err := f.get(ctx, src, maxIconBytes)
	if err != nil {
		return nil, err
	}
	contentType := imageType(body)
	if contentType == "" {
		return nil, fmt.Errorf("%s isn't an image", src)
	}
	return &db.Icon{ContentType: contentType, Data: body}, nil
}

// get fetches a URL, returning its body and final URL after redirects.
func (f *Favicons) get(ctx context.Context, src string, limit int64) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return nil, nil, err
	}
	// Some sites turn away requests that don't look like a browser.
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,image/*,*/*;q=0.8")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.Request.URL, fmt.Errorf("%s: %s", src, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, resp.Request.URL, err
	}
	if int64(len(body)) > limit {
		return nil, resp.Request.URL, fmt.Errorf("%s is too large", src)
	}
	return body, resp.Request.URL, nil
}

var (
	linkTag = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	attr    = regexp.MustCompile(`(?s)([a-zA-Z-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	size    = regexp.MustCompile(`(\d+)x\d+`)
)

// IconCandidates lists the icons a page declares, best first: larger sizes and SVGs first,
// Apple touch icons (usually 180px) before plain icons of unknown size.
func IconCandidates(page []byte, base *url.URL) []string {
	type candidate struct {
		href  string
		score int
	}
	var found []candidate
	for _, tag := range linkTag.FindAll(page, -1) {
		attrs := map[string]string{}
		for _, m := range attr.FindAllSubmatch(tag, -1) {
			attrs[strings.ToLower(string(m[1]))] = string(m[2]) + string(m[3]) + string(m[4])
		}
		rel := strings.Fields(strings.ToLower(attrs["rel"]))
		href := strings.TrimSpace(attrs["href"])
		if href == "" {
			continue
		}
		score := -1
		for _, r := range rel {
			switch r {
			case "icon":
				score = max(score, 32)
			case "apple-touch-icon", "apple-touch-icon-precomposed":
				score = max(score, 180)
			}
			// mask-icon is a one-color SVG meant for Safari's pinned tabs; skip it.
		}
		if score < 0 {
			continue
		}
		if m := size.FindStringSubmatch(attrs["sizes"]); m != nil {
			score, _ = strconv.Atoi(m[1])
		}
		if strings.Contains(attrs["type"], "svg") || strings.HasSuffix(strings.ToLower(strings.SplitN(href, "?", 2)[0]), ".svg") || attrs["sizes"] == "any" {
			score = 1000
		}
		if !strings.HasPrefix(href, "data:") {
			ref, err := url.Parse(href)
			if err != nil {
				continue
			}
			href = base.ResolveReference(ref).String()
		}
		found = append(found, candidate{href, score})
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].score > found[j].score })
	out := make([]string, len(found))
	for i, c := range found {
		out[i] = c.href
	}
	return out
}

// imageType returns an image MIME type for data, or "" if it isn't an image.
func imageType(data []byte) string {
	sniffed := http.DetectContentType(data)
	if strings.HasPrefix(sniffed, "image/") {
		return sniffed
	}
	// DetectContentType sees SVGs as XML or text. Error pages served with 200 may contain
	// inline SVGs too, so anything that looks like HTML is rejected.
	head := bytes.ToLower(data[:min(len(data), 4096)])
	if bytes.Contains(head, []byte("<svg")) && !bytes.Contains(head, []byte("<html")) {
		return "image/svg+xml"
	}
	return ""
}

func decodeDataURL(src string) (*db.Icon, error) {
	meta, payload, ok := strings.Cut(strings.TrimPrefix(src, "data:"), ",")
	if !ok {
		return nil, errors.New("bad data URL")
	}
	var data []byte
	if strings.HasSuffix(meta, ";base64") {
		var err error
		if data, err = base64.StdEncoding.DecodeString(payload); err != nil {
			return nil, err
		}
	} else {
		unescaped, err := url.PathUnescape(payload)
		if err != nil {
			return nil, err
		}
		data = []byte(unescaped)
	}
	contentType := imageType(data)
	if contentType == "" || len(data) > maxIconBytes {
		return nil, errors.New("data URL isn't a usable image")
	}
	return &db.Icon{ContentType: contentType, Data: data}, nil
}
