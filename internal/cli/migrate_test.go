package cli

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrate(t *testing.T) {
	t.Run("creates the schema", func(t *testing.T) {
		s := newStore(t)

		_, err := runCLI(t, "migrate", "-c", s.config)
		require.NoError(t, err)

		_, err = runCLI(t, "validate", "-c", s.config)
		assert.NoError(t, err)
	})

	t.Run("is repeatable", func(t *testing.T) {
		s := newStore(t)

		_, err := runCLI(t, "migrate", "-c", s.config)
		require.NoError(t, err)

		_, err = runCLI(t, "migrate", "-c", s.config)
		assert.NoError(t, err)
	})

	t.Run("populates when asked", func(t *testing.T) {
		s := newStore(t)
		s.fill(t, map[string]string{"index.html": "<h1>home</h1>", "a/b.txt": "b"})

		out, err := runCLI(t, "migrate", "-c", s.config, "-p")
		require.NoError(t, err)

		assert.Contains(t, out, "recorded 2 files")
		assert.ElementsMatch(t, []string{"index.html", "a/b.txt"}, s.paths(t))
	})

	t.Run("records nothing when not asked", func(t *testing.T) {
		s := newStore(t)
		s.fill(t, map[string]string{"index.html": "<h1>home</h1>"})

		out, err := runCLI(t, "migrate", "-c", s.config)
		require.NoError(t, err)

		assert.NotContains(t, out, "recorded")
		assert.Empty(t, s.paths(t))
	})

	// A table stowry did not build is reported, never altered. content_type is
	// the telling case: no index references it, so CREATE TABLE IF NOT EXISTS
	// leaves the table alone and reports no error of its own.
	t.Run("refuses a schema it does not recognise", func(t *testing.T) {
		s := newStore(t)

		raw, err := sql.Open("sqlite", s.dbPath)
		require.NoError(t, err)
		_, err = raw.Exec(`CREATE TABLE stowry_metadata (
			id TEXT NOT NULL PRIMARY KEY, path TEXT NOT NULL UNIQUE,
			etag TEXT NOT NULL, file_size_bytes INTEGER NOT NULL,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			deleted_at TEXT, cleaned_up_at TEXT)`)
		require.NoError(t, err)
		require.NoError(t, raw.Close())

		_, err = runCLI(t, "migrate", "-c", s.config)

		require.Error(t, err)
		assert.ErrorContains(t, err, "missing columns: content_type")
	})
}

func TestValidate(t *testing.T) {
	t.Run("passes on a migrated store", func(t *testing.T) {
		s := newStore(t)
		_, err := runCLI(t, "migrate", "-c", s.config)
		require.NoError(t, err)

		out, err := runCLI(t, "validate", "-c", s.config)

		require.NoError(t, err)
		assert.Contains(t, out, "schema is valid")
	})

	t.Run("fails on a store with no schema", func(t *testing.T) {
		s := newStore(t)

		_, err := runCLI(t, "validate", "-c", s.config)

		require.Error(t, err)
		assert.ErrorContains(t, err, "does not exist")
	})

	// validate reports; it never creates.
	t.Run("creates nothing", func(t *testing.T) {
		s := newStore(t)

		_, err := runCLI(t, "validate", "-c", s.config)
		require.Error(t, err)

		_, err = runCLI(t, "validate", "-c", s.config)
		assert.ErrorContains(t, err, "does not exist", "a second run sees the same empty store")
	})

	t.Run("rejects a table with no unique constraint on path", func(t *testing.T) {
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "meta.db")
		cfg := filepath.Join(dir, "config.yaml")
		body := "database: {type: sqlite, dsn: \"" + dbPath + "\"}\n" +
			"storage: {path: \"" + filepath.Join(dir, "data") + "\"}\nlog: {level: error}\n"
		require.NoError(t, os.WriteFile(cfg, []byte(body), 0o600))

		raw, err := sql.Open("sqlite", dbPath)
		require.NoError(t, err)
		_, err = raw.Exec(`CREATE TABLE stowry_metadata (
			id TEXT NOT NULL PRIMARY KEY, path TEXT NOT NULL,
			content_type TEXT NOT NULL, etag TEXT NOT NULL,
			file_size_bytes INTEGER NOT NULL, created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL, deleted_at TEXT, cleaned_up_at TEXT)`)
		require.NoError(t, err)
		require.NoError(t, raw.Close())

		_, err = runCLI(t, "validate", "-c", cfg)

		require.Error(t, err)
		assert.ErrorContains(t, err, "unique constraint on path")
	})
}
