package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WidgetConfig is one row of widget_config. gqlgen binds it to the GraphQL WidgetConfig type.
type WidgetConfig struct {
	ID         int
	WidgetType string
	Config     map[string]any
	Position   int
	Enabled    bool
}

var ErrNotFound = errors.New("not found")

type WidgetRepo struct {
	pool *pgxpool.Pool
}

func NewWidgetRepo(pool *pgxpool.Pool) *WidgetRepo {
	return &WidgetRepo{pool: pool}
}

const widgetColumns = `id, widget_type, config, position, enabled`

// List returns all widgets ordered by grid position.
func (r *WidgetRepo) List(ctx context.Context) ([]*WidgetConfig, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+widgetColumns+` FROM widget_config ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanWidget)
}

// ListEnabledByType returns enabled widgets of one type, for background cache refreshes.
func (r *WidgetRepo) ListEnabledByType(ctx context.Context, widgetType string) ([]*WidgetConfig, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+widgetColumns+` FROM widget_config WHERE widget_type = $1 AND enabled ORDER BY id`,
		widgetType)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanWidget)
}

// UpdateConfig replaces a widget's config and, if position is non-nil, moves it.
func (r *WidgetRepo) UpdateConfig(ctx context.Context, id int, config map[string]any, position *int) (*WidgetConfig, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return r.one(ctx,
		`UPDATE widget_config
		 SET config = $2, position = COALESCE($3, position), updated_at = NOW()
		 WHERE id = $1
		 RETURNING `+widgetColumns,
		id, raw, position)
}

func (r *WidgetRepo) SetEnabled(ctx context.Context, id int, enabled bool) (*WidgetConfig, error) {
	return r.one(ctx,
		`UPDATE widget_config SET enabled = $2, updated_at = NOW() WHERE id = $1 RETURNING `+widgetColumns,
		id, enabled)
}

func (r *WidgetRepo) one(ctx context.Context, sql string, args ...any) (*WidgetConfig, error) {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	w, err := pgx.CollectExactlyOneRow(rows, scanWidget)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return w, err
}

func scanWidget(row pgx.CollectableRow) (*WidgetConfig, error) {
	var w WidgetConfig
	var enabled *bool
	if err := row.Scan(&w.ID, &w.WidgetType, &w.Config, &w.Position, &enabled); err != nil {
		return nil, err
	}
	// The enabled column is nullable in the spec's schema; treat NULL as enabled.
	w.Enabled = enabled == nil || *enabled
	return &w, nil
}
