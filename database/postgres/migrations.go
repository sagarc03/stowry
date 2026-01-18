package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TableMigration struct {
	TableName string
	Up        func(ctx context.Context, pool *pgxpool.Pool) error
	Down      func(ctx context.Context, pool *pgxpool.Pool) error
}

// getTableMigrations returns all table migrations for the app
func getTableMigrations(tables Tables) []TableMigration {
	migrations := []TableMigration{}

	migrations = append(migrations, TableMigration{
		TableName: tables.MetaData,
		Up:        createMetaTable(tables.MetaData),
		Down:      dropTable(tables.MetaData),
	})

	// Future tables would be added here:
	// if usersTableName, ok := tableNames["users"]; ok { ... }

	return migrations
}

func Migrate(ctx context.Context, pool *pgxpool.Pool, tables Tables) error {
	migrations := getTableMigrations(tables)

	for _, migration := range migrations {
		if err := migration.Up(ctx, pool); err != nil {
			return fmt.Errorf("migrate up %s: %w", migration.TableName, err)
		}
	}

	return nil
}

func DropTables(ctx context.Context, pool *pgxpool.Pool, tables Tables) error {
	migrations := getTableMigrations(tables)

	for i := len(migrations) - 1; i >= 0; i-- {
		migration := migrations[i]
		if err := migration.Down(ctx, pool); err != nil {
			return fmt.Errorf("migrate down %s: %w", migration.TableName, err)
		}
	}

	return nil
}

func createMetaTable(tableName string) func(context.Context, *pgxpool.Pool) error {
	return func(ctx context.Context, pool *pgxpool.Pool) error {
		quotedTable := pgx.Identifier{tableName}.Sanitize()
		indexDeletedAt := pgx.Identifier{fmt.Sprintf("idx_%s_deleted_at", tableName)}.Sanitize()
		indexPendingCleanup := pgx.Identifier{fmt.Sprintf("idx_%s_pending_cleanup", tableName)}.Sanitize()
		indexActiveList := pgx.Identifier{fmt.Sprintf("idx_%s_active_list", tableName)}.Sanitize()

		sql := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
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

			CREATE INDEX IF NOT EXISTS %s
			ON %s (deleted_at)
			WHERE (deleted_at IS NOT NULL);

			CREATE INDEX IF NOT EXISTS %s
			ON %s (deleted_at, cleaned_up_at)
			WHERE (deleted_at IS NOT NULL AND cleaned_up_at IS NULL);

			CREATE INDEX IF NOT EXISTS %s
			ON %s (created_at, path)
			WHERE (deleted_at IS NULL);
		`,
			quotedTable,
			indexDeletedAt, quotedTable,
			indexPendingCleanup, quotedTable,
			indexActiveList, quotedTable,
		)

		_, err := pool.Exec(ctx, sql)
		return err
	}
}

func dropTable(tableName string) func(context.Context, *pgxpool.Pool) error {
	return func(ctx context.Context, pool *pgxpool.Pool) error {
		quotedTable := pgx.Identifier{tableName}.Sanitize()
		sql := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", quotedTable)

		_, err := pool.Exec(ctx, sql)
		return err
	}
}
