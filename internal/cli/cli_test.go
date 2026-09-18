package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sagarc03/stowry/internal/config"
)

// The config file list is the one setting config.Load cannot resolve itself, so
// its precedence is pinned here: flag, then environment, then the default.
func TestConfigFilePrecedence(t *testing.T) {
	write := func(t *testing.T, path, dsn string) string {
		t.Helper()
		body := "database: {type: sqlite, dsn: \"" + dsn + "\"}\nlog: {level: error}\n"
		require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

		return path
	}

	dsnFrom := func(t *testing.T, args ...string) string {
		t.Helper()

		cmd := newRootCmd("test")
		require.NoError(t, cmd.ParseFlags(args))

		files, err := configFiles(cmd)
		require.NoError(t, err)

		cfg, err := config.Load(files, cmd.Flags())
		require.NoError(t, err)

		return cfg.Database.DSN
	}

	t.Run("the flag wins", func(t *testing.T) {
		dir := t.TempDir()
		fromFlag := write(t, filepath.Join(dir, "flag.yaml"), "/flag.db")
		t.Setenv("STOWRY_CONFIG", write(t, filepath.Join(dir, "env.yaml"), "/env.db"))

		assert.Equal(t, "/flag.db", dsnFrom(t, "-c", fromFlag))
	})

	t.Run("the environment is used when the flag is unset", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("STOWRY_CONFIG", write(t, filepath.Join(dir, "env.yaml"), "/env.db"))

		assert.Equal(t, "/env.db", dsnFrom(t))
	})

	t.Run("several files merge left to right", func(t *testing.T) {
		dir := t.TempDir()
		first := write(t, filepath.Join(dir, "first.yaml"), "/first.db")
		second := write(t, filepath.Join(dir, "second.yaml"), "/second.db")
		t.Setenv("STOWRY_CONFIG", first+", "+second)

		assert.Equal(t, "/second.db", dsnFrom(t))
	})

	t.Run("an empty environment falls back to the default", func(t *testing.T) {
		t.Setenv("STOWRY_CONFIG", "")

		assert.Equal(t, ":memory:", dsnFrom(t))
	})
}
