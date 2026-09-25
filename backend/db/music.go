package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Playlist is a named list of songs for the music player. gqlgen binds it to the GraphQL
// Playlist type. Slug names its file in the playlists folder.
type Playlist struct {
	ID    int
	Slug  string
	Name  string
	Theme *string // background theme folder; nil mixes all themes
	Songs []*Song
}

// Song is one entry in a playlist. gqlgen binds it to the GraphQL Song type.
type Song struct {
	ID           int
	PlaylistID   int
	VideoID      string
	Title        string
	ChannelName  string
	ThumbnailURL string
	AddedAt      string // RFC 3339
}

var ErrDuplicate = errors.New("already exists")

type MusicRepo struct {
	pool *pgxpool.Pool
}

func NewMusicRepo(pool *pgxpool.Pool) *MusicRepo {
	return &MusicRepo{pool: pool}
}

// ListPlaylists returns every playlist with its songs, playlists by name and songs in the
// order they were added.
func (r *MusicRepo) ListPlaylists(ctx context.Context) ([]*Playlist, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, slug, name, theme FROM playlists ORDER BY LOWER(name), id`)
	if err != nil {
		return nil, err
	}
	playlists, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Playlist, error) {
		p := &Playlist{Songs: []*Song{}}
		return p, row.Scan(&p.ID, &p.Slug, &p.Name, &p.Theme)
	})
	if err != nil {
		return nil, err
	}

	rows, err = r.pool.Query(ctx, `SELECT `+songColumns+` FROM songs ORDER BY added_at, id`)
	if err != nil {
		return nil, err
	}
	songs, err := pgx.CollectRows(rows, scanSong)
	if err != nil {
		return nil, err
	}
	byID := make(map[int]*Playlist, len(playlists))
	for _, p := range playlists {
		byID[p.ID] = p
	}
	for _, s := range songs {
		if p := byID[s.PlaylistID]; p != nil {
			p.Songs = append(p.Songs, s)
		}
	}
	return playlists, nil
}

// CreatePlaylist returns ErrDuplicate if the slug is taken.
func (r *MusicRepo) CreatePlaylist(ctx context.Context, slug, name string) (*Playlist, error) {
	p := &Playlist{Slug: slug, Name: name, Songs: []*Song{}}
	err := r.pool.QueryRow(ctx, `INSERT INTO playlists (slug, name) VALUES ($1, $2) RETURNING id`, slug, name).Scan(&p.ID)
	if isUniqueViolation(err) {
		return nil, ErrDuplicate
	}
	return p, err
}

// UpdatePlaylist renames a playlist. It returns ErrDuplicate if the slug is taken.
func (r *MusicRepo) UpdatePlaylist(ctx context.Context, id int, slug, name string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE playlists SET slug = $2, name = $3 WHERE id = $1`, id, slug, name)
	if isUniqueViolation(err) {
		return ErrDuplicate
	}
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// SetPlaylistTheme sets a playlist's background theme; nil mixes all themes.
func (r *MusicRepo) SetPlaylistTheme(ctx context.Context, id int, theme *string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE playlists SET theme = $2 WHERE id = $1`, id, theme)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// DeletePlaylist deletes a playlist and its songs.
func (r *MusicRepo) DeletePlaylist(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM playlists WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// AddSong returns ErrDuplicate if the video is already in the playlist.
func (r *MusicRepo) AddSong(ctx context.Context, playlistID int, videoID, title, channelName, thumbnailURL string) (*Song, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO songs (playlist_id, video_id, title, channel_name, thumbnail_url) VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+songColumns,
		playlistID, videoID, title, channelName, thumbnailURL)
	if err != nil {
		return nil, err
	}
	song, err := pgx.CollectExactlyOneRow(rows, scanSong)
	var pgErr *pgconn.PgError
	switch {
	case isUniqueViolation(err):
		return nil, ErrDuplicate
	case errors.As(err, &pgErr) && pgErr.Code == "23503": // foreign_key_violation: no such playlist
		return nil, ErrNotFound
	}
	return song, err
}

// RemoveSong deletes a song and returns the playlist it was in.
func (r *MusicRepo) RemoveSong(ctx context.Context, id int) (playlistID int, err error) {
	err = r.pool.QueryRow(ctx, `DELETE FROM songs WHERE id = $1 RETURNING playlist_id`, id).Scan(&playlistID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return playlistID, err
}

const songColumns = `id, playlist_id, video_id, title, channel_name, thumbnail_url, added_at`

func scanSong(row pgx.CollectableRow) (*Song, error) {
	var s Song
	var added time.Time
	if err := row.Scan(&s.ID, &s.PlaylistID, &s.VideoID, &s.Title, &s.ChannelName, &s.ThumbnailURL, &added); err != nil {
		return nil, err
	}
	// added_at is stored in UTC without a time zone.
	s.AddedAt = added.UTC().Format(time.RFC3339)
	return &s, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
