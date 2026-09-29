// Package backgrounds lists the background video themes. A theme is a folder of videos in the
// backgrounds directory, optionally with a posters/ folder of stills named after each video
// and a font file named after the folder (e.g. zelda/zelda.otf) used for the whole page.
package backgrounds

import (
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// BackgroundTheme is bound to the GraphQL BackgroundTheme type.
type BackgroundTheme struct {
	Name    string
	Videos  []*BackgroundVideo
	FontURL *string
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
	fontExts   = []string{".woff2", ".woff", ".otf", ".ttf"}
)

// List returns every theme in dir that has at least one video, sorted by name. URLs are
// urlPrefix/<theme>/<file>?v=<version>, matching where the web server serves dir. The version
// comes from the file's size and modification time: browsers cache the videos for days, so
// replacing a file under the same name must change its URL or the old video keeps showing.
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
	posters := map[string]fs.DirEntry{} // video base name -> poster file
	if pf, err := os.ReadDir(filepath.Join(path, "posters")); err == nil {
		for _, p := range pf {
			if ext := strings.ToLower(filepath.Ext(p.Name())); hasExt(posterExts, ext) {
				posters[strings.TrimSuffix(p.Name(), filepath.Ext(p.Name()))] = p
			}
		}
	}

	theme := &BackgroundTheme{Name: name, Videos: []*BackgroundVideo{}}
	base := urlPrefix + "/" + url.PathEscape(name) + "/"
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.Name()))
		if f.IsDir() || strings.HasPrefix(f.Name(), ".") {
			continue
		}
		stem := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
		// The theme's font: a font file named after the folder, in any letter case.
		if hasExt(fontExts, ext) && strings.EqualFold(stem, name) {
			if theme.FontURL == nil || fontRank(ext) < fontRank(filepath.Ext(*theme.FontURL)) {
				u := base + url.PathEscape(f.Name()) + version(f)
				theme.FontURL = &u
			}
			continue
		}
		if !hasExt(videoExts, ext) {
			continue
		}
		v := &BackgroundVideo{Name: stem, URL: base + url.PathEscape(f.Name()) + version(f)}
		if poster, ok := posters[stem]; ok {
			u := base + "posters/" + url.PathEscape(poster.Name()) + version(poster)
			v.PosterURL = &u
		}
		theme.Videos = append(theme.Videos, v)
	}
	return theme, nil
}

// fontRank orders font formats by preference (smallest, most web-friendly first), for a theme
// that has the same font in several formats.
func fontRank(ext string) int {
	ext, _, _ = strings.Cut(strings.ToLower(ext), "?")
	for i, e := range fontExts {
		if e == ext {
			return i
		}
	}
	return len(fontExts)
}

// version is a "?v=..." suffix that changes whenever the file is replaced or edited.
func version(f fs.DirEntry) string {
	info, err := f.Info()
	if err != nil {
		return ""
	}
	return "?v=" + strconv.FormatInt(info.ModTime().Unix(), 36) + strconv.FormatInt(info.Size(), 36)
}

func hasExt(exts []string, ext string) bool {
	for _, e := range exts {
		if e == ext {
			return true
		}
	}
	return false
}
