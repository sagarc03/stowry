package postgres_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/database/postgres"
	"github.com/stretchr/testify/assert"
)

func TestConnect(t *testing.T) {
	pool := getSharedTestDatabase(t)
	dsn := getDSN(pool)
	ctx := context.Background()

	tables := stowry.Tables{MetaData: "metadata"}
	db, err := postgres.Connect(ctx, dsn, tables)
	assert.NoError(t, err)
	assert.NotNil(t, db)
	defer func() { _ = db.Close() }()

	// Verify connection is actually usable
	err = db.Ping(ctx)
	assert.NoError(t, err, "ping should succeed after connect")
}

func TestDatabase_Ping(t *testing.T) {
	pool := getSharedTestDatabase(t)
	dsn := getDSN(pool)
	ctx := context.Background()

	tables := stowry.Tables{MetaData: "ping_test"}
	db, err := postgres.Connect(ctx, dsn, tables)
	assert.NoError(t, err, "connect failed")
	defer func() { _ = db.Close() }()

	err = db.Ping(ctx)
	assert.NoError(t, err, "ping should succeed with valid connection")
}

func TestDatabase_Migrate(t *testing.T) {
	pool := getSharedTestDatabase(t)
	dsn := getDSN(pool)
	ctx := context.Background()

	t.Run("success - creates tables", func(t *testing.T) {
		tableName := "migrate_test_" + getRandomString(t)
		tables := stowry.Tables{MetaData: tableName}
		db, err := postgres.Connect(ctx, dsn, tables)
		assert.NoError(t, err)
		defer func() {
			_ = db.Close()
			_ = dropTable(ctx, pool, tableName)
		}()

		err = db.Migrate(ctx)
		assert.NoError(t, err, "migrate should succeed")

		// Verify table exists by trying to use the repo
		repo := db.GetRepo()
		_, err = repo.List(ctx, stowry.ListQuery{Limit: 1})
		assert.NoError(t, err, "repo should work after migration")
	})

	t.Run("idempotent - can run multiple times", func(t *testing.T) {
		tableName := "migrate_idem_" + getRandomString(t)
		tables := stowry.Tables{MetaData: tableName}
		db, err := postgres.Connect(ctx, dsn, tables)
		assert.NoError(t, err)
		defer func() {
			_ = db.Close()
			_ = dropTable(ctx, pool, tableName)
		}()

		err = db.Migrate(ctx)
		assert.NoError(t, err, "first migrate should succeed")

		err = db.Migrate(ctx)
		assert.NoError(t, err, "second migrate should succeed")
	})
}

func TestDatabase_Validate(t *testing.T) {
	pool := getSharedTestDatabase(t)
	dsn := getDSN(pool)
	ctx := context.Background()

	t.Run("success - valid schema after migrate", func(t *testing.T) {
		tableName := "validate_test_" + getRandomString(t)
		tables := stowry.Tables{MetaData: tableName}
		db, err := postgres.Connect(ctx, dsn, tables)
		assert.NoError(t, err)
		defer func() {
			_ = db.Close()
			_ = dropTable(ctx, pool, tableName)
		}()

		err = db.Migrate(ctx)
		assert.NoError(t, err)

		err = db.Validate(ctx)
		assert.NoError(t, err, "validate should succeed after migrate")
	})

	t.Run("error - table does not exist", func(t *testing.T) {
		tables := stowry.Tables{MetaData: "nonexistent_table"}
		db, err := postgres.Connect(ctx, dsn, tables)
		assert.NoError(t, err)
		defer func() { _ = db.Close() }()

		// Don't migrate - table won't exist
		err = db.Validate(ctx)
		assert.Error(t, err)
	})

	t.Run("error - missing columns", func(t *testing.T) {
		tableName := "incomplete_" + getRandomString(t)
		tables := stowry.Tables{MetaData: tableName}

		// Create table with missing columns using the pool directly
		_, err := pool.Exec(ctx, `
			CREATE TABLE `+tableName+` (
				id UUID PRIMARY KEY,
				path TEXT NOT NULL
			)
		`)
		assert.NoError(t, err)
		defer func() { _ = dropTable(ctx, pool, tableName) }()

		db, err := postgres.Connect(ctx, dsn, tables)
		assert.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = db.Validate(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing columns")
	})

	t.Run("error - wrong column type", func(t *testing.T) {
		tableName := "wrongtype_" + getRandomString(t)
		tables := stowry.Tables{MetaData: tableName}

		// Create table with wrong type (file_size_bytes as TEXT instead of BIGINT)
		_, err := pool.Exec(ctx, `
			CREATE TABLE `+tableName+` (
				id UUID PRIMARY KEY,
				path TEXT NOT NULL,
				content_type TEXT NOT NULL,
				etag TEXT NOT NULL,
				file_size_bytes TEXT NOT NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ,
				cleaned_up_at TIMESTAMPTZ
			)
		`)
		assert.NoError(t, err)
		defer func() { _ = dropTable(ctx, pool, tableName) }()

		db, err := postgres.Connect(ctx, dsn, tables)
		assert.NoError(t, err)
		defer func() { _ = db.Close() }()

		err = db.Validate(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file_size_bytes")
	})
}

func TestDatabase_GetRepo(t *testing.T) {
	pool := getSharedTestDatabase(t)
	dsn := getDSN(pool)
	ctx := context.Background()

	tableName := "getrepo_test_" + getRandomString(t)
	tables := stowry.Tables{MetaData: tableName}
	db, err := postgres.Connect(ctx, dsn, tables)
	assert.NoError(t, err)
	defer func() {
		_ = db.Close()
		_ = dropTable(ctx, pool, tableName)
	}()

	err = db.Migrate(ctx)
	assert.NoError(t, err)

	repo := db.GetRepo()
	assert.NotNil(t, repo, "GetRepo should return non-nil repo")

	// Verify repo implements the interface by using it
	entry := stowry.ObjectEntry{
		Path:        "/test.txt",
		Size:        100,
		ETag:        "etag",
		ContentType: "text/plain",
	}
	_, _, err = repo.Upsert(ctx, entry)
	assert.NoError(t, err, "repo should be functional")
}

func TestDatabase_Close(t *testing.T) {
	pool := getSharedTestDatabase(t)
	dsn := getDSN(pool)
	ctx := context.Background()

	tables := stowry.Tables{MetaData: "close_test"}
	db, err := postgres.Connect(ctx, dsn, tables)
	assert.NoError(t, err)

	err = db.Close()
	assert.NoError(t, err, "close should succeed")

	// After close, operations should fail
	err = db.Ping(ctx)
	assert.Error(t, err, "ping should fail after close")
}

// =============================================================================
// Repo Tests (via MetaDataRepo interface)
// =============================================================================

func TestRepo_Upsert(t *testing.T) {
	t.Run("insert - creates new entry", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entry := stowry.ObjectEntry{
			Path:        "/test/file.txt",
			Size:        1024,
			ETag:        "etag123",
			ContentType: "text/plain",
		}

		metadata, inserted, err := repo.Upsert(ctx, entry)
		assert.NoError(t, err, "expected no error, got: %v")
		assert.True(t, inserted, "expected inserted to be true for new entry")
		assert.Equal(t, entry.Path, metadata.Path, "expected path")
		assert.Equal(t, entry.Size, metadata.FileSizeBytes, "expected size")
		assert.Equal(t, entry.ETag, metadata.Etag, "expected etag")
		assert.Equal(t, entry.ContentType, metadata.ContentType, "expected content type")
		assert.NotEqual(t, uuid.Nil, metadata.ID, "expected non-nil UUID")
	})

	t.Run("update - updates existing entry", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entry1 := stowry.ObjectEntry{
			Path:        "/test/file.txt",
			Size:        1024,
			ETag:        "etag123",
			ContentType: "text/plain",
		}

		metadata1, inserted1, err := repo.Upsert(ctx, entry1)
		assert.NoError(t, err, "first upsert failed: %v")
		assert.True(t, inserted1, "expected first upsert to be insert")

		entry2 := stowry.ObjectEntry{
			Path:        "/test/file.txt",
			Size:        2048,
			ETag:        "etag456",
			ContentType: "application/octet-stream",
		}

		metadata2, inserted2, err := repo.Upsert(ctx, entry2)
		assert.NoError(t, err, "second upsert failed: %v")
		if inserted2 {
			t.Error("expected second upsert to be update")
		}
		assert.Equal(t, metadata1.ID, metadata2.ID, "ID should remain the same after update")
		assert.Equal(t, entry2.Size, metadata2.FileSizeBytes, "expected updated size")
		assert.Equal(t, entry2.ETag, metadata2.Etag, "expected updated etag")
		assert.Equal(t, entry2.ContentType, metadata2.ContentType, "expected updated content type")
	})

	t.Run("restore - undeletes soft-deleted entry", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entry := stowry.ObjectEntry{
			Path:        "/test/file.txt",
			Size:        1024,
			ETag:        "etag123",
			ContentType: "text/plain",
		}

		_, _, err := repo.Upsert(ctx, entry)
		assert.NoError(t, err, "first upsert failed: %v")

		err = repo.Delete(ctx, entry.Path)
		assert.NoError(t, err, "delete failed: %v")

		metadata, inserted, err := repo.Upsert(ctx, entry)
		assert.NoError(t, err, "upsert after delete failed: %v")
		if inserted {
			t.Error("expected upsert to be update (restore)")
		}
		assert.Equal(t, entry.Path, metadata.Path, "expected path")
	})
}

func TestRepo_Get(t *testing.T) {
	t.Run("success - gets existing entry", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entry := stowry.ObjectEntry{
			Path:        "/test/file.txt",
			Size:        1024,
			ETag:        "etag123",
			ContentType: "text/plain",
		}

		upsertedMeta, _, err := repo.Upsert(ctx, entry)
		assert.NoError(t, err, "upsert failed: %v")

		metadata, err := repo.Get(ctx, entry.Path)
		assert.NoError(t, err, "expected no error, got: %v")
		assert.Equal(t, entry.Path, metadata.Path, "expected path")
		assert.Equal(t, upsertedMeta.ID, metadata.ID, "expected ID")
	})

	t.Run("error - not found", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		_, err := repo.Get(ctx, "/nonexistent/file.txt")
		assert.Error(t, err, "expected error for nonexistent file")
		assert.ErrorIs(t, err, stowry.ErrNotFound, "expected ErrNotFound")
	})

	t.Run("error - returns not found for deleted entry", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entry := stowry.ObjectEntry{
			Path:        "/test/file.txt",
			Size:        1024,
			ETag:        "etag123",
			ContentType: "text/plain",
		}

		_, _, err := repo.Upsert(ctx, entry)
		assert.NoError(t, err, "upsert failed: %v")

		err = repo.Delete(ctx, entry.Path)
		assert.NoError(t, err, "delete failed: %v")

		_, err = repo.Get(ctx, entry.Path)
		assert.Error(t, err, "expected error for deleted file")
		assert.ErrorIs(t, err, stowry.ErrNotFound, "expected ErrNotFound")
	})
}

func TestRepo_Delete(t *testing.T) {
	t.Run("success - soft deletes existing entry", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entry := stowry.ObjectEntry{
			Path:        "/test/file.txt",
			Size:        1024,
			ETag:        "etag123",
			ContentType: "text/plain",
		}

		_, _, err := repo.Upsert(ctx, entry)
		assert.NoError(t, err, "upsert failed: %v")

		err = repo.Delete(ctx, entry.Path)
		assert.NoError(t, err, "expected no error, got: %v")

		_, err = repo.Get(ctx, entry.Path)
		assert.Error(t, err, "expected error when getting deleted entry")
	})

	t.Run("error - not found", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		err := repo.Delete(ctx, "/nonexistent/file.txt")
		assert.Error(t, err, "expected error for nonexistent file")
		assert.ErrorIs(t, err, stowry.ErrNotFound, "expected ErrNotFound")
	})

	t.Run("error - already deleted", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entry := stowry.ObjectEntry{
			Path:        "/test/file.txt",
			Size:        1024,
			ETag:        "etag123",
			ContentType: "text/plain",
		}

		_, _, err := repo.Upsert(ctx, entry)
		assert.NoError(t, err, "upsert failed: %v")

		err = repo.Delete(ctx, entry.Path)
		assert.NoError(t, err, "first delete failed: %v")

		err = repo.Delete(ctx, entry.Path)
		assert.Error(t, err, "expected error for already deleted file")
		assert.ErrorIs(t, err, stowry.ErrNotFound, "expected ErrNotFound")
	})
}

func TestRepo_List(t *testing.T) {
	t.Run("success - lists all entries", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entries := []stowry.ObjectEntry{
			{Path: "/a/file1.txt", Size: 100, ETag: "etag1", ContentType: "text/plain"},
			{Path: "/b/file2.txt", Size: 200, ETag: "etag2", ContentType: "text/plain"},
			{Path: "/c/file3.txt", Size: 300, ETag: "etag3", ContentType: "text/plain"},
		}

		for _, entry := range entries {
			if _, _, err := repo.Upsert(ctx, entry); err != nil {
				t.Fatalf("upsert failed: %v", err)
			}
		}

		result, err := repo.List(ctx, stowry.ListQuery{PathPrefix: "/", Limit: 10})
		assert.NoError(t, err, "expected no error, got: %v")
		if len(result.Items) != 3 {
			t.Errorf("expected 3 items, got %d", len(result.Items))
		}
		if result.NextCursor != "" {
			t.Errorf("expected empty cursor, got %s", result.NextCursor)
		}
	})

	t.Run("success - filters by path prefix", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entries := []stowry.ObjectEntry{
			{Path: "/images/photo1.jpg", Size: 100, ETag: "etag1", ContentType: "image/jpeg"},
			{Path: "/images/photo2.jpg", Size: 200, ETag: "etag2", ContentType: "image/jpeg"},
			{Path: "/docs/readme.txt", Size: 300, ETag: "etag3", ContentType: "text/plain"},
		}

		for _, entry := range entries {
			if _, _, err := repo.Upsert(ctx, entry); err != nil {
				t.Fatalf("upsert failed: %v", err)
			}
		}

		result, err := repo.List(ctx, stowry.ListQuery{PathPrefix: "/images/", Limit: 10})
		assert.NoError(t, err, "expected no error, got: %v")
		if len(result.Items) != 2 {
			t.Errorf("expected 2 items, got %d", len(result.Items))
		}
		for _, item := range result.Items {
			if item.Path[:8] != "/images/" {
				t.Errorf("expected path to start with /images/, got %s", item.Path)
			}
		}
	})

	t.Run("success - pagination with cursor", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entries := []stowry.ObjectEntry{
			{Path: "/file1.txt", Size: 100, ETag: "etag1", ContentType: "text/plain"},
			{Path: "/file2.txt", Size: 200, ETag: "etag2", ContentType: "text/plain"},
			{Path: "/file3.txt", Size: 300, ETag: "etag3", ContentType: "text/plain"},
		}

		for _, entry := range entries {
			if _, _, err := repo.Upsert(ctx, entry); err != nil {
				t.Fatalf("upsert failed: %v", err)
			}
		}

		result, err := repo.List(ctx, stowry.ListQuery{PathPrefix: "/", Limit: 2})
		assert.NoError(t, err, "expected no error, got: %v")
		if len(result.Items) != 2 {
			t.Errorf("expected 2 items, got %d", len(result.Items))
		}
		if result.NextCursor == "" {
			t.Error("expected non-empty cursor")
		}

		result2, err := repo.List(ctx, stowry.ListQuery{PathPrefix: "/", Limit: 2, Cursor: result.NextCursor})
		assert.NoError(t, err, "expected no error on second page, got: %v")
		if len(result2.Items) != 1 {
			t.Errorf("expected 1 item on second page, got %d", len(result2.Items))
		}
		if result2.NextCursor != "" {
			t.Error("expected empty cursor on last page")
		}

		for _, item1 := range result.Items {
			for _, item2 := range result2.Items {
				if item1.ID == item2.ID {
					t.Error("items should not appear in multiple pages")
				}
			}
		}
	})

	t.Run("success - excludes deleted entries", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entries := []stowry.ObjectEntry{
			{Path: "/file1.txt", Size: 100, ETag: "etag1", ContentType: "text/plain"},
			{Path: "/file2.txt", Size: 200, ETag: "etag2", ContentType: "text/plain"},
		}

		for _, entry := range entries {
			_, _, err := repo.Upsert(ctx, entry)
			assert.NoError(t, err, "upsert failed")
		}

		err := repo.Delete(ctx, "/file1.txt")
		assert.NoError(t, err, "delete failed: %v")

		result, err := repo.List(ctx, stowry.ListQuery{PathPrefix: "/", Limit: 10})
		assert.NoError(t, err)
		assert.Len(t, result.Items, 1)
		if len(result.Items) > 0 {
			assert.Equal(t, "/file2.txt", result.Items[0].Path)
		}
	})

	t.Run("success - empty result", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		result, err := repo.List(ctx, stowry.ListQuery{PathPrefix: "/", Limit: 10})
		assert.NoError(t, err, "expected no error, got: %v")
		if len(result.Items) != 0 {
			t.Errorf("expected 0 items, got %d", len(result.Items))
		}
		if result.NextCursor != "" {
			t.Errorf("expected empty cursor, got %s", result.NextCursor)
		}
	})

	t.Run("success - escapes LIKE special characters in prefix", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entries := []stowry.ObjectEntry{
			{Path: "/foo%bar/file.txt", Size: 100, ETag: "etag1", ContentType: "text/plain"},
			{Path: "/foo_bar/file.txt", Size: 200, ETag: "etag2", ContentType: "text/plain"},
			{Path: "/fooXbar/file.txt", Size: 300, ETag: "etag3", ContentType: "text/plain"},
		}

		for _, entry := range entries {
			if _, _, err := repo.Upsert(ctx, entry); err != nil {
				t.Fatalf("upsert failed: %v", err)
			}
		}

		// Without escaping, % would match any character sequence
		result, err := repo.List(ctx, stowry.ListQuery{PathPrefix: "/foo%bar/", Limit: 10})
		assert.NoError(t, err)
		assert.Len(t, result.Items, 1, "expected only literal match for %%")
		if len(result.Items) > 0 {
			assert.Equal(t, "/foo%bar/file.txt", result.Items[0].Path)
		}

		// Without escaping, _ would match any single character
		result, err = repo.List(ctx, stowry.ListQuery{PathPrefix: "/foo_bar/", Limit: 10})
		assert.NoError(t, err)
		assert.Len(t, result.Items, 1, "expected only literal match for _")
		if len(result.Items) > 0 {
			assert.Equal(t, "/foo_bar/file.txt", result.Items[0].Path)
		}
	})
}

func TestRepo_ListPendingCleanup(t *testing.T) {
	t.Run("success - lists deleted entries", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entries := []stowry.ObjectEntry{
			{Path: "/file1.txt", Size: 100, ETag: "etag1", ContentType: "text/plain"},
			{Path: "/file2.txt", Size: 200, ETag: "etag2", ContentType: "text/plain"},
			{Path: "/file3.txt", Size: 300, ETag: "etag3", ContentType: "text/plain"},
		}

		for _, entry := range entries {
			if _, _, err := repo.Upsert(ctx, entry); err != nil {
				t.Fatalf("upsert failed: %v", err)
			}
		}

		err := repo.Delete(ctx, "/file1.txt")
		assert.NoError(t, err, "delete failed: %v")

		err = repo.Delete(ctx, "/file2.txt")
		assert.NoError(t, err, "delete failed: %v")

		result, err := repo.ListPendingCleanup(ctx, stowry.ListQuery{PathPrefix: "/", Limit: 10})
		assert.NoError(t, err, "expected no error, got: %v")
		if len(result.Items) != 2 {
			t.Errorf("expected 2 items, got %d", len(result.Items))
		}
	})

	t.Run("success - excludes active entries", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entries := []stowry.ObjectEntry{
			{Path: "/file1.txt", Size: 100, ETag: "etag1", ContentType: "text/plain"},
			{Path: "/file2.txt", Size: 200, ETag: "etag2", ContentType: "text/plain"},
		}

		for _, entry := range entries {
			if _, _, err := repo.Upsert(ctx, entry); err != nil {
				t.Fatalf("upsert failed: %v", err)
			}
		}

		err := repo.Delete(ctx, "/file1.txt")
		assert.NoError(t, err, "delete failed: %v")

		result, err := repo.ListPendingCleanup(ctx, stowry.ListQuery{PathPrefix: "/", Limit: 10})
		assert.NoError(t, err)
		assert.Len(t, result.Items, 1)
		if len(result.Items) > 0 {
			assert.Equal(t, "/file1.txt", result.Items[0].Path)
		}
	})

	t.Run("success - excludes cleaned up entries", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entries := []stowry.ObjectEntry{
			{Path: "/file1.txt", Size: 100, ETag: "etag1", ContentType: "text/plain"},
			{Path: "/file2.txt", Size: 200, ETag: "etag2", ContentType: "text/plain"},
		}

		var meta1 stowry.MetaData
		for i, entry := range entries {
			m, _, err := repo.Upsert(ctx, entry)
			assert.NoError(t, err, "upsert failed: %v")
			if i == 0 {
				meta1 = m
			}
		}

		err := repo.Delete(ctx, "/file1.txt")
		assert.NoError(t, err, "delete file1 failed: %v")

		err = repo.Delete(ctx, "/file2.txt")
		assert.NoError(t, err, "delete file2 failed: %v")

		err = repo.MarkCleanedUp(ctx, meta1.ID)
		assert.NoError(t, err, "mark cleaned up failed: %v")

		result, err := repo.ListPendingCleanup(ctx, stowry.ListQuery{PathPrefix: "/", Limit: 10})
		assert.NoError(t, err)
		assert.Len(t, result.Items, 1)
		if len(result.Items) > 0 {
			assert.Equal(t, "/file2.txt", result.Items[0].Path)
		}
	})
}

func TestRepo_MarkCleanedUp(t *testing.T) {
	t.Run("success - marks deleted entry as cleaned up", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entry := stowry.ObjectEntry{
			Path:        "/file1.txt",
			Size:        100,
			ETag:        "etag1",
			ContentType: "text/plain",
		}

		metadata, _, err := repo.Upsert(ctx, entry)
		assert.NoError(t, err, "upsert failed: %v")

		err = repo.Delete(ctx, entry.Path)
		assert.NoError(t, err, "delete failed: %v")

		err = repo.MarkCleanedUp(ctx, metadata.ID)
		assert.NoError(t, err, "expected no error, got: %v")

		result, err := repo.ListPendingCleanup(ctx, stowry.ListQuery{PathPrefix: "/", Limit: 10})
		assert.NoError(t, err, "list pending cleanup failed: %v")
		if len(result.Items) != 0 {
			t.Errorf("expected 0 items after cleanup, got %d", len(result.Items))
		}
	})

	t.Run("error - not found for non-existent id", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		err := repo.MarkCleanedUp(ctx, uuid.New())
		assert.Error(t, err, "expected error for non-existent id")
		assert.ErrorIs(t, err, stowry.ErrNotFound, "expected ErrNotFound")
	})

	t.Run("error - not found for active entry", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entry := stowry.ObjectEntry{
			Path:        "/file1.txt",
			Size:        100,
			ETag:        "etag1",
			ContentType: "text/plain",
		}

		metadata, _, err := repo.Upsert(ctx, entry)
		assert.NoError(t, err, "upsert failed: %v")

		err = repo.MarkCleanedUp(ctx, metadata.ID)
		assert.Error(t, err, "expected error for active entry")
		assert.ErrorIs(t, err, stowry.ErrNotFound, "expected ErrNotFound")
	})

	t.Run("error - already cleaned up", func(t *testing.T) {
		repo, cleanup := setupTestRepo(t)
		defer cleanup()

		ctx := context.Background()

		entry := stowry.ObjectEntry{
			Path:        "/file1.txt",
			Size:        100,
			ETag:        "etag1",
			ContentType: "text/plain",
		}

		metadata, _, err := repo.Upsert(ctx, entry)
		assert.NoError(t, err, "upsert failed: %v")

		err = repo.Delete(ctx, entry.Path)
		assert.NoError(t, err, "delete failed: %v")

		err = repo.MarkCleanedUp(ctx, metadata.ID)
		assert.NoError(t, err, "first mark cleaned up failed: %v")

		err = repo.MarkCleanedUp(ctx, metadata.ID)
		assert.Error(t, err, "expected error for already cleaned up entry")
		assert.ErrorIs(t, err, stowry.ErrNotFound, "expected ErrNotFound")
	})
}
