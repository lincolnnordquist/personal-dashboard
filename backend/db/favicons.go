package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Icon is a site's cached favicon. Empty Data records a failed fetch, so a site without an
// icon isn't fetched again on every page load.
type Icon struct {
	ContentType string
	Data        []byte
	FetchedAt   time.Time
}

type IconRepo struct {
	pool *pgxpool.Pool
}

func NewIconRepo(pool *pgxpool.Pool) *IconRepo {
	return &IconRepo{pool: pool}
}

// GetIcon returns the cached icon for an origin, or nil if there is none.
func (r *IconRepo) GetIcon(ctx context.Context, origin string) (*Icon, error) {
	var icon Icon
	err := r.pool.QueryRow(ctx, `SELECT content_type, data, fetched_at FROM favicons WHERE origin = $1`, origin).
		Scan(&icon.ContentType, &icon.Data, &icon.FetchedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	icon.FetchedAt = icon.FetchedAt.UTC()
	return &icon, nil
}

func (r *IconRepo) SetIcon(ctx context.Context, origin string, icon *Icon) error {
	data := icon.Data
	if data == nil {
		data = []byte{} // a failed fetch; the column is NOT NULL
	}
	// Timestamps are stored as UTC in TIMESTAMP (no time zone) columns.
	_, err := r.pool.Exec(ctx, `
INSERT INTO favicons (origin, content_type, data, fetched_at) VALUES ($1, $2, $3, $4)
ON CONFLICT (origin) DO UPDATE SET content_type = $2, data = $3, fetched_at = $4`,
		origin, icon.ContentType, data, icon.FetchedAt.UTC())
	return err
}
