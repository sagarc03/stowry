package filesystem_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/filesystem"
	"github.com/stretchr/testify/assert"
)

func TestStore_Get_Success(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	content := []byte("test content")
	err = os.WriteFile(filepath.Join(tempDir, "test.txt"), content, 0o644)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	ctx := context.Background()
	result, err := store.Get(ctx, "test.txt")

	assert.NoError(t, err)
	assert.NotNil(t, result)

	readContent, err := io.ReadAll(result)
	assert.NoError(t, err)
	assert.Equal(t, content, readContent)

	err = result.Close()
	assert.NoError(t, err)
}

func TestStore_Get_ContextCanceled(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := store.Get(ctx, "test.txt")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, context.Canceled, err)
}

func TestStore_Get_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	ctx := context.Background()
	result, err := store.Get(ctx, "nonexistent.txt")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, stowry.ErrNotFound)
}

func TestStore_Write_Success(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	content := bytes.NewReader([]byte("test content"))
	ctx := context.Background()

	result, err := store.Write(ctx, "test.txt", content)

	assert.NoError(t, err)
	assert.Equal(t, int64(12), result.BytesWritten)
	assert.NotEmpty(t, result.Etag)
	assert.Equal(t, 64, len(result.Etag)) // SHA256 hex length

	writtenFile := filepath.Join(tempDir, "test.txt")
	data, err := os.ReadFile(writtenFile)
	assert.NoError(t, err)
	assert.Equal(t, []byte("test content"), data)
}

func TestStore_Write_WithSubdirectory(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	content := bytes.NewReader([]byte("nested content"))
	ctx := context.Background()

	result, err := store.Write(ctx, "subdir/nested/test.txt", content)

	assert.NoError(t, err)
	assert.Equal(t, int64(14), result.BytesWritten)
	assert.NotEmpty(t, result.Etag)

	writtenFile := filepath.Join(tempDir, "subdir", "nested", "test.txt")
	data, err := os.ReadFile(writtenFile)
	assert.NoError(t, err)
	assert.Equal(t, []byte("nested content"), data)
}

func TestStore_Write_ContextCanceledBefore(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	content := bytes.NewReader([]byte("test"))
	result, err := store.Write(ctx, "test.txt", content)

	assert.Error(t, err)
	assert.Equal(t, int64(0), result.BytesWritten)
	assert.Empty(t, result.Etag)
	assert.Equal(t, context.Canceled, err)
}

func TestStore_Write_ContextCanceledDuringCopy(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	ctx, cancel := context.WithCancel(context.Background())

	slowReader := &slowReader{
		data:   []byte("test content"),
		cancel: cancel,
	}

	result, err := store.Write(ctx, "test.txt", slowReader)

	assert.Error(t, err)
	assert.Equal(t, int64(0), result.BytesWritten)
	assert.Empty(t, result.Etag)
	assert.ErrorIs(t, err, context.Canceled)
}

type slowReader struct {
	data   []byte
	pos    int
	cancel context.CancelFunc
}

func (r *slowReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	r.cancel()
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func TestStore_Delete_Success(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("content"), 0o644)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	ctx := context.Background()
	err = store.Delete(ctx, "test.txt")

	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(tempDir, "test.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestStore_Delete_ContextCanceled(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = store.Delete(ctx, "test.txt")

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestStore_Delete_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	ctx := context.Background()
	err = store.Delete(ctx, "nonexistent.txt")

	assert.Error(t, err)
	assert.ErrorIs(t, err, stowry.ErrNotFound)
}

func TestStore_List_Success(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0o644)
	assert.NoError(t, err)

	err = os.MkdirAll(filepath.Join(tempDir, "subdir"), 0o755)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "subdir", "file2.json"), []byte("content2"), 0o644)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	ctx := context.Background()
	entries, err := store.List(ctx)

	assert.NoError(t, err)
	assert.Len(t, entries, 2)

	pathMap := make(map[string]stowry.ObjectEntry)
	for _, entry := range entries {
		pathMap[entry.Path] = entry
	}

	file1 := pathMap["file1.txt"]
	assert.Equal(t, int64(8), file1.Size)
	assert.NotEmpty(t, file1.ETag)
	assert.Equal(t, "text/plain; charset=utf-8", file1.ContentType)

	file2 := pathMap[filepath.Join("subdir", "file2.json")]
	assert.Equal(t, int64(8), file2.Size)
	assert.NotEmpty(t, file2.ETag)
	assert.Equal(t, "application/json", file2.ContentType)
}

func TestStore_List_EmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	ctx := context.Background()
	entries, err := store.List(ctx)

	assert.NoError(t, err)
	assert.Empty(t, entries)
}

func TestStore_List_ContextCanceled(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	entries, err := store.List(ctx)

	assert.Error(t, err)
	assert.Nil(t, entries)
	assert.Equal(t, context.Canceled, err)
}

func TestStore_List_NestedDirectories(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	err = os.MkdirAll(filepath.Join(tempDir, "a", "b", "c"), 0o755)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "a", "file1.txt"), []byte("content1"), 0o644)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "a", "b", "file2.txt"), []byte("content2"), 0o644)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "a", "b", "c", "file3.txt"), []byte("content3"), 0o644)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	ctx := context.Background()
	entries, err := store.List(ctx)

	assert.NoError(t, err)
	assert.Len(t, entries, 3)
}

func TestStore_List_UnknownFileExtension(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(tempDir, "file.unknown"), []byte("content"), 0o644)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	ctx := context.Background()
	entries, err := store.List(ctx)

	assert.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "application/octet-stream", entries[0].ContentType)
}

func TestStore_Write_ETagConsistency(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	content := []byte("test content for etag")
	ctx := context.Background()

	result1, err := store.Write(ctx, "file1.txt", bytes.NewReader(content))
	assert.NoError(t, err)

	result2, err := store.Write(ctx, "file2.txt", bytes.NewReader(content))
	assert.NoError(t, err)

	assert.Equal(t, result1.Etag, result2.Etag, "Same content should produce same ETag")

	entries, err := store.List(ctx)
	assert.NoError(t, err)

	for _, entry := range entries {
		assert.Equal(t, result1.Etag, entry.ETag, "Listed ETag should match written ETag")
	}
}

func TestStore_Write_LargeFile(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)

	largeContent := bytes.Repeat([]byte("a"), 1024*1024)
	ctx := context.Background()

	result, err := store.Write(ctx, "large.bin", bytes.NewReader(largeContent))

	assert.NoError(t, err)
	assert.Equal(t, int64(1024*1024), result.BytesWritten)
	assert.NotEmpty(t, result.Etag)

	writtenFile := filepath.Join(tempDir, "large.bin")
	info, err := os.Stat(writtenFile)
	assert.NoError(t, err)
	assert.Equal(t, int64(1024*1024), info.Size())
}

func TestStore_Integration_WriteReadDelete(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)
	ctx := context.Background()

	content := []byte("integration test content")

	result, err := store.Write(ctx, "test.txt", bytes.NewReader(content))
	assert.NoError(t, err)
	assert.Equal(t, int64(len(content)), result.BytesWritten)
	assert.NotEmpty(t, result.Etag)

	reader, err := store.Get(ctx, "test.txt")
	assert.NoError(t, err)
	readContent, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, content, readContent)
	err = reader.Close()
	assert.NoError(t, err)

	entries, err := store.List(ctx)
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "test.txt", entries[0].Path)
	assert.Equal(t, result.Etag, entries[0].ETag)

	err = store.Delete(ctx, "test.txt")
	assert.NoError(t, err)

	_, err = store.Get(ctx, "test.txt")
	assert.Error(t, err)

	entries, err = store.List(ctx)
	assert.NoError(t, err)
	assert.Empty(t, entries)
}

func TestStore_ConcurrentWrites(t *testing.T) {
	tempDir := t.TempDir()
	osDir, err := os.OpenRoot(tempDir)
	assert.NoError(t, err)

	store := filesystem.NewFileStorage(osDir)
	ctx := context.Background()

	done := make(chan bool, 10)
	for i := range 10 {
		go func(n int) {
			content := fmt.Appendf(nil, "content-%d", n)
			path := fmt.Sprintf("file-%d.txt", n)
			_, err := store.Write(ctx, path, bytes.NewReader(content))
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	for range 10 {
		<-done
	}

	entries, err := store.List(ctx)
	assert.NoError(t, err)
	assert.Len(t, entries, 10)
}
