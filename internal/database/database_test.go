package database_test

import (
	"testing"
	"uuid"

	"github.com/sagarc03/stowry/internal/database"
	"github.com/sagarc03/stowry/internal/service"
	"github.com/sagarc03/stowry/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Database is not declared in terms of service.MetaDataRepo, so this assertion
// is what catches the two drifting apart.
var _ service.MetaDataRepo = (database.Database)(nil)

func TestConnect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cfg         database.Config
		errContains string
	}{
		{
			name:        "unsupported type",
			cfg:         database.Config{Type: "mysql", DSN: "whatever", Tables: types.Tables{MetaData: "metadata"}},
			errContains: "unsupported database type",
		},
		{
			name:        "empty type",
			cfg:         database.Config{Type: "", DSN: ":memory:", Tables: types.Tables{MetaData: "metadata"}},
			errContains: "unsupported database type",
		},
		{
			name:        "empty table name",
			cfg:         database.Config{Type: "sqlite", DSN: ":memory:"},
			errContains: "cannot be empty",
		},
		{
			name:        "table name that is not an identifier",
			cfg:         database.Config{Type: "sqlite", DSN: ":memory:", Tables: types.Tables{MetaData: "meta; DROP TABLE x"}},
			errContains: "invalid metadata table name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, err := database.Connect(t.Context(), tt.cfg)
			assert.ErrorContains(t, err, tt.errContains)
			assert.Nil(t, db)
		})
	}
}

func TestDatabase(t *testing.T) {
	t.Parallel()

	for _, b := range backends() {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()

			t.Run("ping", func(t *testing.T) {
				db := b.open(t).db
				assert.NoError(t, db.Ping(t.Context()))
			})

			t.Run("migrate creates a usable table", func(t *testing.T) {
				db := b.migrated(t)

				_, err := db.List(t.Context(), types.ListQuery{Limit: 1})
				assert.NoError(t, err)
			})

			t.Run("migrate is idempotent", func(t *testing.T) {
				db := b.migrated(t)
				assert.NoError(t, db.Migrate(t.Context()))
			})

			t.Run("validate fails before migrate", func(t *testing.T) {
				db := b.open(t).db

				err := db.Validate(t.Context())
				assert.ErrorContains(t, err, "does not exist")
			})

			t.Run("validate passes after migrate", func(t *testing.T) {
				db := b.migrated(t)
				assert.NoError(t, db.Validate(t.Context()))
			})

			for _, bad := range b.badSchemas {
				t.Run("validate rejects "+bad.name, func(t *testing.T) {
					f := b.open(t)
					f.exec(t, bad.ddl(f.table))

					err := f.db.Validate(t.Context())
					assert.ErrorContains(t, err, bad.errContains)
				})
			}

			t.Run("close ends the connection", func(t *testing.T) {
				db := b.open(t).db

				require.NoError(t, db.Close())
				assert.Error(t, db.Ping(t.Context()), "ping should fail after close")
			})
		})
	}
}

func TestRepo(t *testing.T) {
	t.Parallel()

	for _, b := range backends() {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()

			t.Run("upsert", func(t *testing.T) { testUpsert(t, b) })
			t.Run("get", func(t *testing.T) { testGet(t, b) })
			t.Run("delete", func(t *testing.T) { testDelete(t, b) })
			t.Run("list", func(t *testing.T) { testList(t, b) })
			t.Run("list pending cleanup", func(t *testing.T) { testListPendingCleanup(t, b) })
			t.Run("mark cleaned up", func(t *testing.T) { testMarkCleanedUp(t, b) })
		})
	}
}

func testUpsert(t *testing.T, b backend) {
	entry := types.ObjectEntry{Path: "/test/file.txt", Size: 1024, ETag: "etag123", ContentType: "text/plain"}

	t.Run("inserts a new entry", func(t *testing.T) {
		db := b.migrated(t)

		got, inserted, err := db.Upsert(t.Context(), entry)
		require.NoError(t, err)
		assert.True(t, inserted)
		assert.Equal(t, entry.Path, got.Path)
		assert.Equal(t, entry.Size, got.FileSizeBytes)
		assert.Equal(t, entry.ETag, got.Etag)
		assert.Equal(t, entry.ContentType, got.ContentType)
		assert.NotEqual(t, uuid.Nil(), got.ID)
	})

	t.Run("updates an existing entry in place", func(t *testing.T) {
		db := b.migrated(t)

		first, inserted, err := db.Upsert(t.Context(), entry)
		require.NoError(t, err)
		require.True(t, inserted)

		changed := types.ObjectEntry{Path: entry.Path, Size: 2048, ETag: "etag456", ContentType: "application/octet-stream"}
		second, inserted, err := db.Upsert(t.Context(), changed)
		require.NoError(t, err)

		assert.False(t, inserted)
		assert.Equal(t, first.ID, second.ID, "id must survive an update")
		assert.Equal(t, changed.Size, second.FileSizeBytes)
		assert.Equal(t, changed.ETag, second.Etag)
		assert.Equal(t, changed.ContentType, second.ContentType)
	})

	t.Run("restores a soft-deleted entry", func(t *testing.T) {
		db := b.migrated(t)

		_, _, err := db.Upsert(t.Context(), entry)
		require.NoError(t, err)
		require.NoError(t, db.Delete(t.Context(), entry.Path))

		got, inserted, err := db.Upsert(t.Context(), entry)
		require.NoError(t, err)
		assert.False(t, inserted, "restoring reuses the existing row")
		assert.Equal(t, entry.Path, got.Path)

		_, err = db.Get(t.Context(), entry.Path)
		assert.NoError(t, err)
	})
}

func testGet(t *testing.T, b backend) {
	entry := types.ObjectEntry{Path: "/test/file.txt", Size: 1024, ETag: "etag123", ContentType: "text/plain"}

	t.Run("returns a stored entry", func(t *testing.T) {
		db := b.migrated(t)

		upserted, _, err := db.Upsert(t.Context(), entry)
		require.NoError(t, err)

		got, err := db.Get(t.Context(), entry.Path)
		require.NoError(t, err)
		assert.Equal(t, upserted.ID, got.ID)
		assert.Equal(t, entry.Path, got.Path)
		assert.Equal(t, entry.Size, got.FileSizeBytes)
	})

	t.Run("reports an unknown path as not found", func(t *testing.T) {
		db := b.migrated(t)

		_, err := db.Get(t.Context(), "/nonexistent/file.txt")
		assert.ErrorIs(t, err, service.ErrNotFound)
	})

	t.Run("reports a deleted path as not found", func(t *testing.T) {
		db := b.migrated(t)

		_, _, err := db.Upsert(t.Context(), entry)
		require.NoError(t, err)
		require.NoError(t, db.Delete(t.Context(), entry.Path))

		_, err = db.Get(t.Context(), entry.Path)
		assert.ErrorIs(t, err, service.ErrNotFound)
	})
}

func testDelete(t *testing.T, b backend) {
	entry := types.ObjectEntry{Path: "/test/file.txt", Size: 1024, ETag: "etag123", ContentType: "text/plain"}

	t.Run("soft deletes a stored entry", func(t *testing.T) {
		db := b.migrated(t)

		_, _, err := db.Upsert(t.Context(), entry)
		require.NoError(t, err)

		require.NoError(t, db.Delete(t.Context(), entry.Path))

		_, err = db.Get(t.Context(), entry.Path)
		assert.ErrorIs(t, err, service.ErrNotFound)
	})

	t.Run("reports an unknown path as not found", func(t *testing.T) {
		db := b.migrated(t)

		err := db.Delete(t.Context(), "/nonexistent/file.txt")
		assert.ErrorIs(t, err, service.ErrNotFound)
	})

	t.Run("reports a second delete as not found", func(t *testing.T) {
		db := b.migrated(t)

		_, _, err := db.Upsert(t.Context(), entry)
		require.NoError(t, err)
		require.NoError(t, db.Delete(t.Context(), entry.Path))

		assert.ErrorIs(t, db.Delete(t.Context(), entry.Path), service.ErrNotFound)
	})
}

func testList(t *testing.T, b backend) {
	t.Run("returns everything under the prefix", func(t *testing.T) {
		db := b.migrated(t)
		seed(t, db, "/a/file1.txt", "/b/file2.txt", "/c/file3.txt")

		result, err := db.List(t.Context(), types.ListQuery{PathPrefix: "/", Limit: 10})
		require.NoError(t, err)
		assert.Len(t, result.Items, 3)
		assert.Empty(t, result.NextCursor)
	})

	t.Run("filters by prefix", func(t *testing.T) {
		db := b.migrated(t)
		seed(t, db, "/images/photo1.jpg", "/images/photo2.jpg", "/docs/readme.txt")

		result, err := db.List(t.Context(), types.ListQuery{PathPrefix: "/images/", Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, []string{"/images/photo1.jpg", "/images/photo2.jpg"}, paths(result))
	})

	t.Run("pages through results without repeating or dropping any", func(t *testing.T) {
		db := b.migrated(t)
		seed(t, db, "/file1.txt", "/file2.txt", "/file3.txt")

		first, err := db.List(t.Context(), types.ListQuery{PathPrefix: "/", Limit: 2})
		require.NoError(t, err)
		require.Len(t, first.Items, 2)
		require.NotEmpty(t, first.NextCursor)

		second, err := db.List(t.Context(), types.ListQuery{PathPrefix: "/", Limit: 2, Cursor: first.NextCursor})
		require.NoError(t, err)
		assert.Len(t, second.Items, 1)
		assert.Empty(t, second.NextCursor)

		assert.Equal(t,
			[]string{"/file1.txt", "/file2.txt", "/file3.txt"},
			append(paths(first), paths(second)...))
	})

	t.Run("rejects a malformed cursor", func(t *testing.T) {
		db := b.migrated(t)

		_, err := db.List(t.Context(), types.ListQuery{Limit: 10, Cursor: "not-a-cursor"})
		assert.ErrorContains(t, err, "decode cursor")
	})

	t.Run("excludes deleted entries", func(t *testing.T) {
		db := b.migrated(t)
		seed(t, db, "/file1.txt", "/file2.txt")
		require.NoError(t, db.Delete(t.Context(), "/file1.txt"))

		result, err := db.List(t.Context(), types.ListQuery{PathPrefix: "/", Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, []string{"/file2.txt"}, paths(result))
	})

	t.Run("returns an empty page when nothing matches", func(t *testing.T) {
		db := b.migrated(t)

		result, err := db.List(t.Context(), types.ListQuery{PathPrefix: "/", Limit: 10})
		require.NoError(t, err)
		assert.Empty(t, result.Items)
		assert.Empty(t, result.NextCursor)
	})

	t.Run("treats LIKE wildcards in the prefix literally", func(t *testing.T) {
		db := b.migrated(t)
		seed(t, db, "/foo%bar/file.txt", "/foo_bar/file.txt", "/fooXbar/file.txt")

		// Unescaped, % and _ are wildcards and either prefix would match all
		// three paths.
		result, err := db.List(t.Context(), types.ListQuery{PathPrefix: "/foo%bar/", Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, []string{"/foo%bar/file.txt"}, paths(result))

		result, err = db.List(t.Context(), types.ListQuery{PathPrefix: "/foo_bar/", Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, []string{"/foo_bar/file.txt"}, paths(result))
	})
}

func testListPendingCleanup(t *testing.T, b backend) {
	t.Run("returns deleted entries", func(t *testing.T) {
		db := b.migrated(t)
		seed(t, db, "/file1.txt", "/file2.txt", "/file3.txt")
		require.NoError(t, db.Delete(t.Context(), "/file1.txt"))
		require.NoError(t, db.Delete(t.Context(), "/file2.txt"))

		result, err := db.ListPendingCleanup(t.Context(), types.ListQuery{PathPrefix: "/", Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, []string{"/file1.txt", "/file2.txt"}, paths(result))
	})

	t.Run("excludes live entries", func(t *testing.T) {
		db := b.migrated(t)
		seed(t, db, "/file1.txt", "/file2.txt")
		require.NoError(t, db.Delete(t.Context(), "/file1.txt"))

		result, err := db.ListPendingCleanup(t.Context(), types.ListQuery{PathPrefix: "/", Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, []string{"/file1.txt"}, paths(result))
	})

	t.Run("excludes entries already cleaned up", func(t *testing.T) {
		db := b.migrated(t)
		ids := seed(t, db, "/file1.txt", "/file2.txt")
		require.NoError(t, db.Delete(t.Context(), "/file1.txt"))
		require.NoError(t, db.Delete(t.Context(), "/file2.txt"))
		require.NoError(t, db.MarkCleanedUp(t.Context(), ids["/file1.txt"]))

		result, err := db.ListPendingCleanup(t.Context(), types.ListQuery{PathPrefix: "/", Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, []string{"/file2.txt"}, paths(result))
	})
}

func testMarkCleanedUp(t *testing.T, b backend) {
	t.Run("takes a deleted entry off the cleanup list", func(t *testing.T) {
		db := b.migrated(t)
		ids := seed(t, db, "/file1.txt")
		require.NoError(t, db.Delete(t.Context(), "/file1.txt"))

		require.NoError(t, db.MarkCleanedUp(t.Context(), ids["/file1.txt"]))

		result, err := db.ListPendingCleanup(t.Context(), types.ListQuery{PathPrefix: "/", Limit: 10})
		require.NoError(t, err)
		assert.Empty(t, result.Items)
	})

	t.Run("reports an unknown id as not found", func(t *testing.T) {
		db := b.migrated(t)

		assert.ErrorIs(t, db.MarkCleanedUp(t.Context(), uuid.New()), service.ErrNotFound)
	})

	t.Run("reports a live entry as not found", func(t *testing.T) {
		db := b.migrated(t)
		ids := seed(t, db, "/file1.txt")

		assert.ErrorIs(t, db.MarkCleanedUp(t.Context(), ids["/file1.txt"]), service.ErrNotFound)
	})

	t.Run("reports a second cleanup as not found", func(t *testing.T) {
		db := b.migrated(t)
		ids := seed(t, db, "/file1.txt")
		require.NoError(t, db.Delete(t.Context(), "/file1.txt"))
		require.NoError(t, db.MarkCleanedUp(t.Context(), ids["/file1.txt"]))

		assert.ErrorIs(t, db.MarkCleanedUp(t.Context(), ids["/file1.txt"]), service.ErrNotFound)
	})
}

// seed upserts one entry per path, in order, and returns their ids by path.
// Inserting one at a time makes created_at order them as written, which the
// pagination cases rely on.
func seed(t *testing.T, db database.Database, paths ...string) map[string]uuid.UUID {
	t.Helper()

	ids := make(map[string]uuid.UUID, len(paths))
	for i, path := range paths {
		m, _, err := db.Upsert(t.Context(), types.ObjectEntry{
			Path:        path,
			Size:        int64(100 * (i + 1)),
			ETag:        "etag" + path,
			ContentType: "text/plain",
		})
		require.NoError(t, err)
		ids[path] = m.ID
	}

	return ids
}

// paths returns the paths of a page's items, in order.
func paths(result types.ListResult) []string {
	out := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		out = append(out, item.Path)
	}

	return out
}
