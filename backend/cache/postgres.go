// Package cache stores external API responses in PostgreSQL with a TTL.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Entry is a cached response. Data is raw JSON.
type Entry struct {
	Data      []byte
	FetchedAt time.Time
	ExpiresAt time.Time
}

func (e *Entry) Expired(now time.Time) bool {
	return !now.Before(e.ExpiresAt)
}

// Store reads and writes cache entries. Get returns (nil, nil) on a miss.
type Store interface {
	Get(ctx context.Context, widgetType, key string) (*Entry, error)
	Set(ctx context.Context, widgetType, key string, data []byte, ttl time.Duration) error
}

type PostgresStore struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool, now: time.Now}
}

func (s *PostgresStore) Get(ctx context.Context, widgetType, key string) (*Entry, error) {
	var e Entry
	err := s.pool.QueryRow(ctx,
		`SELECT data, fetched_at, expires_at FROM widget_cache WHERE widget_type = $1 AND cache_key = $2`,
		widgetType, key,
	).Scan(&e.Data, &e.FetchedAt, &e.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *PostgresStore) Set(ctx context.Context, widgetType, key string, data []byte, ttl time.Duration) error {
	// Timestamps are stored as UTC in TIMESTAMP (no time zone) columns.
	now := s.now().UTC()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO widget_cache (widget_type, cache_key, data, fetched_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (widget_type, cache_key)
		 DO UPDATE SET data = EXCLUDED.data, fetched_at = EXCLUDED.fetched_at, expires_at = EXCLUDED.expires_at`,
		widgetType, key, data, now, now.Add(ttl))
	return err
}

// GetOrFetch returns the cached value for key if it is still fresh. Otherwise it calls fetch,
// stores the result, and returns it. If fetch fails but an expired entry exists, the stale
// value is returned so the widget keeps showing data while the upstream API is down.
func GetOrFetch[T any](ctx context.Context, store Store, widgetType, key string, ttl time.Duration, now func() time.Time, fetch func(context.Context) (T, error)) (T, error) {
	var zero T

	entry, err := store.Get(ctx, widgetType, key)
	if err != nil {
		log.Printf("cache get %s/%s: %v", widgetType, key, err)
		entry = nil
	}
	if entry != nil && !entry.Expired(now().UTC()) {
		var v T
		if err := json.Unmarshal(entry.Data, &v); err == nil {
			return v, nil
		}
		// A corrupt entry is treated as a miss.
	}

	fresh, fetchErr := fetch(ctx)
	if fetchErr != nil {
		if entry != nil {
			var stale T
			if err := json.Unmarshal(entry.Data, &stale); err == nil {
				log.Printf("serving stale %s/%s after fetch error: %v", widgetType, key, fetchErr)
				return stale, nil
			}
		}
		return zero, fetchErr
	}

	if err := Put(ctx, store, widgetType, key, ttl, fresh); err != nil {
		log.Printf("cache set %s/%s: %v", widgetType, key, err)
	}
	return fresh, nil
}

// Put encodes v as JSON and writes it to the cache.
func Put[T any](ctx context.Context, store Store, widgetType, key string, ttl time.Duration, v T) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	return store.Set(ctx, widgetType, key, raw, ttl)
}
