package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

// quoteIdentifier safely quotes a SQLite identifier.
func quoteIdentifier(name string) string {
	return `"` + name + `"`
}

func createMetaTable(ctx context.Context, db *sql.DB, tableName string) error {
	quotedTable := quoteIdentifier(tableName)
	indexDeletedAt := quoteIdentifier(fmt.Sprintf("idx_%s_deleted_at", tableName))
	indexPendingCleanup := quoteIdentifier(fmt.Sprintf("idx_%s_pending_cleanup", tableName))
	indexActiveList := quoteIdentifier(fmt.Sprintf("idx_%s_active_list", tableName))

	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT NOT NULL PRIMARY KEY,
			path TEXT NOT NULL UNIQUE,
			content_type TEXT NOT NULL,
			etag TEXT NOT NULL,
			file_size_bytes INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			deleted_at TEXT,
			cleaned_up_at TEXT
		)
	`, quotedTable)

	if _, err := db.ExecContext(ctx, createTableSQL); err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	indexSQL := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (deleted_at)`, indexDeletedAt, quotedTable)
	if _, err := db.ExecContext(ctx, indexSQL); err != nil {
		return fmt.Errorf("create index deleted_at: %w", err)
	}

	indexSQL = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (deleted_at, cleaned_up_at)`, indexPendingCleanup, quotedTable)
	if _, err := db.ExecContext(ctx, indexSQL); err != nil {
		return fmt.Errorf("create index pending_cleanup: %w", err)
	}

	indexSQL = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (created_at, path)`, indexActiveList, quotedTable)
	if _, err := db.ExecContext(ctx, indexSQL); err != nil {
		return fmt.Errorf("create index active_list: %w", err)
	}

	return nil
}
