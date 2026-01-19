// Package sqlite implements the repo interface using SQLite
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/database/internal"
)

type repo struct {
	db        *sql.DB
	tableName string
}

func (r *repo) Get(ctx context.Context, path string) (stowry.MetaData, error) {
	query := fmt.Sprintf( //nolint:gosec // G201: table name is validated
		`SELECT id, path, content_type, etag, file_size_bytes, created_at, updated_at
		FROM %s
		WHERE path = ? AND deleted_at IS NULL`, r.tableName)

	var m stowry.MetaData
	var idStr string
	var createdAt, updatedAt string

	err := r.db.QueryRowContext(ctx, query, path).Scan(
		&idStr, &m.Path, &m.ContentType, &m.Etag, &m.FileSizeBytes, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return stowry.MetaData{}, stowry.ErrNotFound
		}
		return stowry.MetaData{}, fmt.Errorf("get: %w", err)
	}

	m.ID, err = uuid.Parse(idStr)
	if err != nil {
		return stowry.MetaData{}, fmt.Errorf("get: parse uuid: %w", err)
	}

	m.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return stowry.MetaData{}, fmt.Errorf("get: parse created_at: %w", err)
	}

	m.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return stowry.MetaData{}, fmt.Errorf("get: parse updated_at: %w", err)
	}

	return m, nil
}

func (r *repo) Upsert(ctx context.Context, entry stowry.ObjectEntry) (stowry.MetaData, bool, error) {
	// Check if entry exists first to determine if this is an insert or update
	var existingID string
	checkQuery := fmt.Sprintf(`SELECT id FROM %s WHERE path = ?`, r.tableName) //nolint:gosec // table name is validated
	err := r.db.QueryRowContext(ctx, checkQuery, entry.Path).Scan(&existingID)
	isInsert := errors.Is(err, sql.ErrNoRows)
	if err != nil && !isInsert {
		return stowry.MetaData{}, false, fmt.Errorf("upsert: check existing: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	var m stowry.MetaData

	if isInsert {
		// Insert new entry
		newID := uuid.New()
		insertQuery := fmt.Sprintf( //nolint:gosec // G201: table name is validated
			`INSERT INTO %s (id, path, content_type, etag, file_size_bytes, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, r.tableName)

		_, err = r.db.ExecContext(ctx, insertQuery,
			newID.String(), entry.Path, entry.ContentType, entry.ETag, entry.Size, now, now,
		)
		if err != nil {
			return stowry.MetaData{}, false, fmt.Errorf("upsert: insert: %w", err)
		}

		m.ID = newID
		m.CreatedAt, _ = time.Parse(time.RFC3339Nano, now)
	} else {
		// Update existing entry
		updateQuery := fmt.Sprintf( //nolint:gosec // G201: table name is validated
			`UPDATE %s
			SET content_type = ?, etag = ?, file_size_bytes = ?, updated_at = ?,
				deleted_at = NULL, cleaned_up_at = NULL
			WHERE path = ?`, r.tableName)

		_, err = r.db.ExecContext(ctx, updateQuery,
			entry.ContentType, entry.ETag, entry.Size, now, entry.Path,
		)
		if err != nil {
			return stowry.MetaData{}, false, fmt.Errorf("upsert: update: %w", err)
		}

		m.ID, _ = uuid.Parse(existingID)

		// Get the original created_at
		var createdAt string
		createdQuery := fmt.Sprintf(`SELECT created_at FROM %s WHERE path = ?`, r.tableName) //nolint:gosec // table name is validated
		if err := r.db.QueryRowContext(ctx, createdQuery, entry.Path).Scan(&createdAt); err != nil {
			return stowry.MetaData{}, false, fmt.Errorf("upsert: get created_at: %w", err)
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	}

	m.Path = entry.Path
	m.ContentType = entry.ContentType
	m.Etag = entry.ETag
	m.FileSizeBytes = entry.Size
	m.UpdatedAt, _ = time.Parse(time.RFC3339Nano, now)

	return m, isInsert, nil
}

func (r *repo) Delete(ctx context.Context, path string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	query := fmt.Sprintf( //nolint:gosec // G201: table name is validated
		`UPDATE %s
		SET deleted_at = ?
		WHERE path = ? AND deleted_at IS NULL`, r.tableName)

	result, err := r.db.ExecContext(ctx, query, now, path)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete: rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("delete: %w", stowry.ErrNotFound)
	}

	return nil
}

func (r *repo) List(ctx context.Context, q stowry.ListQuery) (stowry.ListResult, error) {
	return r.listWithCondition(ctx, q, "deleted_at IS NULL", "list")
}

func (r *repo) ListPendingCleanup(ctx context.Context, q stowry.ListQuery) (stowry.ListResult, error) {
	return r.listWithCondition(ctx, q, "deleted_at IS NOT NULL AND cleaned_up_at IS NULL", "list pending cleanup")
}

func (r *repo) listWithCondition(ctx context.Context, q stowry.ListQuery, whereCondition, opName string) (stowry.ListResult, error) {
	cursor, err := internal.DecodeCursor(q.Cursor)
	if err != nil {
		return stowry.ListResult{}, fmt.Errorf("%s: %w", opName, err)
	}

	escapedPrefix := internal.EscapeLikePattern(q.PathPrefix)

	var query string
	var args []any

	if q.Cursor == "" {
		query = fmt.Sprintf(`
			SELECT id, path, content_type, etag, file_size_bytes, created_at, updated_at
			FROM %s
			WHERE %s AND path LIKE ? || '%%' ESCAPE '\'
			ORDER BY created_at, path
			LIMIT ?
		`, r.tableName, whereCondition)
		args = []any{escapedPrefix, q.Limit + 1}
	} else {
		query = fmt.Sprintf(`
			SELECT id, path, content_type, etag, file_size_bytes, created_at, updated_at
			FROM %s
			WHERE %s AND path LIKE ? || '%%' ESCAPE '\' AND (created_at, path) > (?, ?)
			ORDER BY created_at, path
			LIMIT ?
		`, r.tableName, whereCondition)
		args = []any{escapedPrefix, cursor.CreatedAt.Format(time.RFC3339Nano), cursor.Path, q.Limit + 1}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return stowry.ListResult{}, fmt.Errorf("%s: %w", opName, err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]stowry.MetaData, 0, q.Limit)
	for rows.Next() {
		var m stowry.MetaData
		var idStr, createdAt, updatedAt string

		if scanErr := rows.Scan(&idStr, &m.Path, &m.ContentType, &m.Etag, &m.FileSizeBytes, &createdAt, &updatedAt); scanErr != nil {
			return stowry.ListResult{}, fmt.Errorf("%s: scan: %w", opName, scanErr)
		}

		var parseErr error
		m.ID, parseErr = uuid.Parse(idStr)
		if parseErr != nil {
			return stowry.ListResult{}, fmt.Errorf("%s: parse uuid: %w", opName, parseErr)
		}

		m.CreatedAt, parseErr = time.Parse(time.RFC3339Nano, createdAt)
		if parseErr != nil {
			return stowry.ListResult{}, fmt.Errorf("%s: parse created_at: %w", opName, parseErr)
		}

		m.UpdatedAt, parseErr = time.Parse(time.RFC3339Nano, updatedAt)
		if parseErr != nil {
			return stowry.ListResult{}, fmt.Errorf("%s: parse updated_at: %w", opName, parseErr)
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
		nextCursor = internal.EncodeCursor(lastItem.CreatedAt, lastItem.Path)
		items = items[:q.Limit]
	}

	return stowry.ListResult{Items: items, NextCursor: nextCursor}, nil
}

func (r *repo) MarkCleanedUp(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	query := fmt.Sprintf( //nolint:gosec // G201: table name is validated
		`UPDATE %s
		SET cleaned_up_at = ?
		WHERE id = ? AND deleted_at IS NOT NULL AND cleaned_up_at IS NULL`, r.tableName)

	result, err := r.db.ExecContext(ctx, query, now, id.String())
	if err != nil {
		return fmt.Errorf("mark cleaned up: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark cleaned up: rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("mark cleaned up: %w", stowry.ErrNotFound)
	}

	return nil
}
