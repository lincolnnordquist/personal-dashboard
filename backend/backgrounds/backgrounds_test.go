package backgrounds

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func touch(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, nil, 0o644))
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "zelda", "kakariko.mp4"))
	touch(t, filepath.Join(dir, "zelda", "clock-town.mp4"))
	touch(t, filepath.Join(dir, "zelda", "posters", "kakariko.jpg"))
	touch(t, filepath.Join(dir, "zelda", "notes.txt"))
	touch(t, filepath.Join(dir, "Majora's Mask", "moon.webm"))
	touch(t, filepath.Join(dir, "empty", "readme.md"))
	touch(t, filepath.Join(dir, ".hidden", "x.mp4"))
	touch(t, filepath.Join(dir, "stray.mp4"))

	themes, err := List(dir, "/backgrounds")
	require.NoError(t, err)
	require.Len(t, themes, 2, "folders without videos, hidden folders, and loose files are skipped")

	majora, zelda := themes[0], themes[1]
	assert.Equal(t, "Majora's Mask", majora.Name, "sorted by name, case-insensitively")
	assert.Equal(t, "/backgrounds/Majora%27s%20Mask/moon.webm", majora.Videos[0].URL)
	assert.Nil(t, majora.Videos[0].PosterURL)

	require.Len(t, zelda.Videos, 2)
	assert.Equal(t, "clock-town", zelda.Videos[0].Name)
	assert.Nil(t, zelda.Videos[0].PosterURL, "posters are optional")
	assert.Equal(t, "/backgrounds/zelda/kakariko.mp4", zelda.Videos[1].URL)
	require.NotNil(t, zelda.Videos[1].PosterURL)
	assert.Equal(t, "/backgrounds/zelda/posters/kakariko.jpg", *zelda.Videos[1].PosterURL)

	ok, err := Exists(dir, "zelda")
	require.NoError(t, err)
	assert.True(t, ok)
	ok, _ = Exists(dir, "empty")
	assert.False(t, ok)
}

func TestListMissingDir(t *testing.T) {
	themes, err := List(filepath.Join(t.TempDir(), "nope"), "/backgrounds")
	require.NoError(t, err)
	assert.Empty(t, themes)
}
