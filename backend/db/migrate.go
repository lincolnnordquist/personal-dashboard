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

// migrations run in order, once each; schema_migrations records which have been applied.
// Append new migrations to the end and never edit one that has shipped.
var migrations = []string{
	// 1: initial schema.
	`
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
);`,

	// 2: three-column layout. Places existing widgets in their default columns and adds
	// the calendar widget to databases seeded before it existed.
	`
ALTER TABLE widget_config ADD COLUMN IF NOT EXISTS layout_column VARCHAR(10) NOT NULL DEFAULT 'center'
    CHECK (layout_column IN ('left', 'center', 'right'));

UPDATE widget_config SET
    layout_column = CASE widget_type WHEN 'docker' THEN 'left' WHEN 'weather' THEN 'right' ELSE 'center' END,
    position = CASE widget_type
        WHEN 'docker' THEN 1
        WHEN 'weather' THEN 0
        WHEN 'sports' THEN 0
        WHEN 'reddit' THEN 1
        WHEN 'youtube' THEN 2
        ELSE position END;

INSERT INTO widget_config (widget_type, config, position, layout_column)
SELECT 'calendar', '{"sport": "nfl", "team": "sea"}', 0, 'left'
WHERE EXISTS (SELECT 1 FROM widget_config)
  AND NOT EXISTS (SELECT 1 FROM widget_config WHERE widget_type = 'calendar');`,
}

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

// Migrate applies any pending migrations, then seeds default widgets on first run.
func Migrate(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) error {
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INT PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT NOW()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var current int
	if err := pool.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	for i := current; i < len(migrations); i++ {
		if err := applyMigration(ctx, pool, i+1, migrations[i]); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
	}
	return seed(ctx, pool, cfg)
}

// applyMigration runs one migration and records it, in a single transaction.
func applyMigration(ctx context.Context, pool *pgxpool.Pool, version int, sql string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, sql); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// seed inserts the default widgets into an empty widget_config table.
func seed(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) error {
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM widget_config`).Scan(&count); err != nil {
		return fmt.Errorf("count widgets: %w", err)
	}
	if count > 0 {
		return nil
	}

	positions := map[string]int{}
	for _, w := range defaultWidgets(cfg) {
		raw, err := json.Marshal(w.config)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO widget_config (widget_type, config, position, layout_column) VALUES ($1, $2, $3, $4)`,
			w.widgetType, raw, positions[w.column], w.column,
		); err != nil {
			return fmt.Errorf("seed %s widget: %w", w.widgetType, err)
		}
		positions[w.column]++
	}
	return nil
}

type seedWidget struct {
	widgetType string
	column     string
	config     map[string]any
}

// defaultWidgets are seeded in this order; each widget's position is its order within its column.
func defaultWidgets(cfg *config.Config) []seedWidget {
	return []seedWidget{
		{"calendar", "left", map[string]any{"sport": "nfl", "team": "sea"}},
		{"docker", "left", map[string]any{}},
		{"sports", "center", map[string]any{"sport": "nfl", "featuredTeam": "sea", "favoriteTeams": []string{}}},
		{"reddit", "center", map[string]any{"subreddits": []string{"nflv2", "selfhosted"}}},
		{"youtube", "center", map[string]any{"channelIds": []string{"@fireship", "@linustechtips", "@mkbhd", "@veritasium", "@videogamedunkey"}}},
		{"weather", "right", map[string]any{
			"lat":      cfg.DefaultLat,
			"lon":      cfg.DefaultLon,
			"location": cfg.DefaultLocationName,
			"unit":     "F",
		}},
	}
}
