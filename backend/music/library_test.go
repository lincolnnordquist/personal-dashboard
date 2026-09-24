package music

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dashboard/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStore is an in-memory Store.
type fakeStore struct {
	playlists []*db.Playlist
	nextID    int
}

func (f *fakeStore) id() int {
	f.nextID++
	return f.nextID
}

func (f *fakeStore) ListPlaylists(context.Context) ([]*db.Playlist, error) {
	out := make([]*db.Playlist, len(f.playlists))
	for i, p := range f.playlists {
		cp := *p
		cp.Songs = append([]*db.Song{}, p.Songs...)
		out[i] = &cp
	}
	return out, nil
}

func (f *fakeStore) CreatePlaylist(_ context.Context, slug, name string) (*db.Playlist, error) {
	for _, p := range f.playlists {
		if p.Slug == slug {
			return nil, db.ErrDuplicate
		}
	}
	p := &db.Playlist{ID: f.id(), Slug: slug, Name: name, Songs: []*db.Song{}}
	f.playlists = append(f.playlists, p)
	return p, nil
}

func (f *fakeStore) get(id int) *db.Playlist {
	for _, p := range f.playlists {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func (f *fakeStore) UpdatePlaylist(_ context.Context, id int, slug, name string) error {
	p := f.get(id)
	if p == nil {
		return db.ErrNotFound
	}
	p.Slug, p.Name = slug, name
	return nil
}

func (f *fakeStore) DeletePlaylist(_ context.Context, id int) error {
	for i, p := range f.playlists {
		if p.ID == id {
			f.playlists = append(f.playlists[:i], f.playlists[i+1:]...)
			return nil
		}
	}
	return db.ErrNotFound
}

func (f *fakeStore) AddSong(_ context.Context, playlistID int, videoID, title, channel, thumb string) (*db.Song, error) {
	p := f.get(playlistID)
	if p == nil {
		return nil, db.ErrNotFound
	}
	for _, s := range p.Songs {
		if s.VideoID == videoID {
			return nil, db.ErrDuplicate
		}
	}
	s := &db.Song{ID: f.id(), PlaylistID: playlistID, VideoID: videoID, Title: title, ChannelName: channel, ThumbnailURL: thumb}
	p.Songs = append(p.Songs, s)
	return s, nil
}

func (f *fakeStore) RemoveSong(_ context.Context, id int) (int, error) {
	for _, p := range f.playlists {
		for i, s := range p.Songs {
			if s.ID == id {
				p.Songs = append(p.Songs[:i], p.Songs[i+1:]...)
				return p.ID, nil
			}
		}
	}
	return 0, db.ErrNotFound
}

// summary describes the store as "slug(Name): video,video; …", sorted by slug.
func (f *fakeStore) summary() string {
	var parts []string
	for _, p := range f.playlists {
		var ids []string
		for _, s := range p.Songs {
			ids = append(ids, s.VideoID)
		}
		parts = append(parts, p.Slug+"("+p.Name+"): "+strings.Join(ids, ","))
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

// fakeLookup knows a fixed set of videos.
type fakeLookup map[string]*Video

func (f fakeLookup) Lookup(_ context.Context, id string) (*Video, error) {
	if v, ok := f[id]; ok {
		return v, nil
	}
	return nil, ErrVideoNotFound
}

var lookup = fakeLookup{
	"aaaaaaaaaaa": {Title: "Lost Woods", ChannelName: "Zelda Fan", ThumbnailURL: ThumbnailURL("aaaaaaaaaaa")},
	"bbbbbbbbbbb": {Title: "Kakariko Village", ChannelName: "Zelda Fan", ThumbnailURL: ThumbnailURL("bbbbbbbbbbb")},
	"ccccccccccc": {Title: "Gerudo Valley", ChannelName: "Other Fan", ThumbnailURL: ThumbnailURL("ccccccccccc")},
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func readPlaylist(t *testing.T, path string) (string, []Entry) {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	name, entries, _ := ParsePlaylist(f)
	return name, entries
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestParseAndFormatPlaylist(t *testing.T) {
	in := `# Playlist: Zelda Chill
# a comment
aaaaaaaaaaa  # Lost Woods — Zelda Fan

https://youtu.be/bbbbbbbbbbb
not a video
aaaaaaaaaaa  # duplicate line
`
	name, entries, problems := ParsePlaylist(strings.NewReader(in))
	assert.Equal(t, "Zelda Chill", name)
	assert.Equal(t, []Entry{
		{VideoID: "aaaaaaaaaaa", Title: "Lost Woods", ChannelName: "Zelda Fan"},
		{VideoID: "bbbbbbbbbbb"},
	}, entries)
	assert.Equal(t, []string{"line 6: not a YouTube video link"}, problems)

	out := FormatPlaylist("Boss\nThemes", []Entry{
		{VideoID: "aaaaaaaaaaa", Title: "Lost Woods", ChannelName: "Zelda Fan"},
		{VideoID: "ccccccccccc", Title: "Gerudo\nValley", ChannelName: "Other Fan"},
	})
	name, again, problems := ParsePlaylist(strings.NewReader(string(out)))
	assert.Empty(t, problems)
	assert.Equal(t, "Boss Themes", name, "newlines can't break the format")
	assert.Equal(t, []Entry{
		{VideoID: "aaaaaaaaaaa", Title: "Lost Woods", ChannelName: "Zelda Fan"},
		{VideoID: "ccccccccccc", Title: "Gerudo Valley", ChannelName: "Other Fan"},
	}, again)
}

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Zelda":                 "zelda",
		"Boss Themes!":          "boss-themes",
		"  Majora's Mask  ":     "majora-s-mask",
		"🎵":                     "playlist",
		strings.Repeat("a", 80): strings.Repeat("a", 64),
	} {
		assert.Equal(t, want, Slugify(in), in)
		assert.Regexp(t, SlugPattern, Slugify(in))
	}
}

func TestSyncCreatesFolderFromDatabase(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "playlists")
	legacy := filepath.Join(root, "playlist.txt")
	writeFile(t, legacy, "aaaaaaaaaaa\n")

	store := &fakeStore{}
	p, _ := store.CreatePlaylist(context.Background(), "zelda", "Zelda")
	store.AddSong(context.Background(), p.ID, "aaaaaaaaaaa", "Lost Woods", "Zelda Fan", "")
	lib := NewLibrary(store, lookup, dir, legacy)

	require.NoError(t, lib.Sync(context.Background()))

	name, entries := readPlaylist(t, filepath.Join(dir, "zelda.txt"))
	assert.Equal(t, "Zelda", name)
	assert.Equal(t, []Entry{{VideoID: "aaaaaaaaaaa", Title: "Lost Woods", ChannelName: "Zelda Fan"}}, entries)
	assert.NoFileExists(t, legacy, "the old single playlist file is removed once migrated")
	assert.Equal(t, "zelda(Zelda): aaaaaaaaaaa", store.summary())
}

func TestSyncMakesDatabaseMatchFolder(t *testing.T) {
	dir := t.TempDir()
	store := &fakeStore{}
	ctx := context.Background()
	zelda, _ := store.CreatePlaylist(ctx, "zelda", "Zelda")
	store.AddSong(ctx, zelda.ID, "aaaaaaaaaaa", "Lost Woods", "Zelda Fan", "")
	store.AddSong(ctx, zelda.ID, "bbbbbbbbbbb", "Kakariko Village", "Zelda Fan", "")
	store.CreatePlaylist(ctx, "old", "Old")

	// What another machine pushed: zelda renamed and edited, "old" deleted, "boss" added.
	writeFile(t, filepath.Join(dir, "zelda.txt"), "# Playlist: Zelda Chill\nbbbbbbbbbbb  # Kakariko Village — Zelda Fan\nccccccccccc\n")
	writeFile(t, filepath.Join(dir, "boss.txt"), "aaaaaaaaaaa  # Lost Woods — Zelda Fan\n")
	writeFile(t, filepath.Join(dir, "Bad Name.txt"), "aaaaaaaaaaa\n")

	lib := NewLibrary(store, lookup, dir, "")
	require.NoError(t, lib.Sync(ctx))

	assert.Equal(t, "boss(boss): aaaaaaaaaaa; zelda(Zelda Chill): bbbbbbbbbbb,ccccccccccc", store.summary(),
		"files without a name line use their slug; bad file names are ignored")
	assert.Equal(t, "Gerudo Valley", store.get(zelda.ID).Songs[1].Title, "bare IDs get their title looked up")
}

func TestPlaylistEditsUpdateFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "playlists")
	store := &fakeStore{}
	lib := NewLibrary(store, lookup, dir, "")
	ctx := context.Background()

	chill, err := lib.CreatePlaylist(ctx, "Zelda Chill")
	require.NoError(t, err)
	assert.Equal(t, "zelda-chill", chill.Slug)
	boss, err := lib.CreatePlaylist(ctx, "zelda chill!")
	require.NoError(t, err)
	assert.Equal(t, "zelda-chill-2", boss.Slug, "slugs never collide")

	song, err := lib.AddSong(ctx, chill.ID, "https://youtu.be/aaaaaaaaaaa")
	require.NoError(t, err)
	_, err = lib.AddSong(ctx, boss.ID, "aaaaaaaaaaa")
	require.NoError(t, err, "the same song can be in two playlists")
	_, err = lib.AddSong(ctx, chill.ID, "aaaaaaaaaaa")
	assert.ErrorContains(t, err, `"Lost Woods" is already in this playlist`)

	renamed, err := lib.RenamePlaylist(ctx, boss.ID, "Boss Themes")
	require.NoError(t, err)
	assert.Equal(t, "boss-themes", renamed.Slug)
	assert.ElementsMatch(t, []string{"zelda-chill.txt", "boss-themes.txt"}, listDir(t, dir), "renaming renames the file")

	require.NoError(t, lib.RemoveSong(ctx, song.ID))
	_, entries := readPlaylist(t, filepath.Join(dir, "zelda-chill.txt"))
	assert.Empty(t, entries)

	require.NoError(t, lib.DeletePlaylist(ctx, chill.ID))
	assert.Equal(t, []string{"boss-themes.txt"}, listDir(t, dir))

	_, err = lib.CreatePlaylist(ctx, "   ")
	assert.Error(t, err)
}

func TestLibraryWithoutFolder(t *testing.T) {
	store := &fakeStore{}
	lib := NewLibrary(store, lookup, "", "")
	ctx := context.Background()
	require.NoError(t, lib.Sync(ctx))
	p, err := lib.CreatePlaylist(ctx, "Zelda")
	require.NoError(t, err)
	_, err = lib.AddSong(ctx, p.ID, "aaaaaaaaaaa")
	require.NoError(t, err)
	assert.Equal(t, "zelda(Zelda): aaaaaaaaaaa", store.summary())
}
