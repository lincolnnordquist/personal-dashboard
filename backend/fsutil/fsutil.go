// Package fsutil writes files in the repo's bind-mounted folders (playlists, quick links).
package fsutil

import (
	"os"
	"path/filepath"
	"syscall"
)

// WriteFileAtomic replaces path with data by writing a temporary file and renaming it, so a
// reader never sees a half-written file. The backend runs as root in Docker, so the new file
// is given to the directory's owner to keep it editable by the user.
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".write-*.tmp")
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

// EnsureDir creates dir if needed, owned like its parent (see WriteFileAtomic).
func EnsureDir(dir string) error {
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
