package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"text/template"
	"time"
	"uuid"

	"github.com/sagarc03/stowry/internal/service"
	"github.com/sagarc03/stowry/types"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// sqliteDB stores metadata in SQLite. Ids are held as text and timestamps as
// RFC 3339, whose lexical order is chronological and so keeps the
// (created_at, path) cursor comparison working.
type sqliteDB struct {
	db    *sql.DB
	table string
}

var _ Database = (*sqliteDB)(nil)

// openSQLite opens dsn, a file path or an in-memory database. The connection is
// lazy.
func openSQLite(dsn string, tables types.Tables) (*sqliteDB, error) {
	db, err := sql.Open("sqlite", resolveDSN(dsn))
	if err != nil {
		return nil, fmt.Errorf("connect sqlite: %w", err)
	}

	return &sqliteDB{db: db, table: tables.MetaData}, nil
}

// memoryDBs numbers the in-memory databases this process has opened.
var memoryDBs atomic.Uint64

// resolveDSN rewrites the plain ":memory:" form into the URI that lets a pool
// share one in-memory database.
//
// SQLite gives every plain ":memory:" connection a database of its own, so a
// second pooled connection would open a fresh, empty one and every query on it
// would fail. Shared cache fixes that, but it is keyed by name, and an unnamed
// one would be shared by the whole process; numbering keeps separate calls as
// independent as ":memory:" implies. Any other DSN, including a URI the
// operator wrote themselves, is passed through untouched.
func resolveDSN(dsn string) string {
	if dsn != ":memory:" {
		return dsn
	}

	return fmt.Sprintf("file:stowry%d?mode=memory&cache=shared", memoryDBs.Add(1))
}

func (d *sqliteDB) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

func (d *sqliteDB) Close() error {
	return d.db.Close()
}

// sqliteSchema is the DDL for a metadata table. Every statement is IF NOT
// EXISTS, which is what makes Migrate idempotent.
var sqliteSchema = template.Must(template.New("sqlite").Parse(`
CREATE TABLE IF NOT EXISTS {{.Table}} (
	id TEXT NOT NULL PRIMARY KEY,
	path TEXT NOT NULL UNIQUE,
	content_type TEXT NOT NULL,
	etag TEXT NOT NULL,
	file_size_bytes INTEGER NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	deleted_at TEXT,
	cleaned_up_at TEXT
);

CREATE INDEX IF NOT EXISTS {{.DeletedIndex}}
ON {{.Table}} (deleted_at);

CREATE INDEX IF NOT EXISTS {{.PendingCleanupIndex}}
ON {{.Table}} (deleted_at, cleaned_up_at);

CREATE INDEX IF NOT EXISTS {{.ActiveListIndex}}
ON {{.Table}} (created_at, path);
`))

func (d *sqliteDB) Migrate(ctx context.Context) error {
	schema, err := renderSchema(sqliteSchema, d.table, quoteSQLite)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	if _, err := d.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	return nil
}

// sqliteColumns is the metadata table as PRAGMA table_info reports it.
var sqliteColumns = map[string]column{
	"id":              {"text", false},
	"path":            {"text", false},
	"content_type":    {"text", false},
	"etag":            {"text", false},
	"file_size_bytes": {"integer", false},
	"created_at":      {"text", false},
	"updated_at":      {"text", false},
	"deleted_at":      {"text", true},
	"cleaned_up_at":   {"text", true},
}

func (d *sqliteDB) Validate(ctx context.Context) error {
	// PRAGMA table_info returns no rows for a missing table, which would
	// otherwise read as every column being missing.
	var name string
	err := d.db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, d.table).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("validate: table %s does not exist", d.table)
	}
	if err != nil {
		return fmt.Errorf("validate: check table exists: %w", err)
	}

	rows, err := d.db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, quoteSQLite(d.table))) //nolint:gosec // G201: table name is validated in Connect
	if err != nil {
		return fmt.Errorf("validate: query columns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	got := make(map[string]column)
	for rows.Next() {
		var (
			cid, notNull, pk int
			colName, colType string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &colName, &colType, &notNull, &dflt, &pk); err != nil {
			return fmt.Errorf("validate: scan column: %w", err)
		}
		got[colName] = column{dataType: strings.ToLower(colType), nullable: notNull == 0}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("validate: rows: %w", err)
	}

	if err := checkColumns(d.table, sqliteColumns, got); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	return nil
}

func (d *sqliteDB) Get(ctx context.Context, path string) (types.MetaData, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE path = ? AND deleted_at IS NULL`, metaColumns, quoteSQLite(d.table)) //nolint:gosec // G201: table name is validated in Connect

	m, err := scanSQLiteMeta(d.db.QueryRowContext(ctx, query, path))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.MetaData{}, service.ErrNotFound
		}
		return types.MetaData{}, fmt.Errorf("get: %w", err)
	}

	return m, nil
}

func (d *sqliteDB) Upsert(ctx context.Context, entry types.ObjectEntry) (types.MetaData, bool, error) {
	// SQLite has no gen_random_uuid(), so the id and timestamps are supplied
	// here. RETURNING gives back newID on an insert and the existing id on an
	// update, which is how the two are told apart.
	newID := uuid.New()
	now := time.Now().UTC()

	query := fmt.Sprintf(`
		INSERT INTO %s (id, path, content_type, etag, file_size_bytes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (path) DO UPDATE
		SET content_type = excluded.content_type,
			etag = excluded.etag,
			file_size_bytes = excluded.file_size_bytes,
			updated_at = excluded.updated_at,
			deleted_at = NULL,
			cleaned_up_at = NULL
		RETURNING id, created_at`, quoteSQLite(d.table)) //nolint:gosec // G201: table name is validated in Connect

	var idStr, createdAtStr string
	err := d.db.QueryRowContext(ctx, query,
		newID.String(), entry.Path, entry.ContentType, entry.ETag, entry.Size,
		formatSQLiteTime(now), formatSQLiteTime(now),
	).Scan(&idStr, &createdAtStr)
	if err != nil {
		return types.MetaData{}, false, fmt.Errorf("upsert: %w", err)
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return types.MetaData{}, false, fmt.Errorf("upsert: parse id: %w", err)
	}

	createdAt, err := parseSQLiteTime(createdAtStr)
	if err != nil {
		return types.MetaData{}, false, fmt.Errorf("upsert: parse created_at: %w", err)
	}

	return types.MetaData{
		ID:            id,
		Path:          entry.Path,
		ContentType:   entry.ContentType,
		Etag:          entry.ETag,
		FileSizeBytes: entry.Size,
		CreatedAt:     createdAt,
		UpdatedAt:     now,
	}, id == newID, nil
}

func (d *sqliteDB) Delete(ctx context.Context, path string) error {
	query := fmt.Sprintf(`
		UPDATE %s
		SET deleted_at = ?
		WHERE path = ? AND deleted_at IS NULL`, quoteSQLite(d.table)) //nolint:gosec // G201: table name is validated in Connect

	if err := d.execOne(ctx, query, formatSQLiteTime(time.Now().UTC()), path); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	return nil
}

func (d *sqliteDB) MarkCleanedUp(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf(`
		UPDATE %s
		SET cleaned_up_at = ?
		WHERE id = ? AND deleted_at IS NOT NULL AND cleaned_up_at IS NULL`, quoteSQLite(d.table)) //nolint:gosec // G201: table name is validated in Connect

	if err := d.execOne(ctx, query, formatSQLiteTime(time.Now().UTC()), id.String()); err != nil {
		return fmt.Errorf("mark cleaned up: %w", err)
	}

	return nil
}

func (d *sqliteDB) List(ctx context.Context, q types.ListQuery) (types.ListResult, error) {
	return d.list(ctx, q, "deleted_at IS NULL", "list")
}

func (d *sqliteDB) ListPendingCleanup(ctx context.Context, q types.ListQuery) (types.ListResult, error) {
	return d.list(ctx, q, "deleted_at IS NOT NULL AND cleaned_up_at IS NULL", "list pending cleanup")
}

// list pages rows matching where, ordered by (created_at, path). It reads one
// row beyond the limit to learn whether a further page exists.
func (d *sqliteDB) list(ctx context.Context, q types.ListQuery, where, op string) (types.ListResult, error) {
	after, err := decodeCursor(q.Cursor)
	if err != nil {
		return types.ListResult{}, fmt.Errorf("%s: %w", op, err)
	}

	args := []any{escapeLikePattern(q.PathPrefix)}
	keyset := ""
	if q.Cursor != "" {
		keyset = ` AND (created_at, path) > (?, ?)`
		args = append(args, formatSQLiteTime(after.CreatedAt), after.Path)
	}
	args = append(args, q.Limit+1)

	query := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE %s AND path LIKE ? || '%%' ESCAPE '\'%s
		ORDER BY created_at, path
		LIMIT ?`, metaColumns, quoteSQLite(d.table), where, keyset) //nolint:gosec // G201: table name is validated in Connect and the where clause is one of the two literals in this file

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return types.ListResult{}, fmt.Errorf("%s: %w", op, err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]types.MetaData, 0, q.Limit)
	for rows.Next() {
		m, err := scanSQLiteMeta(rows)
		if err != nil {
			return types.ListResult{}, fmt.Errorf("%s: %w", op, err)
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return types.ListResult{}, fmt.Errorf("%s: rows: %w", op, err)
	}

	var next string
	if len(items) > q.Limit {
		items = items[:q.Limit]
		last := items[len(items)-1]
		next = encodeCursor(last.CreatedAt, last.Path)
	}

	return types.ListResult{Items: items, NextCursor: next}, nil
}

// execOne runs a statement expected to affect one row, reporting ErrNotFound
// when it affects none.
func (d *sqliteDB) execOne(ctx context.Context, query string, args ...any) error {
	result, err := d.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if affected == 0 {
		return service.ErrNotFound
	}

	return nil
}

// scanSQLiteMeta reads metaColumns into a types.MetaData, parsing the columns
// SQLite stores as text.
func scanSQLiteMeta(row rowScanner) (types.MetaData, error) {
	var (
		m                                 types.MetaData
		idStr, createdAtStr, updatedAtStr string
	)

	if err := row.Scan(&idStr, &m.Path, &m.ContentType, &m.Etag, &m.FileSizeBytes, &createdAtStr, &updatedAtStr); err != nil {
		return types.MetaData{}, err
	}

	var err error
	if m.ID, err = uuid.Parse(idStr); err != nil {
		return types.MetaData{}, fmt.Errorf("parse id: %w", err)
	}
	if m.CreatedAt, err = parseSQLiteTime(createdAtStr); err != nil {
		return types.MetaData{}, fmt.Errorf("parse created_at: %w", err)
	}
	if m.UpdatedAt, err = parseSQLiteTime(updatedAtStr); err != nil {
		return types.MetaData{}, fmt.Errorf("parse updated_at: %w", err)
	}

	return m, nil
}

func formatSQLiteTime(t time.Time) string { return t.Format(time.RFC3339Nano) }

func parseSQLiteTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

// quoteSQLite quotes an identifier. Connect already constrains table names to
// [a-z_][a-z0-9_]*, but one of those can still be a SQLite keyword.
func quoteSQLite(name string) string { return `"` + name + `"` }
