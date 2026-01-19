package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func createMetaTable(ctx context.Context, pool *pgxpool.Pool, tableName string) error {
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
	if err != nil {
		return fmt.Errorf("create meta table: %w", err)
	}
	return nil
}
