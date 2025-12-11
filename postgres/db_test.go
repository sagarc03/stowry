package postgres_test

// Schema validation tests verify that ValidateSchema works correctly.
// ValidateSchema is used when users manually migrate their database and need schema verification.
//
// Adding New Table Validators:
// When you add a new table:
// 1. Add the schema map in db.go (e.g., usersTableSchema)
// 2. Add entry to getTableValidations() in db.go
// 3. ValidateSchema() will automatically validate the new table
// 4. Tests automatically cover it

import (
	"context"
	"testing"

	"github.com/sagarc03/stowry/postgres"
	"github.com/stretchr/testify/assert"
)

func TestValidateSchema(t *testing.T) {
	t.Run("success - all tables valid", func(t *testing.T) {
		pool, cleanup := getTestDatabase(t)
		defer cleanup()
		defer pool.Close()

		ctx := context.Background()
		tables := postgres.Tables{MetaData: "metadata"}

		err := postgres.Migrate(ctx, pool, tables)
		assert.NoError(t, err, "failed to migrate")

		err = postgres.ValidateSchema(ctx, pool, tables)
		assert.NoError(t, err, "expected no error for valid schema")
	})

	t.Run("error - table does not exist", func(t *testing.T) {
		pool, cleanup := getTestDatabase(t)
		defer cleanup()
		defer pool.Close()

		ctx := context.Background()
		tables := postgres.Tables{MetaData: "nonexistent_table"}

		err := postgres.ValidateSchema(ctx, pool, tables)
		assert.Error(t, err, "expected error for nonexistent table")
	})

	t.Run("error - table has incomplete schema", func(t *testing.T) {
		pool, cleanup := getTestDatabase(t)
		defer cleanup()
		defer pool.Close()

		ctx := context.Background()
		tables := postgres.Tables{MetaData: "incomplete_metadata"}

		_, err := pool.Exec(ctx, `
			CREATE TABLE incomplete_metadata (
				id UUID PRIMARY KEY,
				path TEXT NOT NULL
			)
		`)
		assert.NoError(t, err, "failed to create test table")

		err = postgres.ValidateSchema(ctx, pool, tables)
		assert.Error(t, err, "expected error for incomplete schema")
	})

	t.Run("error - wrong column types", func(t *testing.T) {
		pool, cleanup := getTestDatabase(t)
		defer cleanup()
		defer pool.Close()

		ctx := context.Background()
		tables := postgres.Tables{MetaData: "wrong_type_metadata"}

		_, err := pool.Exec(ctx, `
			CREATE TABLE wrong_type_metadata (
				id UUID PRIMARY KEY,
				path TEXT NOT NULL,
				content_type TEXT NOT NULL,
				etag TEXT NOT NULL,
				file_size_bytes TEXT NOT NULL,
				created_at TIMESTAMPTZ NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL,
				deleted_at TIMESTAMPTZ,
				cleaned_up_at TIMESTAMPTZ
			)
		`)
		assert.NoError(t, err, "failed to create test table")

		err = postgres.ValidateSchema(ctx, pool, tables)
		assert.Error(t, err, "expected error for wrong column types")
	})

	t.Run("error - wrong nullable constraints", func(t *testing.T) {
		pool, cleanup := getTestDatabase(t)
		defer cleanup()
		defer pool.Close()

		ctx := context.Background()
		tables := postgres.Tables{MetaData: "wrong_nullable_metadata"}

		_, err := pool.Exec(ctx, `
			CREATE TABLE wrong_nullable_metadata (
				id UUID PRIMARY KEY,
				path TEXT,
				content_type TEXT NOT NULL,
				etag TEXT NOT NULL,
				file_size_bytes BIGINT NOT NULL,
				created_at TIMESTAMPTZ NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL,
				deleted_at TIMESTAMPTZ,
				cleaned_up_at TIMESTAMPTZ
			)
		`)
		assert.NoError(t, err, "failed to create test table")

		err = postgres.ValidateSchema(ctx, pool, tables)
		assert.Error(t, err, "expected error for wrong nullable constraint")
	})

	t.Run("success - tables with extra columns are valid", func(t *testing.T) {
		pool, cleanup := getTestDatabase(t)
		defer cleanup()
		defer pool.Close()

		ctx := context.Background()
		tables := postgres.Tables{MetaData: "metadata_extended"}

		err := postgres.Migrate(ctx, pool, tables)
		assert.NoError(t, err, "failed to migrate")

		_, err = pool.Exec(ctx, `
			ALTER TABLE metadata_extended 
			ADD COLUMN custom_field TEXT,
			ADD COLUMN user_metadata JSONB
		`)
		assert.NoError(t, err, "failed to add extra columns")

		err = postgres.ValidateSchema(ctx, pool, tables)
		assert.NoError(t, err, "expected no error for tables with extra columns")
	})

	t.Run("success - validates after migrate and drop cycle", func(t *testing.T) {
		pool, cleanup := getTestDatabase(t)
		defer cleanup()
		defer pool.Close()

		ctx := context.Background()
		tables := postgres.Tables{MetaData: "metadata"}

		err := postgres.Migrate(ctx, pool, tables)
		assert.NoError(t, err, "first migrate failed")

		err = postgres.ValidateSchema(ctx, pool, tables)
		assert.NoError(t, err, "validation failed after first migrate")

		err = postgres.DropTables(ctx, pool, tables)
		assert.NoError(t, err, "drop tables failed")

		err = postgres.Migrate(ctx, pool, tables)
		assert.NoError(t, err, "second migrate failed")

		err = postgres.ValidateSchema(ctx, pool, tables)
		assert.NoError(t, err, "validation failed after second migrate")
	})
}
