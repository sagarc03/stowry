package service_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"syscall"
	"testing"
	"unicode/utf8"
	"uuid"

	"github.com/sagarc03/stowry/internal/service"
	"github.com/sagarc03/stowry/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type SpyMetaDataRepo struct {
	mock.Mock
}

func (s *SpyMetaDataRepo) Get(ctx context.Context, path string) (types.MetaData, error) {
	args := s.Called(ctx, path)
	return args.Get(0).(types.MetaData), args.Error(1)
}

func (s *SpyMetaDataRepo) Upsert(ctx context.Context, entry types.ObjectEntry) (types.MetaData, bool, error) {
	args := s.Called(ctx, entry)
	return args.Get(0).(types.MetaData), args.Bool(1), args.Error(2)
}

func (s *SpyMetaDataRepo) Delete(ctx context.Context, path string) error {
	args := s.Called(ctx, path)
	return args.Error(0)
}

func (s *SpyMetaDataRepo) List(ctx context.Context, q types.ListQuery) (types.ListResult, error) {
	args := s.Called(ctx, q)
	return args.Get(0).(types.ListResult), args.Error(1)
}

func (s *SpyMetaDataRepo) ListPendingCleanup(ctx context.Context, q types.ListQuery) (types.ListResult, error) {
	args := s.Called(ctx, q)
	return args.Get(0).(types.ListResult), args.Error(1)
}

func (s *SpyMetaDataRepo) MarkCleanedUp(ctx context.Context, id uuid.UUID) error {
	args := s.Called(ctx, id)
	return args.Error(0)
}

func NewStowryServiceWithFs(t *testing.T, storage afero.Fs) (*service.Service, *SpyMetaDataRepo) {
	t.Helper()
	spyRepo := new(SpyMetaDataRepo)
	cfg := service.ServiceConfig{Mode: types.ModeStore}
	s, err := service.New(spyRepo, storage, cfg)
	assert.NoError(t, err, "new stowry service")
	return s, spyRepo
}

func NewStowryService(t *testing.T) (*service.Service, *SpyMetaDataRepo, afero.Fs) {
	t.Helper()
	spyStorage := afero.NewMemMapFs()
	s, spyRepo := NewStowryServiceWithFs(t, spyStorage)
	return s, spyRepo, spyStorage
}

// removeFailFs is a filesystem whose Remove always fails, so the best-effort
// cleanup after a failed upsert can be exercised.
type removeFailFs struct {
	afero.Fs
	err error
}

func (f removeFailFs) Remove(string) error { return f.err }

// errReader fails on the first Read, so a failure part-way through the copy to
// storage can be exercised.
type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

func checkFile(t *testing.T, f afero.File, expectedData *bytes.Buffer, expectedBytes int64) {
	t.Helper()

	info, err := f.Stat()
	if err != nil {
		t.Fatalf("stat %s: %v", f.Name(), err)
	}
	if got := info.Size(); got != expectedBytes {
		t.Errorf("size of %s: got %d bytes, want %d", f.Name(), got, expectedBytes)
	}

	// Rewind in case the file was just written to and the offset is at the end.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek %s: %v", f.Name(), err)
	}
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read %s: %v", f.Name(), err)
	}
	if !bytes.Equal(got, expectedData.Bytes()) {
		t.Errorf("contents of %s: got %q, want %q", f.Name(), got, expectedData.Bytes())
	}
}

func checkNoFile(t *testing.T, storage afero.Fs, name string) {
	t.Helper()

	exists, err := afero.Exists(storage, name)
	if err != nil {
		t.Fatalf("stat %s: %v", name, err)
	}
	if exists {
		t.Errorf("file %s exists, want it absent", name)
	}
}

func TestStowryService_Create(t *testing.T) {
	t.Run("success - create new object", func(t *testing.T) {
		svc, repo, storage := NewStowryService(t)
		ctx := context.Background()
		fileName := "documents/test.txt"
		fileBytes := int64(12)
		fileData := "Hello World!"
		// sha256 of fileData.
		fileEtag := "7f83b1657ff1fc53b92dc18148a1d65dfc2d4b1fa3d677284addd200126d9069"

		obj := types.CreateObject{
			Path:        fileName,
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString(fileData)

		expectedMetadata := types.MetaData{
			Path:          fileName,
			ContentType:   "text/plain",
			FileSizeBytes: fileBytes,
			Etag:          fileEtag,
		}

		repo.On("Upsert", ctx, mock.MatchedBy(func(entry types.ObjectEntry) bool {
			return entry.Path == fileName &&
				entry.ContentType == "text/plain" &&
				entry.Size == fileBytes &&
				entry.ETag == fileEtag
		})).Return(expectedMetadata, true, nil)

		result, err := svc.Create(ctx, obj, content)
		assert.NoError(t, err)
		assert.Equal(t, fileName, result.Path)
		repo.AssertExpectations(t)
		svdFile, err := storage.Open(fileName)
		if err != nil {
			t.Fatalf("open saved file %s: %v", fileName, err)
		}
		defer svdFile.Close()
		checkFile(t, svdFile, bytes.NewBufferString(fileData), fileBytes)
	})

	t.Run("success - overwrite existing object", func(t *testing.T) {
		svc, repo, storage := NewStowryService(t)
		ctx := context.Background()
		fileName := "documents/test.txt"
		fileData := "data"
		fileBytes := int64(len(fileData))
		// sha256 of fileData.
		fileEtag := "3a6eb0790f39ac87c94f3856b2dd2c5d110e6811602261a9a923d3bb23adc8b7"

		if err := afero.WriteFile(storage, fileName, []byte("stale contents"), 0o644); err != nil {
			t.Fatalf("seed %s: %v", fileName, err)
		}

		obj := types.CreateObject{
			Path:        fileName,
			ContentType: "text/plain",
		}

		expectedMetadata := types.MetaData{
			Path:          fileName,
			ContentType:   "text/plain",
			FileSizeBytes: fileBytes,
			Etag:          fileEtag,
		}

		repo.On("Upsert", ctx, mock.MatchedBy(func(entry types.ObjectEntry) bool {
			return entry.Path == fileName &&
				entry.Size == fileBytes &&
				entry.ETag == fileEtag
		})).Return(expectedMetadata, false, nil)

		result, err := svc.Create(ctx, obj, bytes.NewBufferString(fileData))
		assert.NoError(t, err)
		assert.Equal(t, fileEtag, result.Etag)
		repo.AssertExpectations(t)

		svdFile, err := storage.Open(fileName)
		if err != nil {
			t.Fatalf("open saved file %s: %v", fileName, err)
		}
		defer svdFile.Close()
		checkFile(t, svdFile, bytes.NewBufferString(fileData), fileBytes)
	})

	t.Run("error - context cancelled before operation", func(t *testing.T) {
		svc, repo, storage := NewStowryService(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		obj := types.CreateObject{
			Path:        "test.txt",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		_, err := svc.Create(ctx, obj, content)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)

		checkNoFile(t, storage, "test.txt")
		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("error - empty path", func(t *testing.T) {
		svc, repo, _ := NewStowryService(t)
		ctx := context.Background()

		obj := types.CreateObject{
			Path:        "",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		_, err := svc.Create(ctx, obj, content)
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrInvalidInput)

		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("error - empty content type", func(t *testing.T) {
		svc, repo, storage := NewStowryService(t)
		ctx := context.Background()

		obj := types.CreateObject{
			Path:        "test.txt",
			ContentType: "",
		}
		content := bytes.NewBufferString("data")

		_, err := svc.Create(ctx, obj, content)
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrInvalidInput)

		checkNoFile(t, storage, "test.txt")
		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("error - invalid path with path traversal", func(t *testing.T) {
		svc, repo, storage := NewStowryService(t)
		ctx := context.Background()

		obj := types.CreateObject{
			Path:        "../etc/passwd",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		_, err := svc.Create(ctx, obj, content)
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrInvalidInput)

		checkNoFile(t, storage, "../etc/passwd")
		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("error - invalid path with absolute path", func(t *testing.T) {
		svc, repo, storage := NewStowryService(t)
		ctx := context.Background()

		obj := types.CreateObject{
			Path:        "/etc/passwd",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		_, err := svc.Create(ctx, obj, content)
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrInvalidInput)

		checkNoFile(t, storage, "/etc/passwd")
		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("error - storage create fails", func(t *testing.T) {
		storage := afero.NewReadOnlyFs(afero.NewMemMapFs())
		svc, repo := NewStowryServiceWithFs(t, storage)
		ctx := context.Background()

		obj := types.CreateObject{
			Path:        "test.txt",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		_, err := svc.Create(ctx, obj, content)
		assert.Error(t, err)
		assert.ErrorIs(t, err, syscall.EPERM)

		checkNoFile(t, storage, "test.txt")
		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("error - reading content fails", func(t *testing.T) {
		svc, repo, _ := NewStowryService(t)
		ctx := context.Background()

		obj := types.CreateObject{
			Path:        "test.txt",
			ContentType: "text/plain",
		}

		readErr := errors.New("connection reset")

		_, err := svc.Create(ctx, obj, errReader{err: readErr})
		assert.Error(t, err)
		assert.ErrorIs(t, err, readErr)

		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("error - metadata upsert fails with successful cleanup", func(t *testing.T) {
		svc, repo, storage := NewStowryService(t)
		ctx := context.Background()

		obj := types.CreateObject{
			Path:        "test.txt",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		upsertErr := errors.New("database error")
		repo.On("Upsert", ctx, mock.Anything).Return(types.MetaData{}, false, upsertErr)

		_, err := svc.Create(ctx, obj, content)
		assert.Error(t, err)
		assert.ErrorIs(t, err, upsertErr)

		repo.AssertExpectations(t)
		checkNoFile(t, storage, "test.txt")
	})

	t.Run("error - metadata upsert fails and cleanup fails", func(t *testing.T) {
		storage := removeFailFs{Fs: afero.NewMemMapFs(), err: errors.New("delete failed")}
		svc, repo := NewStowryServiceWithFs(t, storage)
		ctx := context.Background()

		obj := types.CreateObject{
			Path:        "test.txt",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		upsertErr := errors.New("database error")
		repo.On("Upsert", ctx, mock.Anything).Return(types.MetaData{}, false, upsertErr)

		_, err := svc.Create(ctx, obj, content)
		assert.Error(t, err)
		// Cleanup is best effort: the upsert error is what the caller sees, and
		// the orphaned file is left behind.
		assert.ErrorIs(t, err, upsertErr)

		repo.AssertExpectations(t)
		exists, existsErr := afero.Exists(storage, "test.txt")
		assert.NoError(t, existsErr)
		assert.True(t, exists, "file should be left behind when cleanup fails")
	})

	t.Run("error - context cancelled during upsert", func(t *testing.T) {
		svc, repo, storage := NewStowryService(t)
		ctx := context.Background()

		obj := types.CreateObject{
			Path:        "test.txt",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		repo.On("Upsert", ctx, mock.Anything).Return(types.MetaData{}, false, context.Canceled)

		_, err := svc.Create(ctx, obj, content)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)

		repo.AssertExpectations(t)
		checkNoFile(t, storage, "test.txt")
	})
}

func NewStowryServiceWithMode(t *testing.T, mode types.ServerMode) (*service.Service, *SpyMetaDataRepo, afero.Fs) {
	t.Helper()
	spyRepo := new(SpyMetaDataRepo)
	spyStorage := afero.NewMemMapFs()
	cfg := service.ServiceConfig{Mode: mode}
	s, err := service.New(spyRepo, spyStorage, cfg)
	assert.NoError(t, err, "new stowry service")
	return s, spyRepo, spyStorage
}

// writeFile seeds storage with a file the service is expected to find.
func writeFile(t *testing.T, storage afero.Fs, name, data string) {
	t.Helper()

	if err := afero.WriteFile(storage, name, []byte(data), 0o644); err != nil {
		t.Fatalf("seed %s: %v", name, err)
	}
}

// checkContents drains a reader the service handed back and compares it.
func checkContents(t *testing.T, r io.Reader, want string) {
	t.Helper()

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != want {
		t.Errorf("contents: got %q, want %q", got, want)
	}
}

func TestStowryService_Get(t *testing.T) {
	t.Run("success - get object in store mode", func(t *testing.T) {
		svc, repo, storage := NewStowryServiceWithMode(t, types.ModeStore)
		ctx := context.Background()

		expectedMetadata := types.MetaData{
			Path:          "documents/test.txt",
			ContentType:   "text/plain",
			FileSizeBytes: 12,
			Etag:          "abc123",
		}

		writeFile(t, storage, "documents/test.txt", "Hello World!")
		repo.On("Get", ctx, "documents/test.txt").Return(expectedMetadata, nil)

		metadata, file, err := svc.Get(ctx, "documents/test.txt")
		assert.NoError(t, err)
		assert.Equal(t, "documents/test.txt", metadata.Path)
		defer file.Close()
		checkContents(t, file, "Hello World!")

		repo.AssertExpectations(t)
	})

	t.Run("success - static mode fallback to index.html", func(t *testing.T) {
		svc, repo, storage := NewStowryServiceWithMode(t, types.ModeStatic)
		ctx := context.Background()

		indexMetadata := types.MetaData{
			Path:          "documents/index.html",
			ContentType:   "text/html",
			FileSizeBytes: 100,
			Etag:          "xyz789",
		}

		writeFile(t, storage, "documents/index.html", "<html></html>")
		repo.On("Get", ctx, "documents").Return(types.MetaData{}, service.ErrNotFound)
		repo.On("Get", ctx, "documents.html").Return(types.MetaData{}, service.ErrNotFound)
		repo.On("Get", ctx, "documents/index.html").Return(indexMetadata, nil)

		metadata, file, err := svc.Get(ctx, "documents")
		assert.NoError(t, err)
		assert.Equal(t, "documents/index.html", metadata.Path)
		defer file.Close()
		checkContents(t, file, "<html></html>")

		repo.AssertExpectations(t)
	})

	t.Run("success - spa mode fallback to index.html", func(t *testing.T) {
		svc, repo, storage := NewStowryServiceWithMode(t, types.ModeSPA)
		ctx := context.Background()

		indexMetadata := types.MetaData{
			Path:          "index.html",
			ContentType:   "text/html",
			FileSizeBytes: 100,
			Etag:          "xyz789",
		}

		writeFile(t, storage, "index.html", "<html></html>")
		repo.On("Get", ctx, "non-existent-route").Return(types.MetaData{}, service.ErrNotFound)
		repo.On("Get", ctx, "index.html").Return(indexMetadata, nil)

		metadata, file, err := svc.Get(ctx, "non-existent-route")
		assert.NoError(t, err)
		assert.Equal(t, "index.html", metadata.Path)
		defer file.Close()
		checkContents(t, file, "<html></html>")

		repo.AssertExpectations(t)
	})

	t.Run("error - context cancelled before operation", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeStore)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, _, err := svc.Get(ctx, "test.txt")
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)

		repo.AssertNotCalled(t, "Get")
	})

	t.Run("error - metadata not found in store mode", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeStore)
		ctx := context.Background()

		repo.On("Get", ctx, "nonexistent.txt").Return(types.MetaData{}, service.ErrNotFound)

		_, _, err := svc.Get(ctx, "nonexistent.txt")
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrNotFound)

		repo.AssertExpectations(t)
	})

	t.Run("error - static mode fallback also fails", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeStatic)
		ctx := context.Background()

		repo.On("Get", ctx, "documents").Return(types.MetaData{}, service.ErrNotFound)
		repo.On("Get", ctx, "documents.html").Return(types.MetaData{}, service.ErrNotFound)
		repo.On("Get", ctx, "documents/index.html").Return(types.MetaData{}, service.ErrNotFound)

		_, _, err := svc.Get(ctx, "documents")
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrNotFound)

		repo.AssertExpectations(t)
	})

	t.Run("error - spa mode fallback also fails", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeSPA)
		ctx := context.Background()

		repo.On("Get", ctx, "route").Return(types.MetaData{}, service.ErrNotFound)
		repo.On("Get", ctx, "index.html").Return(types.MetaData{}, service.ErrNotFound)

		_, _, err := svc.Get(ctx, "route")
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrNotFound)

		repo.AssertExpectations(t)
	})

	t.Run("error - repo returns non-NotFound error", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeStore)
		ctx := context.Background()

		dbErr := errors.New("database error")
		repo.On("Get", ctx, "test.txt").Return(types.MetaData{}, dbErr)

		_, _, err := svc.Get(ctx, "test.txt")
		assert.Error(t, err)
		assert.ErrorIs(t, err, dbErr)

		repo.AssertExpectations(t)
	})

	t.Run("error - metadata exists but file is missing from storage", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeStore)
		ctx := context.Background()

		metadata := types.MetaData{
			Path:          "test.txt",
			ContentType:   "text/plain",
			FileSizeBytes: 12,
			Etag:          "abc123",
		}

		repo.On("Get", ctx, "test.txt").Return(metadata, nil)

		_, _, err := svc.Get(ctx, "test.txt")
		assert.Error(t, err)
		assert.ErrorIs(t, err, fs.ErrNotExist)

		repo.AssertExpectations(t)
	})

	t.Run("static mode - first path exists, no fallback needed", func(t *testing.T) {
		svc, repo, storage := NewStowryServiceWithMode(t, types.ModeStatic)
		ctx := context.Background()

		metadata := types.MetaData{
			Path:          "documents/file.txt",
			ContentType:   "text/plain",
			FileSizeBytes: 12,
			Etag:          "abc123",
		}

		writeFile(t, storage, "documents/file.txt", "content")
		repo.On("Get", ctx, "documents/file.txt").Return(metadata, nil)

		_, file, err := svc.Get(ctx, "documents/file.txt")
		assert.NoError(t, err)
		defer file.Close()
		checkContents(t, file, "content")

		repo.AssertExpectations(t)
		repo.AssertNotCalled(t, "Get", mock.Anything, "documents/file.txt.html")
		repo.AssertNotCalled(t, "Get", mock.Anything, "documents/file.txt/index.html")
	})

	t.Run("spa mode - first path exists, no fallback needed", func(t *testing.T) {
		svc, repo, storage := NewStowryServiceWithMode(t, types.ModeSPA)
		ctx := context.Background()

		metadata := types.MetaData{
			Path:          "api/data.json",
			ContentType:   "application/json",
			FileSizeBytes: 50,
			Etag:          "abc123",
		}

		writeFile(t, storage, "api/data.json", "{}")
		repo.On("Get", ctx, "api/data.json").Return(metadata, nil)

		_, file, err := svc.Get(ctx, "api/data.json")
		assert.NoError(t, err)
		defer file.Close()
		checkContents(t, file, "{}")

		repo.AssertExpectations(t)
		repo.AssertNotCalled(t, "Get", mock.Anything, "index.html")
	})

	t.Run("error - empty path returns not found in store mode", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeStore)
		ctx := context.Background()

		_, _, err := svc.Get(ctx, "")
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrNotFound)

		repo.AssertNotCalled(t, "Get")
	})

	t.Run("success - empty path serves index.html in static mode", func(t *testing.T) {
		svc, repo, storage := NewStowryServiceWithMode(t, types.ModeStatic)
		ctx := context.Background()

		indexMetadata := types.MetaData{
			Path:          "index.html",
			ContentType:   "text/html",
			FileSizeBytes: 100,
			Etag:          "xyz789",
		}

		writeFile(t, storage, "index.html", "<html></html>")
		repo.On("Get", ctx, "index.html").Return(indexMetadata, nil)

		metadata, file, err := svc.Get(ctx, "")
		assert.NoError(t, err)
		assert.Equal(t, "index.html", metadata.Path)
		defer file.Close()
		checkContents(t, file, "<html></html>")

		repo.AssertExpectations(t)
	})

	t.Run("success - empty path serves index.html in spa mode", func(t *testing.T) {
		svc, repo, storage := NewStowryServiceWithMode(t, types.ModeSPA)
		ctx := context.Background()

		indexMetadata := types.MetaData{
			Path:          "index.html",
			ContentType:   "text/html",
			FileSizeBytes: 100,
			Etag:          "xyz789",
		}

		writeFile(t, storage, "index.html", "<html></html>")
		repo.On("Get", ctx, "index.html").Return(indexMetadata, nil)

		metadata, file, err := svc.Get(ctx, "")
		assert.NoError(t, err)
		assert.Equal(t, "index.html", metadata.Path)
		defer file.Close()
		checkContents(t, file, "<html></html>")

		repo.AssertExpectations(t)
	})

	t.Run("error - empty path returns not found when index.html doesn't exist in static mode", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeStatic)
		ctx := context.Background()

		// First call: index.html (from empty path conversion)
		repo.On("Get", ctx, "index.html").Return(types.MetaData{}, service.ErrNotFound).Once()
		// Clean URL fallback: index.html.html
		repo.On("Get", ctx, "index.html.html").Return(types.MetaData{}, service.ErrNotFound).Once()
		// Directory index fallback: index.html/index.html
		repo.On("Get", ctx, "index.html/index.html").Return(types.MetaData{}, service.ErrNotFound).Once()

		_, _, err := svc.Get(ctx, "")
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrNotFound)

		repo.AssertExpectations(t)
	})

	t.Run("error - empty path returns not found when index.html doesn't exist in spa mode", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeSPA)
		ctx := context.Background()

		// First call: index.html (from empty path conversion)
		// SPA fallback also tries index.html, so expect two calls
		repo.On("Get", ctx, "index.html").Return(types.MetaData{}, service.ErrNotFound).Twice()

		_, _, err := svc.Get(ctx, "")
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrNotFound)

		repo.AssertExpectations(t)
	})

	t.Run("success - static mode clean URL resolves foo.html", func(t *testing.T) {
		svc, repo, storage := NewStowryServiceWithMode(t, types.ModeStatic)
		ctx := context.Background()

		htmlMetadata := types.MetaData{
			Path:          "about.html",
			ContentType:   "text/html",
			FileSizeBytes: 200,
			Etag:          "html123",
		}

		writeFile(t, storage, "about.html", "<html>About</html>")
		repo.On("Get", ctx, "about").Return(types.MetaData{}, service.ErrNotFound)
		repo.On("Get", ctx, "about.html").Return(htmlMetadata, nil)

		metadata, file, err := svc.Get(ctx, "about")
		assert.NoError(t, err)
		assert.Equal(t, "about.html", metadata.Path)
		defer file.Close()
		checkContents(t, file, "<html>About</html>")

		repo.AssertExpectations(t)
	})

	t.Run("success - static mode trailing slash resolves to directory index", func(t *testing.T) {
		svc, repo, storage := NewStowryServiceWithMode(t, types.ModeStatic)
		ctx := context.Background()

		indexMetadata := types.MetaData{
			Path:          "docs/index.html",
			ContentType:   "text/html",
			FileSizeBytes: 100,
			Etag:          "idx123",
		}

		writeFile(t, storage, "docs/index.html", "<html>Docs</html>")
		repo.On("Get", ctx, "docs/").Return(types.MetaData{}, service.ErrNotFound)
		repo.On("Get", ctx, "docs/index.html").Return(indexMetadata, nil)

		metadata, file, err := svc.Get(ctx, "docs/")
		assert.NoError(t, err)
		assert.Equal(t, "docs/index.html", metadata.Path)
		defer file.Close()
		checkContents(t, file, "<html>Docs</html>")

		repo.AssertExpectations(t)
	})

	t.Run("error - static mode trailing slash with no index returns not found", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeStatic)
		ctx := context.Background()

		repo.On("Get", ctx, "docs/").Return(types.MetaData{}, service.ErrNotFound)
		repo.On("Get", ctx, "docs/index.html").Return(types.MetaData{}, service.ErrNotFound)

		_, _, err := svc.Get(ctx, "docs/")
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrNotFound)

		repo.AssertExpectations(t)
	})

	t.Run("success - static mode exact match takes priority over html fallback", func(t *testing.T) {
		svc, repo, storage := NewStowryServiceWithMode(t, types.ModeStatic)
		ctx := context.Background()

		exactMetadata := types.MetaData{
			Path:          "about",
			ContentType:   "application/octet-stream",
			FileSizeBytes: 50,
			Etag:          "exact123",
		}

		writeFile(t, storage, "about", "exact content")
		repo.On("Get", ctx, "about").Return(exactMetadata, nil)

		metadata, file, err := svc.Get(ctx, "about")
		assert.NoError(t, err)
		assert.Equal(t, "about", metadata.Path)
		defer file.Close()
		checkContents(t, file, "exact content")

		repo.AssertExpectations(t)
		repo.AssertNotCalled(t, "Get", mock.Anything, "about.html")
	})
}

func TestStowryService_Delete(t *testing.T) {
	t.Run("success - delete object", func(t *testing.T) {
		svc, repo, storage := NewStowryService(t)
		ctx := context.Background()

		writeFile(t, storage, "documents/test.txt", "Hello World!")
		repo.On("Delete", ctx, "documents/test.txt").Return(nil)

		err := svc.Delete(ctx, "documents/test.txt")
		assert.NoError(t, err)

		repo.AssertExpectations(t)
		checkNoFile(t, storage, "documents/test.txt")
	})

	t.Run("error - context cancelled before operation", func(t *testing.T) {
		svc, repo, storage := NewStowryService(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		writeFile(t, storage, "test.txt", "data")

		err := svc.Delete(ctx, "test.txt")
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)

		repo.AssertNotCalled(t, "Delete")
		exists, existsErr := afero.Exists(storage, "test.txt")
		assert.NoError(t, existsErr)
		assert.True(t, exists, "file should be left alone")
	})

	t.Run("error - empty path", func(t *testing.T) {
		svc, repo, _ := NewStowryService(t)
		ctx := context.Background()

		err := svc.Delete(ctx, "")
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrInvalidInput)

		repo.AssertNotCalled(t, "Delete")
	})

	t.Run("error - repository delete fails", func(t *testing.T) {
		svc, repo, storage := NewStowryService(t)
		ctx := context.Background()

		writeFile(t, storage, "test.txt", "data")

		dbErr := errors.New("database error")
		repo.On("Delete", ctx, "test.txt").Return(dbErr)

		err := svc.Delete(ctx, "test.txt")
		assert.Error(t, err)
		assert.ErrorIs(t, err, dbErr)

		repo.AssertExpectations(t)
		// Metadata delete failed, so the file is left in place.
		exists, existsErr := afero.Exists(storage, "test.txt")
		assert.NoError(t, existsErr)
		assert.True(t, exists, "file should be left alone")
	})

	t.Run("error - file missing from storage", func(t *testing.T) {
		svc, repo, _ := NewStowryService(t)
		ctx := context.Background()

		repo.On("Delete", ctx, "test.txt").Return(nil)

		err := svc.Delete(ctx, "test.txt")
		assert.Error(t, err)
		assert.ErrorIs(t, err, fs.ErrNotExist)

		repo.AssertExpectations(t)
	})
}

func TestStowryService_List(t *testing.T) {
	t.Run("success - list with results", func(t *testing.T) {
		svc, repo, _ := NewStowryService(t)
		ctx := context.Background()

		query := types.ListQuery{
			PathPrefix: "documents/",
			Limit:      10,
			Cursor:     "",
		}

		expectedResult := types.ListResult{
			Items: []types.MetaData{
				{Path: "documents/file1.txt", ContentType: "text/plain", FileSizeBytes: 100, Etag: "etag1"},
				{Path: "documents/file2.pdf", ContentType: "application/pdf", FileSizeBytes: 200, Etag: "etag2"},
			},
			NextCursor: "cursor123",
		}

		repo.On("List", ctx, query).Return(expectedResult, nil)

		result, err := svc.List(ctx, query)
		assert.NoError(t, err)
		assert.Len(t, result.Items, 2)
		assert.Equal(t, "cursor123", result.NextCursor)

		repo.AssertExpectations(t)
	})

	t.Run("success - empty list", func(t *testing.T) {
		svc, repo, _ := NewStowryService(t)
		ctx := context.Background()

		query := types.ListQuery{
			PathPrefix: "nonexistent/",
			Limit:      10,
		}

		expectedResult := types.ListResult{
			Items:      []types.MetaData{},
			NextCursor: "",
		}

		repo.On("List", ctx, query).Return(expectedResult, nil)

		result, err := svc.List(ctx, query)
		assert.NoError(t, err)
		assert.Empty(t, result.Items)
		assert.Empty(t, result.NextCursor)

		repo.AssertExpectations(t)
	})

	t.Run("success - with pagination cursor", func(t *testing.T) {
		svc, repo, _ := NewStowryService(t)
		ctx := context.Background()

		query := types.ListQuery{
			PathPrefix: "",
			Limit:      5,
			Cursor:     "previous_cursor",
		}

		expectedResult := types.ListResult{
			Items: []types.MetaData{
				{Path: "file3.txt", ContentType: "text/plain", FileSizeBytes: 150, Etag: "etag3"},
				{Path: "file4.txt", ContentType: "text/plain", FileSizeBytes: 250, Etag: "etag4"},
			},
			NextCursor: "next_cursor",
		}

		repo.On("List", ctx, query).Return(expectedResult, nil)

		result, err := svc.List(ctx, query)
		assert.NoError(t, err)
		assert.Len(t, result.Items, 2)
		assert.Equal(t, "next_cursor", result.NextCursor)

		repo.AssertExpectations(t)
	})

	t.Run("success - no path prefix lists all", func(t *testing.T) {
		svc, repo, _ := NewStowryService(t)
		ctx := context.Background()

		query := types.ListQuery{
			Limit: 100,
		}

		expectedResult := types.ListResult{
			Items: []types.MetaData{
				{Path: "root1.txt", ContentType: "text/plain", FileSizeBytes: 10, Etag: "etag1"},
				{Path: "docs/file.pdf", ContentType: "application/pdf", FileSizeBytes: 20, Etag: "etag2"},
				{Path: "images/photo.jpg", ContentType: "image/jpeg", FileSizeBytes: 30, Etag: "etag3"},
			},
			NextCursor: "",
		}

		repo.On("List", ctx, query).Return(expectedResult, nil)

		result, err := svc.List(ctx, query)
		assert.NoError(t, err)
		assert.Len(t, result.Items, 3)

		repo.AssertExpectations(t)
	})

	t.Run("error - context cancelled before operation", func(t *testing.T) {
		svc, repo, _ := NewStowryService(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		query := types.ListQuery{Limit: 10}

		_, err := svc.List(ctx, query)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)

		repo.AssertNotCalled(t, "List")
	})

	t.Run("error - repository list fails", func(t *testing.T) {
		svc, repo, _ := NewStowryService(t)
		ctx := context.Background()

		query := types.ListQuery{Limit: 10}

		dbErr := errors.New("database error")
		repo.On("List", ctx, query).Return(types.ListResult{}, dbErr)

		_, err := svc.List(ctx, query)
		assert.Error(t, err)
		assert.ErrorIs(t, err, dbErr)

		repo.AssertExpectations(t)
	})

	t.Run("error - context cancelled during list", func(t *testing.T) {
		svc, repo, _ := NewStowryService(t)
		ctx := context.Background()

		query := types.ListQuery{Limit: 10}

		repo.On("List", ctx, query).Return(types.ListResult{}, context.Canceled)

		_, err := svc.List(ctx, query)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)

		repo.AssertExpectations(t)
	})
}

func TestStowryService_Info(t *testing.T) {
	t.Run("success - get metadata in store mode", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeStore)
		ctx := context.Background()

		expectedMetadata := types.MetaData{
			Path:          "documents/test.txt",
			ContentType:   "text/plain",
			FileSizeBytes: 12,
			Etag:          "abc123",
		}

		repo.On("Get", ctx, "documents/test.txt").Return(expectedMetadata, nil)

		metadata, err := svc.Info(ctx, "documents/test.txt")
		assert.NoError(t, err)
		assert.Equal(t, "documents/test.txt", metadata.Path)
		assert.Equal(t, "text/plain", metadata.ContentType)
		assert.Equal(t, int64(12), metadata.FileSizeBytes)
		assert.Equal(t, "abc123", metadata.Etag)

		repo.AssertExpectations(t)
	})

	t.Run("success - static mode fallback to index.html", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeStatic)
		ctx := context.Background()

		indexMetadata := types.MetaData{
			Path:          "documents/index.html",
			ContentType:   "text/html",
			FileSizeBytes: 100,
			Etag:          "xyz789",
		}

		repo.On("Get", ctx, "documents").Return(types.MetaData{}, service.ErrNotFound)
		repo.On("Get", ctx, "documents.html").Return(types.MetaData{}, service.ErrNotFound)
		repo.On("Get", ctx, "documents/index.html").Return(indexMetadata, nil)

		metadata, err := svc.Info(ctx, "documents")
		assert.NoError(t, err)
		assert.Equal(t, "documents/index.html", metadata.Path)

		repo.AssertExpectations(t)
	})

	t.Run("success - spa mode fallback to index.html", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeSPA)
		ctx := context.Background()

		indexMetadata := types.MetaData{
			Path:          "index.html",
			ContentType:   "text/html",
			FileSizeBytes: 100,
			Etag:          "xyz789",
		}

		repo.On("Get", ctx, "non-existent-route").Return(types.MetaData{}, service.ErrNotFound)
		repo.On("Get", ctx, "index.html").Return(indexMetadata, nil)

		metadata, err := svc.Info(ctx, "non-existent-route")
		assert.NoError(t, err)
		assert.Equal(t, "index.html", metadata.Path)

		repo.AssertExpectations(t)
	})

	t.Run("error - context cancelled before operation", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeStore)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := svc.Info(ctx, "test.txt")
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)

		repo.AssertNotCalled(t, "Get")
	})

	t.Run("error - metadata not found in store mode", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeStore)
		ctx := context.Background()

		repo.On("Get", ctx, "nonexistent.txt").Return(types.MetaData{}, service.ErrNotFound)

		_, err := svc.Info(ctx, "nonexistent.txt")
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrNotFound)

		repo.AssertExpectations(t)
	})

	t.Run("error - empty path returns not found in store mode", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeStore)
		ctx := context.Background()

		_, err := svc.Info(ctx, "")
		assert.Error(t, err)
		assert.ErrorIs(t, err, service.ErrNotFound)

		repo.AssertNotCalled(t, "Get")
	})

	t.Run("success - empty path serves index.html in static mode", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeStatic)
		ctx := context.Background()

		indexMetadata := types.MetaData{
			Path:          "index.html",
			ContentType:   "text/html",
			FileSizeBytes: 100,
			Etag:          "xyz789",
		}

		repo.On("Get", ctx, "index.html").Return(indexMetadata, nil)

		metadata, err := svc.Info(ctx, "")
		assert.NoError(t, err)
		assert.Equal(t, "index.html", metadata.Path)

		repo.AssertExpectations(t)
	})

	t.Run("success - empty path serves index.html in spa mode", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeSPA)
		ctx := context.Background()

		indexMetadata := types.MetaData{
			Path:          "index.html",
			ContentType:   "text/html",
			FileSizeBytes: 100,
			Etag:          "xyz789",
		}

		repo.On("Get", ctx, "index.html").Return(indexMetadata, nil)

		metadata, err := svc.Info(ctx, "")
		assert.NoError(t, err)
		assert.Equal(t, "index.html", metadata.Path)

		repo.AssertExpectations(t)
	})

	t.Run("error - repo returns non-NotFound error", func(t *testing.T) {
		svc, repo, _ := NewStowryServiceWithMode(t, types.ModeStore)
		ctx := context.Background()

		dbErr := errors.New("database error")
		repo.On("Get", ctx, "test.txt").Return(types.MetaData{}, dbErr)

		_, err := svc.Info(ctx, "test.txt")
		assert.Error(t, err)
		assert.ErrorIs(t, err, dbErr)

		repo.AssertExpectations(t)
	})

	t.Run("no file storage access", func(t *testing.T) {
		svc, repo, storage := NewStowryServiceWithMode(t, types.ModeStore)
		ctx := context.Background()

		expectedMetadata := types.MetaData{
			Path:          "test.txt",
			ContentType:   "text/plain",
			FileSizeBytes: 50,
			Etag:          "abc123",
		}

		repo.On("Get", ctx, "test.txt").Return(expectedMetadata, nil)

		// Storage is empty: Info must succeed without ever touching the file.
		_, err := svc.Info(ctx, "test.txt")
		assert.NoError(t, err)

		repo.AssertExpectations(t)
		checkNoFile(t, storage, "test.txt")
	})
}

func TestIsValidPath(t *testing.T) {
	// Create a path with invalid UTF-8 (without embedding raw invalid bytes in source)
	invalidUTF8 := string([]byte{'/', 'a', 0xff, 'b'})

	tt := []struct {
		Name string
		Path string
		Want bool
	}{
		// Basics
		{Name: "root path", Path: "/", Want: false},
		{Name: "empty path", Path: "", Want: false},
		{Name: "no leading slash", Path: "/some/path", Want: false},
		{Name: "ends with slash", Path: "some/path/", Want: false},

		// Double dots anywhere are invalid
		{Name: "double dots segment", Path: "../", Want: false},
		{Name: "double dots in middle segment", Path: "a/../b", Want: false},
		{Name: "double dots at end", Path: "/a/..", Want: false},
		{Name: "double dots in filename", Path: "/a/b..c", Want: false},
		{Name: "double dots prefix", Path: "/a/..b", Want: false},

		// Signle dots segment are invalid
		{Name: "single dot segment not allowed", Path: "a/./b", Want: false},
		{Name: "single dot only", Path: ".", Want: false},

		// Double slashes invalid
		{Name: "double slash", Path: "a//b", Want: false},
		{Name: "leading double slash", Path: "//a", Want: false},

		// Forbidden characters
		{Name: "contains space", Path: "some path/file.ext", Want: false},
		{Name: "contains tab", Path: "some\tpath/file.ext", Want: false},
		{Name: "contains newline", Path: "some\npath/file.ext", Want: false},
		{Name: "contains carriage return", Path: "some\rpath/file.ext", Want: false},
		{Name: "contains backslash", Path: `some\path/file.ext`, Want: false},
		{Name: "contains hash", Path: "some/path#frag", Want: false},
		{Name: "contains question mark", Path: "some/path?x=1", Want: false},
		{Name: "contains tilde", Path: "some/~path/file.ext", Want: false},

		// Control chars / NUL
		{Name: "contains NUL", Path: "some\x00path/file.ext", Want: false},
		{Name: "contains DEL", Path: "some\x7fpath/file.ext", Want: false},
		{Name: "contains control char", Path: "some\x1fpath/file.ext", Want: false},

		// UTF-8 validity
		{Name: "invalid utf8", Path: invalidUTF8, Want: false},

		// Valid examples
		{Name: "simple valid", Path: "some/path/file.ext", Want: true},
		{Name: "hidden file valid", Path: ".hidden/file", Want: true},
		{Name: "underscores and dashes valid", Path: "some_path/with-dash/file_name.ext", Want: true},
		{Name: "percent is allowed as literal", Path: "a/%2e/b", Want: true}, // you didn't ban '%'
		{Name: "unicode valid", Path: "привет/世界/file.ext", Want: true},
	}

	// sanity check for our generated invalid UTF-8 case
	if utf8.ValidString(invalidUTF8) {
		t.Fatalf("test setup error: invalidUTF8 is unexpectedly valid")
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			got := service.IsValidPath(tc.Path)
			if got != tc.Want {
				expected := "valid"
				if !tc.Want {
					expected = "invalid"
				}
				t.Errorf("expected path %q to be %s, got %v", tc.Path, expected, got)
			}
		})
	}
}
