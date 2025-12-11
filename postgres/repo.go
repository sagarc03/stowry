// Package postgres implements the repo interface for all the services
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sagarc03/stowry"
)

// Tables is an alias for stowry.Tables for package compatibility.
type Tables = stowry.Tables

type Repo struct {
	pool      *pgxpool.Pool
	tableName string
}

func NewRepo(pool *pgxpool.Pool, tables Tables) (*Repo, error) {
	if err := tables.Validate(); err != nil {
		return nil, fmt.Errorf("new repo: %w", err)
	}

	return &Repo{pool: pool, tableName: tables.MetaData}, nil
}

// Ping verifies database connectivity
func (r *Repo) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *Repo) Get(ctx context.Context, path string) (stowry.MetaData, error) {
	query := fmt.Sprintf(`
		SELECT id, path, content_type, etag, file_size_bytes, created_at, updated_at
		FROM %s
		WHERE path = $1 AND deleted_at IS NULL
	`, r.tableName)

	var m stowry.MetaData
	err := r.pool.QueryRow(ctx, query, path).Scan(
		&m.ID, &m.Path, &m.ContentType, &m.Etag, &m.FileSizeBytes, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return stowry.MetaData{}, stowry.ErrNotFound
		}
		return stowry.MetaData{}, fmt.Errorf("get: %w", err)
	}

	return m, nil
}

func (r *Repo) Upsert(ctx context.Context, entry stowry.ObjectEntry) (stowry.MetaData, bool, error) {
	query := fmt.Sprintf(`
		INSERT INTO %s (path, content_type, etag, file_size_bytes)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (path) DO UPDATE
		SET content_type = EXCLUDED.content_type,
			etag = EXCLUDED.etag,
			file_size_bytes = EXCLUDED.file_size_bytes,
			updated_at = NOW(),
			deleted_at = NULL,
			cleaned_up_at = NULL
		RETURNING id, path, content_type, etag, file_size_bytes, created_at, updated_at,
			(xmax = 0) AS inserted
	`, r.tableName)

	var m stowry.MetaData
	var inserted bool

	err := r.pool.QueryRow(ctx, query, entry.Path, entry.ContentType, entry.ETag, entry.Size).Scan(
		&m.ID, &m.Path, &m.ContentType, &m.Etag, &m.FileSizeBytes, &m.CreatedAt, &m.UpdatedAt, &inserted,
	)
	if err != nil {
		return stowry.MetaData{}, false, fmt.Errorf("upsert: %w", err)
	}

	return m, inserted, nil
}

func (r *Repo) Delete(ctx context.Context, path string) error {
	query := fmt.Sprintf(`
		UPDATE %s
		SET deleted_at = NOW()
		WHERE path = $1 AND deleted_at IS NULL
	`, r.tableName)

	result, err := r.pool.Exec(ctx, query, path)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("delete: %w", stowry.ErrNotFound)
	}

	return nil
}

func (r *Repo) List(ctx context.Context, q stowry.ListQuery) (stowry.ListResult, error) {
	return r.listWithCondition(ctx, q, "deleted_at IS NULL", "list")
}

func (r *Repo) ListPendingCleanup(ctx context.Context, q stowry.ListQuery) (stowry.ListResult, error) {
	return r.listWithCondition(ctx, q, "deleted_at IS NOT NULL AND cleaned_up_at IS NULL", "list pending cleanup")
}

func (r *Repo) listWithCondition(ctx context.Context, q stowry.ListQuery, whereCondition, opName string) (stowry.ListResult, error) {
	cursor, err := stowry.DecodeCursor(q.Cursor)
	if err != nil {
		return stowry.ListResult{}, fmt.Errorf("%s: %w", opName, err)
	}

	escapedPrefix := stowry.EscapeLikePattern(q.PathPrefix)

	var query string
	var args []any

	if q.Cursor == "" {
		query = fmt.Sprintf(`
			SELECT id, path, content_type, etag, file_size_bytes, created_at, updated_at
			FROM %s
			WHERE %s AND path LIKE $1 || '%%'
			ORDER BY created_at, path
			LIMIT $2
		`, r.tableName, whereCondition)
		args = []any{escapedPrefix, q.Limit + 1}
	} else {
		query = fmt.Sprintf(`
			SELECT id, path, content_type, etag, file_size_bytes, created_at, updated_at
			FROM %s
			WHERE %s AND path LIKE $1 || '%%' AND (created_at, path) > ($2, $3)
			ORDER BY created_at, path
			LIMIT $4
		`, r.tableName, whereCondition)
		args = []any{escapedPrefix, cursor.CreatedAt, cursor.Path, q.Limit + 1}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return stowry.ListResult{}, fmt.Errorf("%s: %w", opName, err)
	}
	defer rows.Close()

	items := make([]stowry.MetaData, 0, q.Limit)
	for rows.Next() {
		var m stowry.MetaData
		if err := rows.Scan(&m.ID, &m.Path, &m.ContentType, &m.Etag, &m.FileSizeBytes, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return stowry.ListResult{}, fmt.Errorf("%s: scan: %w", opName, err)
		}
		items = append(items, m)
	}

	if err := rows.Err(); err != nil {
		return stowry.ListResult{}, fmt.Errorf("%s: rows: %w", opName, err)
	}

	var nextCursor string
	if len(items) > q.Limit {
		// Cursor points to the last item of the current page
		lastItem := items[q.Limit-1]
		nextCursor = stowry.EncodeCursor(lastItem.CreatedAt, lastItem.Path)
		items = items[:q.Limit]
	}

	return stowry.ListResult{Items: items, NextCursor: nextCursor}, nil
}

func (r *Repo) MarkCleanedUp(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf(`
		UPDATE %s
		SET cleaned_up_at = NOW()
		WHERE id = $1 AND deleted_at IS NOT NULL AND cleaned_up_at IS NULL
	`, r.tableName)

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark cleaned up: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("mark cleaned up: %w", stowry.ErrNotFound)
	}

	return nil
}
