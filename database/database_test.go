package database_test

import (
	"context"
	"testing"

	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers

func newTestConfig(tableName string) database.Config {
	return database.Config{
		Type:   "sqlite",
		DSN:    ":memory:",
		Tables: stowry.Tables{MetaData: tableName},
	}
}

func setupTestDB(t *testing.T, tableName string) database.Database {
	t.Helper()
	ctx := context.Background()

	db, err := database.Connect(ctx, newTestConfig(tableName))
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })

	return db
}

func setupTestDBWithMigration(t *testing.T, tableName string) database.Database {
	t.Helper()
	ctx := context.Background()

	db := setupTestDB(t, tableName)

	err := db.Migrate(ctx)
	require.NoError(t, err)

	return db
}

// Tests for Connect routing logic

func TestConnect_SQLite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db := setupTestDB(t, "test_metadata")

	err := db.Ping(ctx)
	assert.NoError(t, err)
}

func TestConnect_InvalidType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cfg := database.Config{
		Type:   "invalid",
		DSN:    "whatever",
		Tables: stowry.Tables{MetaData: "test_metadata"},
	}

	_, err := database.Connect(ctx, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database type")
}

func TestConnect_EmptyType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cfg := database.Config{
		Type:   "",
		DSN:    ":memory:",
		Tables: stowry.Tables{MetaData: "test_metadata"},
	}

	_, err := database.Connect(ctx, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database type")
}

// Tests for Database interface methods

func TestDatabase_Ping(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db := setupTestDB(t, "ping_test")

	err := db.Ping(ctx)
	assert.NoError(t, err)
}

func TestDatabase_Migrate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db := setupTestDB(t, "migrate_test")

	err := db.Migrate(ctx)
	require.NoError(t, err)

	repo := db.GetRepo()
	require.NotNil(t, repo)

	// Verify table works
	_, err = repo.List(ctx, stowry.ListQuery{Limit: 1})
	assert.NoError(t, err)
}

func TestDatabase_Migrate_Idempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db := setupTestDB(t, "migrate_idem_test")

	err := db.Migrate(ctx)
	require.NoError(t, err)

	err = db.Migrate(ctx)
	assert.NoError(t, err, "migrate should be idempotent")
}

func TestDatabase_Validate_BeforeMigration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db := setupTestDB(t, "validate_before_test")

	err := db.Validate(ctx)
	assert.Error(t, err, "validate should fail without tables")
}

func TestDatabase_Validate_AfterMigration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db := setupTestDBWithMigration(t, "validate_after_test")

	err := db.Validate(ctx)
	assert.NoError(t, err, "validate should pass after migration")
}

func TestDatabase_GetRepo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db := setupTestDBWithMigration(t, "getrepo_test")

	repo := db.GetRepo()
	require.NotNil(t, repo)

	entry := stowry.ObjectEntry{
		Path:        "test/file.txt",
		Size:        100,
		ETag:        "abc123",
		ContentType: "text/plain",
	}

	meta, inserted, err := repo.Upsert(ctx, entry)
	require.NoError(t, err)
	assert.True(t, inserted)
	assert.Equal(t, "test/file.txt", meta.Path)
}

func TestDatabase_Close(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cfg := newTestConfig("close_test")
	db, err := database.Connect(ctx, cfg)
	require.NoError(t, err)

	err = db.Close()
	assert.NoError(t, err)

	err = db.Ping(ctx)
	assert.Error(t, err, "ping should fail after close")
}

func TestDatabase_Repo_List(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db := setupTestDBWithMigration(t, "list_test")
	repo := db.GetRepo()

	entries := []stowry.ObjectEntry{
		{Path: "images/a.jpg", Size: 100, ETag: "a", ContentType: "image/jpeg"},
		{Path: "images/b.jpg", Size: 200, ETag: "b", ContentType: "image/jpeg"},
		{Path: "docs/readme.md", Size: 50, ETag: "c", ContentType: "text/markdown"},
	}

	for _, e := range entries {
		_, _, err := repo.Upsert(ctx, e)
		require.NoError(t, err)
	}

	t.Run("with prefix filter", func(t *testing.T) {
		result, err := repo.List(ctx, stowry.ListQuery{PathPrefix: "images/", Limit: 10})
		require.NoError(t, err)
		assert.Len(t, result.Items, 2)
	})

	t.Run("without filter", func(t *testing.T) {
		result, err := repo.List(ctx, stowry.ListQuery{Limit: 10})
		require.NoError(t, err)
		assert.Len(t, result.Items, 3)
	})
}

// Note: Postgres-specific tests are in database/postgres package.
// The Connect function's postgres routing is implicitly tested there.
