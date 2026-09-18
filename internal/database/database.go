// Package database stores object metadata in SQLite or PostgreSQL.
//
// Connect returns one value that manages the connection, its schema and the
// metadata in it:
//
//	db, err := database.Connect(ctx, cfg)
//	if err != nil {
//		return err
//	}
//	defer db.Close()
//
//	if err := db.Migrate(ctx); err != nil {
//		return err
//	}
//
//	svc := service.New(db, storage)
//
// Database is not declared in terms of service.MetaDataRepo; it satisfies that
// interface implicitly.
package database

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"text/template"
	"uuid"

	"github.com/sagarc03/stowry/types"
)

// Database reads and writes object metadata over a connection it owns.
type Database interface {
	// Get returns the metadata for path, or ErrNotFound.
	Get(ctx context.Context, path string) (types.MetaData, error)

	// Upsert creates or replaces the metadata for entry.Path, restoring it if
	// it was soft-deleted. The bool reports whether the entry was created.
	Upsert(ctx context.Context, entry types.ObjectEntry) (types.MetaData, bool, error)

	// Delete soft-deletes the metadata for path, or returns ErrNotFound.
	Delete(ctx context.Context, path string) error

	// List returns a page of live metadata entries matching q.
	List(ctx context.Context, q types.ListQuery) (types.ListResult, error)

	// ListPendingCleanup returns a page of soft-deleted entries whose files
	// have not been removed.
	ListPendingCleanup(ctx context.Context, q types.ListQuery) (types.ListResult, error)

	// MarkCleanedUp records that the file for a soft-deleted entry has been
	// removed, or returns ErrNotFound if the entry is not pending cleanup.
	MarkCleanedUp(ctx context.Context, id uuid.UUID) error

	// Ping reports whether the connection is alive.
	Ping(ctx context.Context) error

	// Migrate creates the required tables and indexes, then validates the
	// result. It is idempotent. It never alters a table that already exists,
	// so an unrecognised schema is reported rather than changed.
	Migrate(ctx context.Context) error

	// Validate reports whether the existing schema matches what this package
	// expects. Deployments that migrate out of band use it instead of Migrate.
	Validate(ctx context.Context) error

	// Close releases the connection.
	Close() error
}

// Config addresses a metadata backend.
type Config struct {
	// Type is the backend to use: "sqlite" or "postgres".
	Type string
	// DSN is the backend-specific connection string.
	DSN string
	// Tables names the tables metadata is stored in.
	Tables types.Tables
}

// Connect opens the backend named by cfg.Type. It does not touch the schema;
// call Migrate or Validate for that.
func Connect(ctx context.Context, cfg Config) (Database, error) {
	if err := cfg.Tables.Validate(); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	switch cfg.Type {
	case "sqlite":
		return openSQLite(cfg.DSN, cfg.Tables)
	case "postgres":
		return openPostgres(ctx, cfg.DSN, cfg.Tables)
	default:
		return nil, fmt.Errorf("connect: unsupported database type: %s", cfg.Type)
	}
}

// metaColumns is the column list every read selects, in the order the
// scan helpers expect.
const metaColumns = `id, path, content_type, etag, file_size_bytes, created_at, updated_at`

// rowScanner is satisfied by the row and rows types of database/sql and pgx.
type rowScanner interface {
	Scan(dest ...any) error
}

// column is the type and nullability of a metadata column, as the backend's
// own catalogue reports it.
type column struct {
	dataType string
	nullable bool
}

// uniquePathColumn is the column Upsert's ON CONFLICT targets. Without a
// single-column unique constraint on it every write fails.
const uniquePathColumn = "path"

func errNoUniquePath(table string) error {
	return fmt.Errorf(
		"table %s has no single-column unique constraint on %s, which writes require",
		table, uniquePathColumn)
}

// checkColumns compares got against want and reports every discrepancy in a
// single error.
func checkColumns(table string, want, got map[string]column) error {
	var missing, mismatched []string

	for _, name := range slices.Sorted(maps.Keys(want)) {
		expected := want[name]
		actual, ok := got[name]
		switch {
		case !ok:
			missing = append(missing, name)
		case actual != expected:
			mismatched = append(mismatched, fmt.Sprintf("%s: expected %s nullable=%v, got %s nullable=%v",
				name, expected.dataType, expected.nullable, actual.dataType, actual.nullable))
		}
	}

	if missing == nil && mismatched == nil {
		return nil
	}

	var msg strings.Builder
	fmt.Fprintf(&msg, "table %s schema validation failed:", table)
	if missing != nil {
		fmt.Fprintf(&msg, "\n  missing columns: %s", strings.Join(missing, ", "))
	}
	for _, m := range mismatched {
		fmt.Fprintf(&msg, "\n  mismatched column %s", m)
	}

	return errors.New(msg.String())
}

// schemaNames holds the quoted identifiers a schema template renders into.
type schemaNames struct {
	Table               string
	DeletedIndex        string
	PendingCleanupIndex string
	ActiveListIndex     string
}

// renderSchema executes tmpl against the identifiers derived from table.
func renderSchema(tmpl *template.Template, table string, quote func(string) string) (string, error) {
	names := schemaNames{
		Table:               quote(table),
		DeletedIndex:        quote("idx_" + table + "_deleted_at"),
		PendingCleanupIndex: quote("idx_" + table + "_pending_cleanup"),
		ActiveListIndex:     quote("idx_" + table + "_active_list"),
	}

	var sql strings.Builder
	if err := tmpl.Execute(&sql, names); err != nil {
		return "", fmt.Errorf("render %s schema: %w", tmpl.Name(), err)
	}

	return sql.String(), nil
}
