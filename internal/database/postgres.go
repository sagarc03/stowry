package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sagarc03/stowry/internal/service"
	"github.com/sagarc03/stowry/types"
)

// postgresDB stores metadata in PostgreSQL. Ids and timestamps are native
// column types, so rows scan straight into types.MetaData and the database
// owns the clock.
type postgresDB struct {
	pool  *pgxpool.Pool
	table string
}

var _ Database = (*postgresDB)(nil)

// openPostgres creates a connection pool for dsn. The pool connects lazily, so
// an unreachable server surfaces on the first query.
func openPostgres(ctx context.Context, dsn string, tables types.Tables) (*postgresDB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	return &postgresDB{pool: pool, table: tables.MetaData}, nil
}

func (d *postgresDB) Ping(ctx context.Context) error {
	return d.pool.Ping(ctx)
}

func (d *postgresDB) Close() error {
	d.pool.Close()
	return nil
}

// postgresSchema is the DDL for a metadata table. Every statement is
// IF NOT EXISTS, which is what makes Migrate idempotent. The indexes are
// partial so each covers only the rows its query reads.
var postgresSchema = template.Must(template.New("postgres").Parse(`
CREATE TABLE IF NOT EXISTS {{.Table}} (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	path TEXT NOT NULL UNIQUE,
	content_type TEXT NOT NULL,
	etag TEXT NOT NULL,
	file_size_bytes BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	deleted_at TIMESTAMPTZ,
	cleaned_up_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS {{.DeletedIndex}}
ON {{.Table}} (deleted_at)
WHERE deleted_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS {{.PendingCleanupIndex}}
ON {{.Table}} (deleted_at, cleaned_up_at)
WHERE deleted_at IS NOT NULL AND cleaned_up_at IS NULL;

CREATE INDEX IF NOT EXISTS {{.ActiveListIndex}}
ON {{.Table}} (created_at, path)
WHERE deleted_at IS NULL;
`))

func (d *postgresDB) Migrate(ctx context.Context) error {
	schema, err := renderSchema(postgresSchema, d.table, quotePostgres)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	if _, err := d.pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// The DDL is all IF NOT EXISTS, so it succeeds without touching a table
	// that already exists. Validating is what stops that being a silent pass.
	if err := d.Validate(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	return nil
}

// postgresColumns is the metadata table as information_schema.columns reports it.
var postgresColumns = map[string]column{
	"id":              {"uuid", false},
	"path":            {"text", false},
	"content_type":    {"text", false},
	"etag":            {"text", false},
	"file_size_bytes": {"bigint", false},
	"created_at":      {"timestamp with time zone", false},
	"updated_at":      {"timestamp with time zone", false},
	"deleted_at":      {"timestamp with time zone", true},
	"cleaned_up_at":   {"timestamp with time zone", true},
}

func (d *postgresDB) Validate(ctx context.Context) error {
	// information_schema.columns returns no rows for a missing table, which
	// would otherwise read as every column being missing.
	var exists bool
	err := d.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)`, d.table).Scan(&exists)
	if err != nil {
		return fmt.Errorf("validate: check table exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("validate: table %s does not exist", d.table)
	}

	// Filtered by schema as the check above is, so a table of the same name in
	// another visible schema cannot merge its columns into this one's.
	rows, err := d.pool.Query(ctx, `
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1`, d.table)
	if err != nil {
		return fmt.Errorf("validate: query columns: %w", err)
	}
	defer rows.Close()

	got := make(map[string]column)
	for rows.Next() {
		var name, dataType, nullable string
		if err := rows.Scan(&name, &dataType, &nullable); err != nil {
			return fmt.Errorf("validate: scan column: %w", err)
		}
		got[name] = column{dataType: strings.ToLower(dataType), nullable: nullable == "YES"}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("validate: rows: %w", err)
	}

	if err := checkColumns(d.table, postgresColumns, got); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	unique, err := d.hasUniquePath(ctx)
	if err != nil {
		return fmt.Errorf("validate: check unique %s: %w", uniquePathColumn, err)
	}

	if !unique {
		return fmt.Errorf("validate: %w", errNoUniquePath(d.table))
	}

	return nil
}

// hasUniquePath reports whether a single-column unique index covers path.
// A partial index does not count: it constrains only the rows it covers.
func (d *postgresDB) hasUniquePath(ctx context.Context) (bool, error) {
	var exists bool

	err := d.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_index i
			JOIN pg_class c ON c.oid = i.indrelid
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = i.indkey[0]
			WHERE c.relname = $1
			  AND n.nspname = 'public'
			  AND i.indisunique
			  AND i.indnkeyatts = 1
			  AND i.indpred IS NULL
			  AND a.attname = $2
		)`, d.table, uniquePathColumn).Scan(&exists)

	return exists, err
}

func (d *postgresDB) Get(ctx context.Context, path string) (types.MetaData, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE path = $1 AND deleted_at IS NULL`, metaColumns, quotePostgres(d.table)) //nolint:gosec // G201: table name is validated in Connect

	m, err := scanPostgresMeta(d.pool.QueryRow(ctx, query, path))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.MetaData{}, service.ErrNotFound
		}
		return types.MetaData{}, fmt.Errorf("get: %w", err)
	}

	return m, nil
}

func (d *postgresDB) Upsert(ctx context.Context, entry types.ObjectEntry) (types.MetaData, bool, error) {
	// xmax is zero on an inserted row and non-zero on an updated one, which is
	// how the two are told apart.
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
		RETURNING %s, (xmax = 0) AS inserted`, quotePostgres(d.table), metaColumns) //nolint:gosec // G201: table name is validated in Connect

	var m types.MetaData
	var inserted bool
	err := d.pool.QueryRow(ctx, query, entry.Path, entry.ContentType, entry.ETag, entry.Size).Scan(
		&m.ID, &m.Path, &m.ContentType, &m.Etag, &m.FileSizeBytes, &m.CreatedAt, &m.UpdatedAt, &inserted,
	)
	if err != nil {
		return types.MetaData{}, false, fmt.Errorf("upsert: %w", err)
	}

	return m, inserted, nil
}

func (d *postgresDB) Delete(ctx context.Context, path string) error {
	query := fmt.Sprintf(`
		UPDATE %s
		SET deleted_at = NOW()
		WHERE path = $1 AND deleted_at IS NULL`, quotePostgres(d.table)) //nolint:gosec // G201: table name is validated in Connect

	if err := d.execOne(ctx, query, path); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	return nil
}

func (d *postgresDB) MarkCleanedUp(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf(`
		UPDATE %s
		SET cleaned_up_at = NOW()
		WHERE id = $1 AND deleted_at IS NOT NULL AND cleaned_up_at IS NULL`, quotePostgres(d.table)) //nolint:gosec // G201: table name is validated in Connect

	if err := d.execOne(ctx, query, id); err != nil {
		return fmt.Errorf("mark cleaned up: %w", err)
	}

	return nil
}

func (d *postgresDB) List(ctx context.Context, q types.ListQuery) (types.ListResult, error) {
	return d.list(ctx, q, "deleted_at IS NULL", "list")
}

func (d *postgresDB) ListPendingCleanup(ctx context.Context, q types.ListQuery) (types.ListResult, error) {
	return d.list(ctx, q, "deleted_at IS NOT NULL AND cleaned_up_at IS NULL", "list pending cleanup")
}

// list pages rows matching where, ordered by (created_at, path). It reads one
// row beyond the limit to learn whether a further page exists.
func (d *postgresDB) list(ctx context.Context, q types.ListQuery, where, op string) (types.ListResult, error) {
	after, err := decodeCursor(q.Cursor)
	if err != nil {
		return types.ListResult{}, fmt.Errorf("%s: %w", op, err)
	}

	args := []any{escapeLikePattern(q.PathPrefix)}
	keyset := ""
	if q.Cursor != "" {
		keyset = ` AND (created_at, path) > ($2, $3)`
		args = append(args, after.CreatedAt, after.Path)
	}
	args = append(args, q.Limit+1)

	query := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE %s AND path LIKE $1 || '%%'%s
		ORDER BY created_at, path
		LIMIT $%d`, metaColumns, quotePostgres(d.table), where, keyset, len(args)) //nolint:gosec // G201: table name is validated in Connect and the where clause is one of the two literals in this file

	rows, err := d.pool.Query(ctx, query, args...)
	if err != nil {
		return types.ListResult{}, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	items := make([]types.MetaData, 0, q.Limit)
	for rows.Next() {
		m, err := scanPostgresMeta(rows)
		if err != nil {
			return types.ListResult{}, fmt.Errorf("%s: scan: %w", op, err)
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
func (d *postgresDB) execOne(ctx context.Context, query string, args ...any) error {
	result, err := d.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return service.ErrNotFound
	}

	return nil
}

// scanPostgresMeta reads metaColumns into a types.MetaData.
func scanPostgresMeta(row rowScanner) (types.MetaData, error) {
	var m types.MetaData
	err := row.Scan(&m.ID, &m.Path, &m.ContentType, &m.Etag, &m.FileSizeBytes, &m.CreatedAt, &m.UpdatedAt)
	return m, err
}

// quotePostgres quotes an identifier. Connect already constrains table names to
// [a-z_][a-z0-9_]*, but one of those can still be a reserved word.
func quotePostgres(name string) string { return pgx.Identifier{name}.Sanitize() }
