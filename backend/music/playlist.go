package music

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Entry is one line of the playlist file.
type Entry struct {
	VideoID     string
	Title       string
	ChannelName string
}

// namePrefix and themePrefix mark the lines holding a playlist's display name and its
// background theme.
const (
	namePrefix  = "# Playlist:"
	themePrefix = "# Theme:"
)

// Header is the metadata at the top of a playlist file.
type Header struct {
	Name  string
	Theme string // empty mixes all background themes
}

const playlistHelp = `# Music player playlist, shared between machines through git.
# One YouTube video per line: a video ID or link, optionally followed by "# title — channel".
# The dashboard rewrites this file when songs are added or removed in the player, and picks
# up changes made here (for example after a git pull) within a few seconds.
`

// titleSeparator splits "title — channel" in a line's comment.
const titleSeparator = " — "

// ParsePlaylist reads a playlist file. The header comes from "# Playlist: name" and
// "# Theme: folder" lines (empty when missing); other blank and # lines are ignored. Lines
// that aren't a YouTube video are returned as problems rather than failing the whole file.
func ParsePlaylist(r io.Reader) (header Header, entries []Entry, problems []string) {
	seen := map[string]bool{}
	scanner := bufio.NewScanner(r)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if rest, ok := strings.CutPrefix(line, namePrefix); ok && header.Name == "" {
			header.Name = strings.TrimSpace(rest)
			continue
		}
		if rest, ok := strings.CutPrefix(line, themePrefix); ok && header.Theme == "" {
			header.Theme = strings.TrimSpace(rest)
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ref, comment, _ := strings.Cut(line, "#")
		id, err := ParseVideoID(ref)
		if err != nil {
			problems = append(problems, fmt.Sprintf("line %d: %v", n, err))
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		title, channel, _ := strings.Cut(strings.TrimSpace(comment), titleSeparator)
		entries = append(entries, Entry{VideoID: id, Title: strings.TrimSpace(title), ChannelName: strings.TrimSpace(channel)})
	}
	return header, entries, problems
}

// FormatPlaylist renders a playlist in the format ParsePlaylist reads.
func FormatPlaylist(header Header, entries []Entry) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s %s\n", namePrefix, oneLine(header.Name))
	if header.Theme != "" {
		fmt.Fprintf(&b, "%s %s\n", themePrefix, oneLine(header.Theme))
	}
	b.WriteString(playlistHelp)
	b.WriteString("\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "%s  # %s%s%s\n", e.VideoID, oneLine(e.Title), titleSeparator, oneLine(e.ChannelName))
	}
	return b.Bytes()
}

// oneLine collapses whitespace, including newlines, so a value always fits on one line.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

var (
	slugInvalid = regexp.MustCompile(`[^a-z0-9]+`)
	// SlugPattern is what a playlist file's name (without .txt) must look like.
	SlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
)

// Slugify turns a playlist name into a file-safe slug, e.g. "Boss Themes!" -> "boss-themes".
func Slugify(name string) string {
	slug := strings.Trim(slugInvalid.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(slug) > 64 {
		slug = strings.TrimRight(slug[:64], "-")
	}
	if slug == "" {
		slug = "playlist"
	}
	return slug
}
