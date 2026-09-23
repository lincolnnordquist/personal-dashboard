// Package db manages the PostgreSQL connection pool and schema migrations.
package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"dashboard/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

const schema = `
CREATE TABLE IF NOT EXISTS widget_config (
    id SERIAL PRIMARY KEY,
    widget_type VARCHAR(50) NOT NULL,
    config JSONB NOT NULL,
    position INT NOT NULL,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS widget_cache (
    id SERIAL PRIMARY KEY,
    widget_type VARCHAR(50) NOT NULL,
    cache_key VARCHAR(255) NOT NULL,
    data JSONB NOT NULL,
    fetched_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    UNIQUE(widget_type, cache_key)
);
`

// Connect opens a connection pool, retrying while Postgres finishes starting up.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	for attempt := 1; ; attempt++ {
		if err = pool.Ping(ctx); err == nil {
			return pool, nil
		}
		if attempt == 10 {
			pool.Close()
			return nil, fmt.Errorf("database unreachable after %d attempts: %w", attempt, err)
		}
		time.Sleep(time.Second)
	}
}

// Migrate creates the schema if needed and seeds default widgets on first run.
func Migrate(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM widget_config`).Scan(&count); err != nil {
		return fmt.Errorf("count widgets: %w", err)
	}
	if count > 0 {
		return nil
	}

	for i, w := range defaultWidgets(cfg) {
		raw, err := json.Marshal(w.config)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO widget_config (widget_type, config, position) VALUES ($1, $2, $3)`,
			w.widgetType, raw, i,
		); err != nil {
			return fmt.Errorf("seed %s widget: %w", w.widgetType, err)
		}
	}
	return nil
}

type seedWidget struct {
	widgetType string
	config     map[string]any
}

func defaultWidgets(cfg *config.Config) []seedWidget {
	return []seedWidget{
		{"weather", map[string]any{
			"lat":      cfg.DefaultLat,
			"lon":      cfg.DefaultLon,
			"location": cfg.DefaultLocationName,
			"unit":     "F",
		}},
		{"sports", map[string]any{"sports": []string{"nfl", "nba"}, "favoriteTeams": []string{}}},
		{"reddit", map[string]any{"subreddits": []string{"golang", "selfhosted"}}},
		{"youtube", map[string]any{"channelIds": []string{}}},
		{"docker", map[string]any{}},
	}
}
