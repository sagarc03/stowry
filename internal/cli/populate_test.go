package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sagarc03/stowry/internal/database"
	"github.com/sagarc03/stowry/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// store is a real sqlite file and a real storage directory, which is what
// populate reads and writes through.
type testStore struct {
	config  string
	dbPath  string
	dataDir string
}

func newStore(t *testing.T) testStore {
	t.Helper()

	dir := t.TempDir()
	s := testStore{
		config:  filepath.Join(dir, "config.yaml"),
		dbPath:  filepath.Join(dir, "meta.db"),
		dataDir: filepath.Join(dir, "data"),
	}

	body := "database: {type: sqlite, dsn: \"" + s.dbPath + "\"}\n" +
		"storage: {path: \"" + s.dataDir + "\"}\n" +
		"log: {level: error}\n"
	require.NoError(t, os.WriteFile(s.config, []byte(body), 0o600))

	return s
}

// paths returns every object path recorded in the metadata database.
func (s testStore) paths(t *testing.T) []string {
	t.Helper()

	db, err := database.Connect(t.Context(), database.Config{
		Type:   "sqlite",
		DSN:    s.dbPath,
		Tables: types.Tables{MetaData: "stowry_metadata"},
	})
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	result, err := db.List(t.Context(), types.ListQuery{Limit: 1000})
	require.NoError(t, err)

	out := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		out = append(out, item.Path)
	}

	return out
}

func (s testStore) meta(t *testing.T, path string) types.MetaData {
	t.Helper()

	db, err := database.Connect(t.Context(), database.Config{
		Type:   "sqlite",
		DSN:    s.dbPath,
		Tables: types.Tables{MetaData: "stowry_metadata"},
	})
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	m, err := db.Get(t.Context(), path)
	require.NoError(t, err)

	return m
}

// fill writes files into the storage directory, which is where populate
// expects to find them already.
func (s testStore) fill(t *testing.T, files map[string]string) {
	t.Helper()

	for name, content := range files {
		full := filepath.Join(s.dataDir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o600))
	}
}

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer

	cmd := newRootCmd("test")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)

	// Execute must run before the buffer is read: a multi-value return would
	// evaluate out.String() first.
	err := cmd.Execute()

	return out.String(), err
}

func TestPopulate(t *testing.T) {
	s := newStore(t)
	s.fill(t, map[string]string{
		"index.html":         "<h1>home</h1>",
		"about.html":         "about",
		"assets/app.css":     "body{}",
		"docs/deep/guide.md": "deep doc",
	})

	out, err := runCLI(t, "populate", "-c", s.config, "-m")
	require.NoError(t, err)
	assert.Contains(t, out, "recorded 4 files")

	t.Run("the files are left untouched", func(t *testing.T) {
		for name, want := range map[string]string{
			"index.html":         "<h1>home</h1>",
			"assets/app.css":     "body{}",
			"docs/deep/guide.md": "deep doc",
		} {
			got, err := os.ReadFile(filepath.Join(s.dataDir, filepath.FromSlash(name)))
			require.NoError(t, err, name)
			assert.Equal(t, want, string(got), name)
		}
	})

	t.Run("every file reaches the database", func(t *testing.T) {
		assert.ElementsMatch(t,
			[]string{"index.html", "about.html", "assets/app.css", "docs/deep/guide.md"},
			s.paths(t))
	})

	t.Run("content type is guessed from the extension", func(t *testing.T) {
		assert.Contains(t, s.meta(t, "assets/app.css").ContentType, "text/css")
		assert.Contains(t, s.meta(t, "index.html").ContentType, "text/html")
	})

	t.Run("size and etag are recorded", func(t *testing.T) {
		m := s.meta(t, "docs/deep/guide.md")
		assert.Equal(t, int64(len("deep doc")), m.FileSizeBytes)
		assert.Len(t, m.Etag, 64, "sha256 hex")
	})
}

// Re-recording the same directory updates the entry rather than adding another.
// The second run asks for no migration, which is the ordinary case.
func TestPopulateIsRepeatable(t *testing.T) {
	s := newStore(t)
	s.fill(t, map[string]string{"a.txt": "one"})

	_, err := runCLI(t, "populate", "-c", s.config, "-m")
	require.NoError(t, err)
	first := s.meta(t, "a.txt")

	s.fill(t, map[string]string{"a.txt": "two!"})

	_, err = runCLI(t, "populate", "-c", s.config)
	require.NoError(t, err)
	second := s.meta(t, "a.txt")

	assert.Equal(t, []string{"a.txt"}, s.paths(t))
	assert.Equal(t, first.ID, second.ID, "the entry is updated, not replaced")
	assert.Equal(t, int64(4), second.FileSizeBytes, "the new size is picked up")
	assert.NotEqual(t, first.Etag, second.Etag, "the new etag is picked up")
}

// Without --migrate the schema has to be there already, and saying so beats a
// driver error on the first write.
func TestPopulateRequiresSchema(t *testing.T) {
	s := newStore(t)
	s.fill(t, map[string]string{"a.txt": "one"})

	_, err := runCLI(t, "populate", "-c", s.config)

	require.Error(t, err)
	assert.ErrorContains(t, err, "does not exist")
	assert.ErrorContains(t, err, "stowry migrate")
}

// In-memory storage holds no files, so there is nothing to record. That is not
// an error - the default config asks for it - it just records nothing.
func TestPopulateInMemoryRecordsNothing(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	body := "database: {type: sqlite, dsn: \":memory:\"}\n" +
		"storage: {path: \":memory:\"}\nlog: {level: error}\n"
	require.NoError(t, os.WriteFile(cfg, []byte(body), 0o600))

	out, err := runCLI(t, "populate", "-c", cfg, "-m")

	require.NoError(t, err)
	assert.Contains(t, out, "recorded 0 files")
}

// A file that cannot be served stops the run, and the entries recorded before
// it are kept and reported.
func TestPopulateStopsAtUnservablePath(t *testing.T) {
	s := newStore(t)
	s.fill(t, map[string]string{
		"fine.txt":   "ok",
		"we?ird.txt": "rejected by the path rules",
	})

	out, err := runCLI(t, "populate", "-c", s.config, "-m")

	require.Error(t, err)
	assert.ErrorContains(t, err, "we?ird.txt")
	assert.Contains(t, out, "recorded 1 files")
	assert.Equal(t, []string{"fine.txt"}, s.paths(t), "the good file is still recorded")
}

func TestPopulateRejectsBadStoragePath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "afile")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	cfg := filepath.Join(dir, "config.yaml")
	body := "database: {type: sqlite, dsn: \"" + filepath.Join(dir, "m.db") + "\"}\n" +
		"storage: {path: \"" + file + "\"}\nlog: {level: error}\n"
	require.NoError(t, os.WriteFile(cfg, []byte(body), 0o600))

	_, err := runCLI(t, "populate", "-c", cfg, "-m")
	assert.ErrorContains(t, err, "create storage directory")
}

func TestPopulateEmptyDir(t *testing.T) {
	s := newStore(t)
	require.NoError(t, os.MkdirAll(s.dataDir, 0o700))

	out, err := runCLI(t, "populate", "-c", s.config, "-m")

	require.NoError(t, err)
	assert.Contains(t, out, "recorded 0 files")
}
