package music

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"dashboard/db"
)

// Store is the database side of the playlists (db.MusicRepo in production).
type Store interface {
	ListPlaylists(ctx context.Context) ([]*db.Playlist, error)
	CreatePlaylist(ctx context.Context, slug, name string) (*db.Playlist, error)
	UpdatePlaylist(ctx context.Context, id int, slug, name string) error
	DeletePlaylist(ctx context.Context, id int) error
	AddSong(ctx context.Context, playlistID int, videoID, title, channelName, thumbnailURL string) (*db.Song, error)
	RemoveSong(ctx context.Context, id int) (playlistID int, err error)
}

// VideoLookup finds a video's title and channel (OEmbedClient in production).
type VideoLookup interface {
	Lookup(ctx context.Context, videoID string) (*Video, error)
}

// Library holds the music player's playlists. Each playlist is a file, <slug>.txt, in dir,
// and those files are the source of truth, shared between machines through git; the database
// holds the same playlists for the API. With no dir configured, the database alone is used.
type Library struct {
	store  Store
	lookup VideoLookup
	dir    string
	// legacyFile is the single playlist file used before multiple playlists. Sync removes it
	// once its songs are in the database (migration 4 moved them into a "Zelda" playlist).
	legacyFile string

	mu        sync.Mutex
	signature string // the folder's state when last read or written, to spot outside changes
}

func NewLibrary(store Store, lookup VideoLookup, dir, legacyFile string) *Library {
	return &Library{store: store, lookup: lookup, dir: dir, legacyFile: legacyFile}
}

func (l *Library) Playlists(ctx context.Context) ([]*db.Playlist, error) {
	return l.store.ListPlaylists(ctx)
}

func (l *Library) CreatePlaylist(ctx context.Context, name string) (*db.Playlist, error) {
	name = oneLine(name)
	if name == "" {
		return nil, errors.New("playlist name can't be empty")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	slug, err := l.freeSlug(ctx, name, 0)
	if err != nil {
		return nil, err
	}
	p, err := l.store.CreatePlaylist(ctx, slug, name)
	if err != nil {
		return nil, err
	}
	return p, l.writeFiles(ctx)
}

func (l *Library) RenamePlaylist(ctx context.Context, id int, name string) (*db.Playlist, error) {
	name = oneLine(name)
	if name == "" {
		return nil, errors.New("playlist name can't be empty")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	slug, err := l.freeSlug(ctx, name, id)
	if err != nil {
		return nil, err
	}
	if err := l.store.UpdatePlaylist(ctx, id, slug, name); err != nil {
		return nil, err
	}
	if err := l.writeFiles(ctx); err != nil {
		return nil, err
	}
	return l.find(ctx, id)
}

func (l *Library) DeletePlaylist(ctx context.Context, id int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.store.DeletePlaylist(ctx, id); err != nil {
		return err
	}
	return l.writeFiles(ctx)
}

// AddSong looks up a YouTube link and adds it to a playlist.
func (l *Library) AddSong(ctx context.Context, playlistID int, url string) (*db.Song, error) {
	videoID, err := ParseVideoID(url)
	if err != nil {
		return nil, err
	}
	video, err := l.lookup.Lookup(ctx, videoID)
	if err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	song, err := l.store.AddSong(ctx, playlistID, videoID, video.Title, video.ChannelName, video.ThumbnailURL)
	if errors.Is(err, db.ErrDuplicate) {
		return nil, fmt.Errorf("%q is already in this playlist", video.Title)
	}
	if err != nil {
		return nil, err
	}
	return song, l.writeFiles(ctx)
}

func (l *Library) RemoveSong(ctx context.Context, id int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.store.RemoveSong(ctx, id); err != nil {
		return err
	}
	return l.writeFiles(ctx)
}

// Sync makes the database match the playlist files: playlists and songs only in the files are
// added, and those only in the database are removed. If the folder has no playlist files yet,
// it is created from the database instead, so existing playlists are never lost.
func (l *Library) Sync(ctx context.Context) error {
	if l.dir == "" {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	files, err := l.playlistFiles()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		if err := l.writeFiles(ctx); err != nil {
			return err
		}
		l.removeLegacyFile()
		return nil
	}

	existing, err := l.store.ListPlaylists(ctx)
	if err != nil {
		return err
	}
	bySlug := make(map[string]*db.Playlist, len(existing))
	for _, p := range existing {
		bySlug[p.Slug] = p
	}

	for slug, path := range files {
		if err := l.syncFile(ctx, slug, path, bySlug[slug]); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
	}
	for _, p := range existing {
		if _, ok := files[p.Slug]; !ok {
			if err := l.store.DeletePlaylist(ctx, p.ID); err != nil && !errors.Is(err, db.ErrNotFound) {
				return err
			}
		}
	}
	l.removeLegacyFile()
	l.signature = l.folderSignature()
	return nil
}

// syncFile makes one playlist in the database match its file. p is nil if it's new.
func (l *Library) syncFile(ctx context.Context, slug, path string, p *db.Playlist) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	name, entries, problems := ParsePlaylist(f)
	f.Close()
	for _, problem := range problems {
		log.Printf("playlist %s: %s", filepath.Base(path), problem)
	}
	if name == "" {
		name = slug
	}

	if p == nil {
		if p, err = l.store.CreatePlaylist(ctx, slug, name); err != nil {
			return err
		}
	} else if p.Name != name {
		if err := l.store.UpdatePlaylist(ctx, p.ID, slug, name); err != nil {
			return err
		}
	}

	inFile := map[string]bool{}
	for _, e := range entries {
		inFile[e.VideoID] = true
	}
	inDB := map[string]bool{}
	for _, s := range p.Songs {
		inDB[s.VideoID] = true
		if !inFile[s.VideoID] {
			if _, err := l.store.RemoveSong(ctx, s.ID); err != nil && !errors.Is(err, db.ErrNotFound) {
				return err
			}
		}
	}
	for _, e := range entries {
		if inDB[e.VideoID] {
			continue
		}
		title, channel := e.Title, e.ChannelName
		if title == "" {
			// Hand-added lines may be just an ID or link: look the title up.
			video, err := l.lookup.Lookup(ctx, e.VideoID)
			if err != nil {
				log.Printf("playlist %s: skipping %s: %v", filepath.Base(path), e.VideoID, err)
				continue
			}
			title, channel = video.Title, video.ChannelName
		}
		if _, err := l.store.AddSong(ctx, p.ID, e.VideoID, title, channel, ThumbnailURL(e.VideoID)); err != nil && !errors.Is(err, db.ErrDuplicate) {
			return err
		}
	}
	return nil
}

// Watch re-syncs whenever the playlist folder changes on disk, e.g. after a git pull.
func (l *Library) Watch(ctx context.Context, interval time.Duration) {
	if l.dir == "" {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		sig := l.folderSignature()
		l.mu.Lock()
		changed := sig != l.signature
		l.mu.Unlock()
		if changed {
			if err := l.Sync(ctx); err != nil {
				log.Printf("playlist sync: %v", err)
			} else {
				log.Printf("playlists: reloaded %s", l.dir)
			}
		}
	}
}

// writeFiles rewrites the playlist folder from the database: one file per playlist, and
// files for deleted or renamed playlists are removed. Callers hold l.mu.
func (l *Library) writeFiles(ctx context.Context) error {
	if l.dir == "" {
		return nil
	}
	playlists, err := l.store.ListPlaylists(ctx)
	if err != nil {
		return err
	}
	if err := ensureDir(l.dir); err != nil {
		return fmt.Errorf("create playlist folder: %w", err)
	}
	keep := map[string]bool{}
	for _, p := range playlists {
		entries := make([]Entry, len(p.Songs))
		for i, s := range p.Songs {
			entries[i] = Entry{VideoID: s.VideoID, Title: s.Title, ChannelName: s.ChannelName}
		}
		if err := writeFileAtomic(l.path(p.Slug), FormatPlaylist(p.Name, entries)); err != nil {
			return fmt.Errorf("write playlist %s: %w", p.Slug, err)
		}
		keep[p.Slug] = true
	}
	files, err := l.playlistFiles()
	if err != nil {
		return err
	}
	for slug, path := range files {
		if !keep[slug] {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	// Remember our own writes so Watch doesn't treat them as outside changes.
	l.signature = l.folderSignature()
	return nil
}

// playlistFiles maps slug to path for every <slug>.txt in the folder.
func (l *Library) playlistFiles() (map[string]string, error) {
	entries, err := os.ReadDir(l.dir)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, e := range entries {
		slug, ok := strings.CutSuffix(e.Name(), ".txt")
		if !ok || e.IsDir() {
			continue
		}
		if !SlugPattern.MatchString(slug) {
			log.Printf("playlists: ignoring %s (file names must be lowercase letters, digits, and dashes)", e.Name())
			continue
		}
		files[slug] = filepath.Join(l.dir, e.Name())
	}
	return files, nil
}

// folderSignature summarizes the playlist files' names, sizes, and times.
func (l *Library) folderSignature() string {
	files, _ := l.playlistFiles()
	var parts []string
	for slug, path := range files {
		if info, err := os.Stat(path); err == nil {
			parts = append(parts, fmt.Sprintf("%s:%d:%d", slug, info.Size(), info.ModTime().UnixNano()))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func (l *Library) path(slug string) string {
	return filepath.Join(l.dir, slug+".txt")
}

func (l *Library) removeLegacyFile() {
	if l.legacyFile == "" {
		return
	}
	if err := os.Remove(l.legacyFile); err == nil {
		log.Printf("playlists: removed %s; playlists now live in %s", l.legacyFile, l.dir)
	}
}

// freeSlug returns a slug for name that no other playlist uses, adding -2, -3… if needed.
// selfID is the playlist being renamed (0 when creating), whose own slug counts as free.
func (l *Library) freeSlug(ctx context.Context, name string, selfID int) (string, error) {
	playlists, err := l.store.ListPlaylists(ctx)
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, p := range playlists {
		if p.ID != selfID {
			taken[p.Slug] = true
		}
	}
	base := Slugify(name)
	slug := base
	for n := 2; taken[slug]; n++ {
		slug = fmt.Sprintf("%s-%d", base, n)
	}
	return slug, nil
}

func (l *Library) find(ctx context.Context, id int) (*db.Playlist, error) {
	playlists, err := l.store.ListPlaylists(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range playlists {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, db.ErrNotFound
}
