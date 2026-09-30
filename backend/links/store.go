package links

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"dashboard/fsutil"
)

// Store holds the quick links. The file at path is the source of truth: it's read on startup
// and whenever it changes on disk (e.g. after a git pull), and rewritten when the links are
// edited in the dashboard. With no path configured, links are kept in memory only.
type Store struct {
	path string

	mu        sync.RWMutex
	groups    []*QuickLinkGroup
	version   string          // a hash of the links, so a save from a stale editor is caught
	origins   map[string]bool // every link's origin, for Favicons
	signature string          // the file's state when last read or written
}

func NewStore(path string) *Store {
	s := &Store{path: path}
	s.set(nil)
	return s
}

// QuickLinks is every link group plus a version for saving (see Save). gqlgen binds it to
// the GraphQL type of the same name.
type QuickLinks struct {
	Version string
	Groups  []*QuickLinkGroup
}

// ErrStale means the links changed since the editor loaded them.
var ErrStale = errors.New("the links were changed somewhere else (another tab, or a git pull); showing the latest now, so try that again")

// Links returns the link groups in file order.
func (s *Store) Links() *QuickLinks {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &QuickLinks{Version: s.version, Groups: s.groups}
}

// HasOrigin reports whether any link points at origin (see Origin).
func (s *Store) HasOrigin(origin string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.origins[origin]
}

// Save replaces all the links and rewrites the file. version must be the one the editor
// loaded; if the links have changed since, nothing is saved and ErrStale is returned.
func (s *Store) Save(version string, groups []*QuickLinkGroup) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if version != s.version {
		return ErrStale
	}
	if s.path != "" {
		if err := fsutil.EnsureDir(filepath.Dir(s.path)); err != nil {
			return fmt.Errorf("create links folder: %w", err)
		}
		if err := fsutil.WriteFileAtomic(s.path, Format(groups)); err != nil {
			return fmt.Errorf("write links: %w", err)
		}
		// Remember our own write so Watch doesn't treat it as an outside change.
		s.signature = s.fileSignature()
	}
	s.set(groups)
	return nil
}

// Load reads the links file. A missing file means no links yet.
func (s *Store) Load() error {
	if s.path == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sig := s.fileSignature()
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.set(nil)
		s.signature = sig
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	groups, problems := Parse(f)
	for _, p := range problems {
		log.Printf("links %s: %s", filepath.Base(s.path), p)
	}
	s.set(groups)
	s.signature = sig
	return nil
}

// Watch reloads the file whenever it changes on disk.
func (s *Store) Watch(ctx context.Context, interval time.Duration) {
	if s.path == "" {
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
		s.mu.RLock()
		changed := s.fileSignature() != s.signature
		s.mu.RUnlock()
		if changed {
			if err := s.Load(); err != nil {
				log.Printf("links: %v", err)
			} else {
				log.Printf("links: reloaded %s", s.path)
			}
		}
	}
}

// set replaces the links in memory. Callers hold s.mu for writing.
func (s *Store) set(groups []*QuickLinkGroup) {
	if groups == nil {
		groups = []*QuickLinkGroup{}
	}
	s.groups = groups
	sum := sha256.Sum256(Format(groups))
	s.version = hex.EncodeToString(sum[:8])
	s.origins = map[string]bool{}
	for _, g := range groups {
		for _, l := range g.Links {
			if origin, err := Origin(l.URL); err == nil {
				s.origins[origin] = true
			}
		}
	}
}

func (s *Store) fileSignature() string {
	info, err := os.Stat(s.path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
}
