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
	Position   int    // order within the column
	Column     string // "left", "center", or "right"
	Enabled    bool
}

var ErrNotFound = errors.New("not found")

type WidgetRepo struct {
	pool *pgxpool.Pool
}

func NewWidgetRepo(pool *pgxpool.Pool) *WidgetRepo {
	return &WidgetRepo{pool: pool}
}

const widgetColumns = `id, widget_type, config, position, layout_column, enabled`

// List returns all widgets, grouped by column and ordered within each.
func (r *WidgetRepo) List(ctx context.Context) ([]*WidgetConfig, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+widgetColumns+` FROM widget_config ORDER BY layout_column, position, id`)
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

// MoveWidget moves a widget to a position within a column, possibly a different column than
// it's in now, closing the gap it leaves behind and opening one at its destination so every
// column's positions stay contiguous.
func (r *WidgetRepo) MoveWidget(ctx context.Context, id int, column string, position int) (*WidgetConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var fromColumn string
	var fromPosition int
	if err := tx.QueryRow(ctx, `SELECT layout_column, position FROM widget_config WHERE id = $1`, id).
		Scan(&fromColumn, &fromPosition); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE widget_config SET position = position - 1 WHERE layout_column = $1 AND position > $2`,
		fromColumn, fromPosition); err != nil {
		return nil, err
	}

	// Clamp to the destination column's size (it may be the column the widget just left).
	var count int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM widget_config WHERE layout_column = $1 AND id != $2`, column, id,
	).Scan(&count); err != nil {
		return nil, err
	}
	position = clamp(position, 0, count)

	if _, err := tx.Exec(ctx,
		`UPDATE widget_config SET position = position + 1 WHERE layout_column = $1 AND position >= $2 AND id != $3`,
		column, position, id); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx,
		`UPDATE widget_config SET layout_column = $2, position = $3, updated_at = NOW()
		 WHERE id = $1 RETURNING `+widgetColumns,
		id, column, position)
	if err != nil {
		return nil, err
	}
	w, err := pgx.CollectExactlyOneRow(rows, scanWidget)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return w, nil
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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
	if err := row.Scan(&w.ID, &w.WidgetType, &w.Config, &w.Position, &w.Column, &enabled); err != nil {
		return nil, err
	}
	// The enabled column is nullable in the spec's schema; treat NULL as enabled.
	w.Enabled = enabled == nil || *enabled
	return &w, nil
}
