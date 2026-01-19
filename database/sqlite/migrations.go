package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sagarc03/stowry"
)

// quoteIdentifier safely quotes a SQLite identifier
func quoteIdentifier(name string) string {
	return `"` + name + `"`
}

type TableMigration struct {
	TableName string
	Up        func(ctx context.Context, db *sql.DB) error
	Down      func(ctx context.Context, db *sql.DB) error
}

// getTableMigrations returns all table migrations for the app
func getTableMigrations(tables stowry.Tables) []TableMigration {
	migrations := []TableMigration{}

	migrations = append(migrations, TableMigration{
		TableName: tables.MetaData,
		Up:        createMetaTable(tables.MetaData),
		Down:      dropTable(tables.MetaData),
	})

	return migrations
}

func Migrate(ctx context.Context, db *sql.DB, tables stowry.Tables) error {
	migrations := getTableMigrations(tables)

	for _, migration := range migrations {
		if err := migration.Up(ctx, db); err != nil {
			return fmt.Errorf("migrate up %s: %w", migration.TableName, err)
		}
	}

	return nil
}

func DropTables(ctx context.Context, db *sql.DB, tables stowry.Tables) error {
	migrations := getTableMigrations(tables)

	for i := len(migrations) - 1; i >= 0; i-- {
		migration := migrations[i]
		if err := migration.Down(ctx, db); err != nil {
			return fmt.Errorf("migrate down %s: %w", migration.TableName, err)
		}
	}

	return nil
}

func createMetaTable(tableName string) func(context.Context, *sql.DB) error {
	return func(ctx context.Context, db *sql.DB) error {
		quotedTable := quoteIdentifier(tableName)
		indexDeletedAt := quoteIdentifier(fmt.Sprintf("idx_%s_deleted_at", tableName))
		indexPendingCleanup := quoteIdentifier(fmt.Sprintf("idx_%s_pending_cleanup", tableName))
		indexActiveList := quoteIdentifier(fmt.Sprintf("idx_%s_active_list", tableName))

		// Create table
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

		// Create indexes (SQLite doesn't support partial indexes with WHERE in older versions,
		// but SQLite 3.8.0+ does support them)
		indexSQL := fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS %s ON %s (deleted_at)
		`, indexDeletedAt, quotedTable)

		if _, err := db.ExecContext(ctx, indexSQL); err != nil {
			return fmt.Errorf("create index deleted_at: %w", err)
		}

		indexSQL = fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS %s ON %s (deleted_at, cleaned_up_at)
		`, indexPendingCleanup, quotedTable)

		if _, err := db.ExecContext(ctx, indexSQL); err != nil {
			return fmt.Errorf("create index pending_cleanup: %w", err)
		}

		indexSQL = fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS %s ON %s (created_at, path)
		`, indexActiveList, quotedTable)

		if _, err := db.ExecContext(ctx, indexSQL); err != nil {
			return fmt.Errorf("create index active_list: %w", err)
		}

		return nil
	}
}

func dropTable(tableName string) func(context.Context, *sql.DB) error {
	return func(ctx context.Context, db *sql.DB) error {
		quotedTable := quoteIdentifier(tableName)
		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s", quotedTable)

		_, err := db.ExecContext(ctx, dropSQL)
		return err
	}
}
