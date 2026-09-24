package music

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

// Entry is one line of the playlist file.
type Entry struct {
	VideoID     string
	Title       string
	ChannelName string
}

// namePrefix marks the line that holds a playlist's display name.
const namePrefix = "# Playlist:"

const playlistHelp = `# Music player playlist, shared between machines through git.
# One YouTube video per line: a video ID or link, optionally followed by "# title — channel".
# The dashboard rewrites this file when songs are added or removed in the player, and picks
# up changes made here (for example after a git pull) within a few seconds.
`

// titleSeparator splits "title — channel" in a line's comment.
const titleSeparator = " — "

// ParsePlaylist reads a playlist file. The name comes from a "# Playlist: name" line (empty if
// there is none); other blank and # lines are ignored. Lines that aren't a YouTube video are
// returned as problems rather than failing the whole file.
func ParsePlaylist(r io.Reader) (name string, entries []Entry, problems []string) {
	seen := map[string]bool{}
	scanner := bufio.NewScanner(r)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if rest, ok := strings.CutPrefix(line, namePrefix); ok && name == "" {
			name = strings.TrimSpace(rest)
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
	return name, entries, problems
}

// FormatPlaylist renders a playlist in the format ParsePlaylist reads.
func FormatPlaylist(name string, entries []Entry) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s %s\n", namePrefix, oneLine(name))
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

// writeFileAtomic replaces path with data by writing a temporary file and renaming it, so a
// reader never sees a half-written playlist. The backend runs as root in Docker, so the new
// file is given to the directory's owner to keep it editable by the user.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".playlist-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	chownToParent(tmp.Name())
	return os.Rename(tmp.Name(), path)
}

// ensureDir creates dir if needed, owned like its parent (see writeFileAtomic).
func ensureDir(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	chownToParent(dir)
	return nil
}

// chownToParent gives path to the owner of its parent directory when running as root.
func chownToParent(path string) {
	if os.Geteuid() != 0 {
		return
	}
	if info, err := os.Stat(filepath.Dir(path)); err == nil {
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			_ = os.Chown(path, int(st.Uid), int(st.Gid))
		}
	}
}
