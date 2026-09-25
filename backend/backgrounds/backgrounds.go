// Package backgrounds lists the background video themes. A theme is a folder of videos in the
// backgrounds directory, optionally with a posters/ folder of stills named after each video.
package backgrounds

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BackgroundTheme is bound to the GraphQL BackgroundTheme type.
type BackgroundTheme struct {
	Name   string
	Videos []*BackgroundVideo
}

// BackgroundVideo is bound to the GraphQL BackgroundVideo type.
type BackgroundVideo struct {
	Name      string
	URL       string
	PosterURL *string
}

var (
	videoExts  = []string{".mp4", ".webm"}
	posterExts = []string{".jpg", ".jpeg", ".webp", ".png"}
)

// List returns every theme in dir that has at least one video, sorted by name. URLs are
// urlPrefix/<theme>/<file>, matching where the web server serves dir.
func List(dir, urlPrefix string) ([]*BackgroundTheme, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []*BackgroundTheme{}, nil
	}
	if err != nil {
		return nil, err
	}
	themes := []*BackgroundTheme{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		theme, err := readTheme(filepath.Join(dir, e.Name()), e.Name(), urlPrefix)
		if err != nil {
			return nil, err
		}
		if len(theme.Videos) > 0 {
			themes = append(themes, theme)
		}
	}
	sort.Slice(themes, func(i, j int) bool { return strings.ToLower(themes[i].Name) < strings.ToLower(themes[j].Name) })
	return themes, nil
}

// Exists reports whether a theme with this name has videos.
func Exists(dir, name string) (bool, error) {
	themes, err := List(dir, "")
	if err != nil {
		return false, err
	}
	for _, t := range themes {
		if t.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func readTheme(path, name, urlPrefix string) (*BackgroundTheme, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	posters := map[string]string{} // video base name -> poster file name
	if pf, err := os.ReadDir(filepath.Join(path, "posters")); err == nil {
		for _, p := range pf {
			if ext := strings.ToLower(filepath.Ext(p.Name())); hasExt(posterExts, ext) {
				posters[strings.TrimSuffix(p.Name(), filepath.Ext(p.Name()))] = p.Name()
			}
		}
	}

	theme := &BackgroundTheme{Name: name, Videos: []*BackgroundVideo{}}
	base := urlPrefix + "/" + url.PathEscape(name) + "/"
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.Name()))
		if f.IsDir() || strings.HasPrefix(f.Name(), ".") || !hasExt(videoExts, ext) {
			continue
		}
		stem := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
		v := &BackgroundVideo{Name: stem, URL: base + url.PathEscape(f.Name())}
		if poster, ok := posters[stem]; ok {
			u := base + "posters/" + url.PathEscape(poster)
			v.PosterURL = &u
		}
		theme.Videos = append(theme.Videos, v)
	}
	return theme, nil
}

func hasExt(exts []string, ext string) bool {
	for _, e := range exts {
		if e == ext {
			return true
		}
	}
	return false
}
