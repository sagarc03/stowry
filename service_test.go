package stowry_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/sagarc03/stowry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type SpyMetaDataRepo struct {
	mock.Mock
}

func (s *SpyMetaDataRepo) Get(ctx context.Context, path string) (stowry.MetaData, error) {
	args := s.Called(ctx, path)
	return args.Get(0).(stowry.MetaData), args.Error(1)
}

func (s *SpyMetaDataRepo) Upsert(ctx context.Context, entry stowry.ObjectEntry) (stowry.MetaData, bool, error) {
	args := s.Called(ctx, entry)
	return args.Get(0).(stowry.MetaData), args.Bool(1), args.Error(2)
}

func (s *SpyMetaDataRepo) Delete(ctx context.Context, path string) error {
	args := s.Called(ctx, path)
	return args.Error(0)
}

func (s *SpyMetaDataRepo) List(ctx context.Context, q stowry.ListQuery) (stowry.ListResult, error) {
	args := s.Called(ctx, q)
	return args.Get(0).(stowry.ListResult), args.Error(1)
}

func (s *SpyMetaDataRepo) ListPendingCleanup(ctx context.Context, q stowry.ListQuery) (stowry.ListResult, error) {
	args := s.Called(ctx, q)
	return args.Get(0).(stowry.ListResult), args.Error(1)
}

func (s *SpyMetaDataRepo) MarkCleanedUp(ctx context.Context, id uuid.UUID) error {
	args := s.Called(ctx, id)
	return args.Error(0)
}

type SpyFileStorage struct {
	mock.Mock
}

func (s *SpyFileStorage) Get(ctx context.Context, path string) (io.ReadSeekCloser, error) {
	args := s.Called(ctx, path)
	return args.Get(0).(io.ReadSeekCloser), args.Error(1)
}

func (s *SpyFileStorage) Write(ctx context.Context, path string, content io.Reader) (stowry.SaveResult, error) {
	args := s.Called(ctx, path, content)
	return args.Get(0).(stowry.SaveResult), args.Error(1)
}

func (s *SpyFileStorage) Delete(ctx context.Context, path string) error {
	args := s.Called(ctx, path)
	return args.Error(0)
}

func (s *SpyFileStorage) List(ctx context.Context) ([]stowry.ObjectEntry, error) {
	args := s.Called(ctx)
	return args.Get(0).([]stowry.ObjectEntry), args.Error(1)
}

func NewStowryService(t *testing.T) (*stowry.StowryService, *SpyMetaDataRepo, *SpyFileStorage) {
	t.Helper()
	spyRepo := new(SpyMetaDataRepo)
	spyStorage := new(SpyFileStorage)
	s, err := stowry.NewStowryService(spyRepo, spyStorage, stowry.ModeStore)
	assert.NoError(t, err, "new stowry service")
	return s, spyRepo, spyStorage
}

func TestStowryService_Populate(t *testing.T) {
	t.Run("success with multiple files", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		files := []stowry.ObjectEntry{
			{Path: "file1.txt", ContentType: "text/plain", Size: 100, ETag: "etag1"},
			{Path: "file2.jpg", ContentType: "image/jpeg", Size: 200, ETag: "etag2"},
			{Path: "file3.pdf", ContentType: "application/pdf", Size: 300, ETag: "etag3"},
		}

		storage.On("List", ctx).Return(files, nil)
		repo.On("Upsert", ctx, files[0]).Return(stowry.MetaData{}, false, nil)
		repo.On("Upsert", ctx, files[1]).Return(stowry.MetaData{}, false, nil)
		repo.On("Upsert", ctx, files[2]).Return(stowry.MetaData{}, false, nil)

		err := service.Populate(ctx)
		assert.NoError(t, err)

		storage.AssertExpectations(t)
		repo.AssertExpectations(t)
	})

	t.Run("success with empty list", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		storage.On("List", ctx).Return([]stowry.ObjectEntry{}, nil)

		err := service.Populate(ctx)
		assert.NoError(t, err)

		storage.AssertExpectations(t)
		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("storage list error", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		storageErr := io.ErrUnexpectedEOF
		storage.On("List", ctx).Return([]stowry.ObjectEntry{}, storageErr)

		err := service.Populate(ctx)
		assert.Error(t, err)

		storage.AssertExpectations(t)
		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("upsert error on first file", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		files := []stowry.ObjectEntry{
			{Path: "file1.txt", ContentType: "text/plain", Size: 100, ETag: "etag1"},
		}

		upsertErr := io.ErrClosedPipe
		storage.On("List", ctx).Return(files, nil)
		repo.On("Upsert", ctx, files[0]).Return(stowry.MetaData{}, false, upsertErr)

		err := service.Populate(ctx)
		assert.Error(t, err)

		storage.AssertExpectations(t)
		repo.AssertExpectations(t)
	})

	t.Run("upsert error on second file", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		files := []stowry.ObjectEntry{
			{Path: "file1.txt", ContentType: "text/plain", Size: 100, ETag: "etag1"},
			{Path: "file2.jpg", ContentType: "image/jpeg", Size: 200, ETag: "etag2"},
		}

		upsertErr := io.ErrClosedPipe
		storage.On("List", ctx).Return(files, nil)
		repo.On("Upsert", ctx, files[0]).Return(stowry.MetaData{}, false, nil)
		repo.On("Upsert", ctx, files[1]).Return(stowry.MetaData{}, false, upsertErr)

		err := service.Populate(ctx)
		assert.Error(t, err)

		storage.AssertExpectations(t)
		repo.AssertExpectations(t)
	})

	t.Run("context cancelled before operation", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := service.Populate(ctx)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)

		storage.AssertNotCalled(t, "List")
		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("context cancelled during list", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		storage.On("List", ctx).Return([]stowry.ObjectEntry{}, context.Canceled)

		err := service.Populate(ctx)
		assert.Error(t, err)

		storage.AssertExpectations(t)
		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("context cancelled during upsert", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		files := []stowry.ObjectEntry{
			{Path: "file1.txt", ContentType: "text/plain", Size: 100, ETag: "etag1"},
			{Path: "file2.jpg", ContentType: "image/jpeg", Size: 200, ETag: "etag2"},
		}

		storage.On("List", ctx).Return(files, nil)
		repo.On("Upsert", ctx, files[0]).Return(stowry.MetaData{}, false, nil)
		repo.On("Upsert", ctx, files[1]).Return(stowry.MetaData{}, false, context.Canceled)

		err := service.Populate(ctx)
		assert.Error(t, err)

		storage.AssertExpectations(t)
		repo.AssertExpectations(t)
	})
}

func TestStowryService_Create(t *testing.T) {
	t.Run("success - create new object", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		obj := stowry.CreateObject{
			Path:        "documents/test.txt",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("Hello World!")

		saveResult := stowry.SaveResult{
			BytesWritten: 12,
			Etag:         "abc123",
		}

		expectedMetadata := stowry.MetaData{
			Path:          "documents/test.txt",
			ContentType:   "text/plain",
			FileSizeBytes: 12,
			Etag:          "abc123",
		}

		storage.On("Write", ctx, "documents/test.txt", content).Return(saveResult, nil)
		repo.On("Upsert", ctx, mock.MatchedBy(func(entry stowry.ObjectEntry) bool {
			return entry.Path == "documents/test.txt" &&
				entry.ContentType == "text/plain" &&
				entry.Size == 12 &&
				entry.ETag == "abc123"
		})).Return(expectedMetadata, true, nil)

		result, err := service.Create(ctx, obj, content)
		assert.NoError(t, err)
		assert.Equal(t, "documents/test.txt", result.Path)

		storage.AssertExpectations(t)
		repo.AssertExpectations(t)
	})

	t.Run("error - context cancelled before operation", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		obj := stowry.CreateObject{
			Path:        "test.txt",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		_, err := service.Create(ctx, obj, content)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)

		storage.AssertNotCalled(t, "Write")
		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("error - empty path", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		obj := stowry.CreateObject{
			Path:        "",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		_, err := service.Create(ctx, obj, content)
		assert.Error(t, err)
		assert.ErrorIs(t, err, stowry.ErrInvalidInput)

		storage.AssertNotCalled(t, "Write")
		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("error - empty content type", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		obj := stowry.CreateObject{
			Path:        "test.txt",
			ContentType: "",
		}
		content := bytes.NewBufferString("data")

		_, err := service.Create(ctx, obj, content)
		assert.Error(t, err)
		assert.ErrorIs(t, err, stowry.ErrInvalidInput)

		storage.AssertNotCalled(t, "Write")
		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("error - invalid path with path traversal", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		obj := stowry.CreateObject{
			Path:        "../etc/passwd",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		_, err := service.Create(ctx, obj, content)
		assert.Error(t, err)
		assert.ErrorIs(t, err, stowry.ErrInvalidInput)

		storage.AssertNotCalled(t, "Write")
		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("error - invalid path with absolute path", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		obj := stowry.CreateObject{
			Path:        "/etc/passwd",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		_, err := service.Create(ctx, obj, content)
		assert.Error(t, err)
		assert.ErrorIs(t, err, stowry.ErrInvalidInput)

		storage.AssertNotCalled(t, "Write")
		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("error - storage write fails", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		obj := stowry.CreateObject{
			Path:        "test.txt",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		writeErr := errors.New("disk full")
		storage.On("Write", ctx, "test.txt", content).Return(stowry.SaveResult{}, writeErr)

		_, err := service.Create(ctx, obj, content)
		assert.Error(t, err)

		storage.AssertExpectations(t)
		repo.AssertNotCalled(t, "Upsert")
	})

	t.Run("error - metadata upsert fails with successful cleanup", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		obj := stowry.CreateObject{
			Path:        "test.txt",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		saveResult := stowry.SaveResult{
			BytesWritten: 4,
			Etag:         "xyz789",
		}

		upsertErr := errors.New("database error")
		storage.On("Write", ctx, "test.txt", content).Return(saveResult, nil)
		repo.On("Upsert", ctx, mock.Anything).Return(stowry.MetaData{}, false, upsertErr)
		storage.On("Delete", mock.Anything, "test.txt").Return(nil)

		_, err := service.Create(ctx, obj, content)
		assert.Error(t, err)

		storage.AssertExpectations(t)
		repo.AssertExpectations(t)
		storage.AssertCalled(t, "Delete", mock.Anything, "test.txt")
	})

	t.Run("error - metadata upsert fails and cleanup fails", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		obj := stowry.CreateObject{
			Path:        "test.txt",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		saveResult := stowry.SaveResult{
			BytesWritten: 4,
			Etag:         "xyz789",
		}

		upsertErr := errors.New("database error")
		deleteErr := errors.New("delete failed")
		storage.On("Write", ctx, "test.txt", content).Return(saveResult, nil)
		repo.On("Upsert", ctx, mock.Anything).Return(stowry.MetaData{}, false, upsertErr)
		storage.On("Delete", mock.Anything, "test.txt").Return(deleteErr)

		_, err := service.Create(ctx, obj, content)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cleanup failed")

		storage.AssertExpectations(t)
		repo.AssertExpectations(t)
	})

	t.Run("error - context cancelled during storage write", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		obj := stowry.CreateObject{
			Path:        "test.txt",
			ContentType: "text/plain",
		}
		content := bytes.NewBufferString("data")

		storage.On("Write", ctx, "test.txt", content).Return(stowry.SaveResult{}, context.Canceled)

		_, err := service.Create(ctx, obj, content)
		assert.Error(t, err)

		storage.AssertExpectations(t)
		repo.AssertNotCalled(t, "Upsert")
	})
}

func NewStowryServiceWithMode(t *testing.T, mode stowry.ServerMode) (*stowry.StowryService, *SpyMetaDataRepo, *SpyFileStorage) {
	t.Helper()
	spyRepo := new(SpyMetaDataRepo)
	spyStorage := new(SpyFileStorage)
	s, err := stowry.NewStowryService(spyRepo, spyStorage, mode)
	assert.NoError(t, err, "new stowry service")
	return s, spyRepo, spyStorage
}

func TestStowryService_Get(t *testing.T) {
	t.Run("success - get object in store mode", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeStore)
		ctx := context.Background()

		expectedMetadata := stowry.MetaData{
			Path:          "documents/test.txt",
			ContentType:   "text/plain",
			FileSizeBytes: 12,
			Etag:          "abc123",
		}

		mockFile := &mockReadSeekCloser{content: []byte("Hello World!")}

		repo.On("Get", ctx, "documents/test.txt").Return(expectedMetadata, nil)
		storage.On("Get", ctx, "documents/test.txt").Return(mockFile, nil)

		metadata, file, err := service.Get(ctx, "documents/test.txt")
		assert.NoError(t, err)
		assert.Equal(t, "documents/test.txt", metadata.Path)
		assert.Same(t, mockFile, file)

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
	})

	t.Run("success - static mode fallback to index.html", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeStatic)
		ctx := context.Background()

		indexMetadata := stowry.MetaData{
			Path:          "documents/index.html",
			ContentType:   "text/html",
			FileSizeBytes: 100,
			Etag:          "xyz789",
		}

		mockFile := &mockReadSeekCloser{content: []byte("<html></html>")}

		repo.On("Get", ctx, "documents").Return(stowry.MetaData{}, stowry.ErrNotFound)
		repo.On("Get", ctx, "documents/index.html").Return(indexMetadata, nil)
		storage.On("Get", ctx, "documents/index.html").Return(mockFile, nil)

		metadata, file, err := service.Get(ctx, "documents")
		assert.NoError(t, err)
		assert.Equal(t, "documents/index.html", metadata.Path)
		assert.Same(t, mockFile, file)

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
	})

	t.Run("success - spa mode fallback to index.html", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeSPA)
		ctx := context.Background()

		indexMetadata := stowry.MetaData{
			Path:          "index.html",
			ContentType:   "text/html",
			FileSizeBytes: 100,
			Etag:          "xyz789",
		}

		mockFile := &mockReadSeekCloser{content: []byte("<html></html>")}

		repo.On("Get", ctx, "non-existent-route").Return(stowry.MetaData{}, stowry.ErrNotFound)
		repo.On("Get", ctx, "index.html").Return(indexMetadata, nil)
		storage.On("Get", ctx, "index.html").Return(mockFile, nil)

		metadata, file, err := service.Get(ctx, "non-existent-route")
		assert.NoError(t, err)
		assert.Equal(t, "index.html", metadata.Path)
		assert.Same(t, mockFile, file)

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
	})

	t.Run("error - context cancelled before operation", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeStore)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, _, err := service.Get(ctx, "test.txt")
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)

		repo.AssertNotCalled(t, "Get")
		storage.AssertNotCalled(t, "Get")
	})

	t.Run("error - metadata not found in store mode", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeStore)
		ctx := context.Background()

		repo.On("Get", ctx, "nonexistent.txt").Return(stowry.MetaData{}, stowry.ErrNotFound)

		_, _, err := service.Get(ctx, "nonexistent.txt")
		assert.Error(t, err)
		assert.ErrorIs(t, err, stowry.ErrNotFound)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "Get")
	})

	t.Run("error - static mode fallback also fails", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeStatic)
		ctx := context.Background()

		repo.On("Get", ctx, "documents").Return(stowry.MetaData{}, stowry.ErrNotFound)
		repo.On("Get", ctx, "documents/index.html").Return(stowry.MetaData{}, stowry.ErrNotFound)

		_, _, err := service.Get(ctx, "documents")
		assert.Error(t, err)
		assert.ErrorIs(t, err, stowry.ErrNotFound)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "Get")
	})

	t.Run("error - spa mode fallback also fails", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeSPA)
		ctx := context.Background()

		repo.On("Get", ctx, "route").Return(stowry.MetaData{}, stowry.ErrNotFound)
		repo.On("Get", ctx, "index.html").Return(stowry.MetaData{}, stowry.ErrNotFound)

		_, _, err := service.Get(ctx, "route")
		assert.Error(t, err)
		assert.ErrorIs(t, err, stowry.ErrNotFound)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "Get")
	})

	t.Run("error - repo returns non-NotFound error", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeStore)
		ctx := context.Background()

		dbErr := errors.New("database error")
		repo.On("Get", ctx, "test.txt").Return(stowry.MetaData{}, dbErr)

		_, _, err := service.Get(ctx, "test.txt")
		assert.Error(t, err)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "Get")
	})

	t.Run("error - storage get fails", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeStore)
		ctx := context.Background()

		metadata := stowry.MetaData{
			Path:          "test.txt",
			ContentType:   "text/plain",
			FileSizeBytes: 12,
			Etag:          "abc123",
		}

		storageErr := errors.New("storage error")
		repo.On("Get", ctx, "test.txt").Return(metadata, nil)
		storage.On("Get", ctx, "test.txt").Return(&mockReadSeekCloser{}, storageErr)

		_, _, err := service.Get(ctx, "test.txt")
		assert.Error(t, err)

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
	})

	t.Run("static mode - first path exists, no fallback needed", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeStatic)
		ctx := context.Background()

		metadata := stowry.MetaData{
			Path:          "documents/file.txt",
			ContentType:   "text/plain",
			FileSizeBytes: 12,
			Etag:          "abc123",
		}

		mockFile := &mockReadSeekCloser{content: []byte("content")}

		repo.On("Get", ctx, "documents/file.txt").Return(metadata, nil)
		storage.On("Get", ctx, "documents/file.txt").Return(mockFile, nil)

		_, file, err := service.Get(ctx, "documents/file.txt")
		assert.NoError(t, err)
		assert.Same(t, mockFile, file)

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
		repo.AssertNotCalled(t, "Get", mock.Anything, "documents/file.txt/index.html")
	})

	t.Run("spa mode - first path exists, no fallback needed", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeSPA)
		ctx := context.Background()

		metadata := stowry.MetaData{
			Path:          "api/data.json",
			ContentType:   "application/json",
			FileSizeBytes: 50,
			Etag:          "abc123",
		}

		mockFile := &mockReadSeekCloser{content: []byte("{}")}

		repo.On("Get", ctx, "api/data.json").Return(metadata, nil)
		storage.On("Get", ctx, "api/data.json").Return(mockFile, nil)

		_, file, err := service.Get(ctx, "api/data.json")
		assert.NoError(t, err)
		assert.Same(t, mockFile, file)

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
		repo.AssertNotCalled(t, "Get", mock.Anything, "index.html")
	})

	t.Run("error - empty path returns not found in store mode", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeStore)
		ctx := context.Background()

		_, _, err := service.Get(ctx, "")
		assert.Error(t, err)
		assert.ErrorIs(t, err, stowry.ErrNotFound)

		repo.AssertNotCalled(t, "Get")
		storage.AssertNotCalled(t, "Get")
	})

	t.Run("success - empty path serves index.html in static mode", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeStatic)
		ctx := context.Background()

		indexMetadata := stowry.MetaData{
			Path:          "index.html",
			ContentType:   "text/html",
			FileSizeBytes: 100,
			Etag:          "xyz789",
		}

		mockFile := &mockReadSeekCloser{content: []byte("<html></html>")}

		repo.On("Get", ctx, "index.html").Return(indexMetadata, nil)
		storage.On("Get", ctx, "index.html").Return(mockFile, nil)

		metadata, file, err := service.Get(ctx, "")
		assert.NoError(t, err)
		assert.Equal(t, "index.html", metadata.Path)
		assert.Same(t, mockFile, file)

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
	})

	t.Run("success - empty path serves index.html in spa mode", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeSPA)
		ctx := context.Background()

		indexMetadata := stowry.MetaData{
			Path:          "index.html",
			ContentType:   "text/html",
			FileSizeBytes: 100,
			Etag:          "xyz789",
		}

		mockFile := &mockReadSeekCloser{content: []byte("<html></html>")}

		repo.On("Get", ctx, "index.html").Return(indexMetadata, nil)
		storage.On("Get", ctx, "index.html").Return(mockFile, nil)

		metadata, file, err := service.Get(ctx, "")
		assert.NoError(t, err)
		assert.Equal(t, "index.html", metadata.Path)
		assert.Same(t, mockFile, file)

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
	})

	t.Run("error - empty path returns not found when index.html doesn't exist in static mode", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeStatic)
		ctx := context.Background()

		// First call: index.html (from empty path conversion)
		repo.On("Get", ctx, "index.html").Return(stowry.MetaData{}, stowry.ErrNotFound).Once()
		// Fallback call: index.html/index.html (static mode fallback)
		repo.On("Get", ctx, "index.html/index.html").Return(stowry.MetaData{}, stowry.ErrNotFound).Once()

		_, _, err := service.Get(ctx, "")
		assert.Error(t, err)
		assert.ErrorIs(t, err, stowry.ErrNotFound)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "Get")
	})

	t.Run("error - empty path returns not found when index.html doesn't exist in spa mode", func(t *testing.T) {
		service, repo, storage := NewStowryServiceWithMode(t, stowry.ModeSPA)
		ctx := context.Background()

		// First call: index.html (from empty path conversion)
		// SPA fallback also tries index.html, so expect two calls
		repo.On("Get", ctx, "index.html").Return(stowry.MetaData{}, stowry.ErrNotFound).Twice()

		_, _, err := service.Get(ctx, "")
		assert.Error(t, err)
		assert.ErrorIs(t, err, stowry.ErrNotFound)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "Get")
	})
}

type mockReadSeekCloser struct {
	content []byte
	pos     int64
}

func (m *mockReadSeekCloser) Read(p []byte) (n int, err error) {
	if m.pos >= int64(len(m.content)) {
		return 0, io.EOF
	}
	n = copy(p, m.content[m.pos:])
	m.pos += int64(n)
	return n, nil
}

func (m *mockReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = m.pos + offset
	case io.SeekEnd:
		abs = int64(len(m.content)) + offset
	default:
		return 0, errors.New("invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("negative position")
	}
	m.pos = abs
	return abs, nil
}

func (m *mockReadSeekCloser) Close() error {
	return nil
}

func TestStowryService_Delete(t *testing.T) {
	t.Run("success - delete object", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		repo.On("Delete", ctx, "documents/test.txt").Return(nil)

		err := service.Delete(ctx, "documents/test.txt")
		assert.NoError(t, err)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "Delete")
	})

	t.Run("error - context cancelled before operation", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := service.Delete(ctx, "test.txt")
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)

		repo.AssertNotCalled(t, "Delete")
		storage.AssertNotCalled(t, "Delete")
	})

	t.Run("error - empty path", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		err := service.Delete(ctx, "")
		assert.Error(t, err)
		assert.ErrorIs(t, err, stowry.ErrInvalidInput)

		repo.AssertNotCalled(t, "Delete")
		storage.AssertNotCalled(t, "Delete")
	})

	t.Run("error - repository delete fails", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		dbErr := errors.New("database error")
		repo.On("Delete", ctx, "test.txt").Return(dbErr)

		err := service.Delete(ctx, "test.txt")
		assert.Error(t, err)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "Delete")
	})
}

func TestStowryService_List(t *testing.T) {
	t.Run("success - list with results", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{
			PathPrefix: "documents/",
			Limit:      10,
			Cursor:     "",
		}

		expectedResult := stowry.ListResult{
			Items: []stowry.MetaData{
				{Path: "documents/file1.txt", ContentType: "text/plain", FileSizeBytes: 100, Etag: "etag1"},
				{Path: "documents/file2.pdf", ContentType: "application/pdf", FileSizeBytes: 200, Etag: "etag2"},
			},
			NextCursor: "cursor123",
		}

		repo.On("List", ctx, query).Return(expectedResult, nil)

		result, err := service.List(ctx, query)
		assert.NoError(t, err)
		assert.Len(t, result.Items, 2)
		assert.Equal(t, "cursor123", result.NextCursor)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "List")
	})

	t.Run("success - empty list", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{
			PathPrefix: "nonexistent/",
			Limit:      10,
		}

		expectedResult := stowry.ListResult{
			Items:      []stowry.MetaData{},
			NextCursor: "",
		}

		repo.On("List", ctx, query).Return(expectedResult, nil)

		result, err := service.List(ctx, query)
		assert.NoError(t, err)
		assert.Empty(t, result.Items)
		assert.Empty(t, result.NextCursor)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "List")
	})

	t.Run("success - with pagination cursor", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{
			PathPrefix: "",
			Limit:      5,
			Cursor:     "previous_cursor",
		}

		expectedResult := stowry.ListResult{
			Items: []stowry.MetaData{
				{Path: "file3.txt", ContentType: "text/plain", FileSizeBytes: 150, Etag: "etag3"},
				{Path: "file4.txt", ContentType: "text/plain", FileSizeBytes: 250, Etag: "etag4"},
			},
			NextCursor: "next_cursor",
		}

		repo.On("List", ctx, query).Return(expectedResult, nil)

		result, err := service.List(ctx, query)
		assert.NoError(t, err)
		assert.Len(t, result.Items, 2)
		assert.Equal(t, "next_cursor", result.NextCursor)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "List")
	})

	t.Run("success - no path prefix lists all", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{
			Limit: 100,
		}

		expectedResult := stowry.ListResult{
			Items: []stowry.MetaData{
				{Path: "root1.txt", ContentType: "text/plain", FileSizeBytes: 10, Etag: "etag1"},
				{Path: "docs/file.pdf", ContentType: "application/pdf", FileSizeBytes: 20, Etag: "etag2"},
				{Path: "images/photo.jpg", ContentType: "image/jpeg", FileSizeBytes: 30, Etag: "etag3"},
			},
			NextCursor: "",
		}

		repo.On("List", ctx, query).Return(expectedResult, nil)

		result, err := service.List(ctx, query)
		assert.NoError(t, err)
		assert.Len(t, result.Items, 3)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "List")
	})

	t.Run("error - context cancelled before operation", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		query := stowry.ListQuery{Limit: 10}

		_, err := service.List(ctx, query)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)

		repo.AssertNotCalled(t, "List")
		storage.AssertNotCalled(t, "List")
	})

	t.Run("error - repository list fails", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{Limit: 10}

		dbErr := errors.New("database error")
		repo.On("List", ctx, query).Return(stowry.ListResult{}, dbErr)

		_, err := service.List(ctx, query)
		assert.Error(t, err)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "List")
	})

	t.Run("error - context cancelled during list", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{Limit: 10}

		repo.On("List", ctx, query).Return(stowry.ListResult{}, context.Canceled)

		_, err := service.List(ctx, query)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "List")
	})
}

func TestStowryService_Tombstone(t *testing.T) {
	t.Run("success - tombstone multiple files", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{
			PathPrefix: "deleted/",
			Limit:      10,
		}

		id1 := uuid.New()
		id2 := uuid.New()
		id3 := uuid.New()

		pendingCleanup := stowry.ListResult{
			Items: []stowry.MetaData{
				{ID: id1, Path: "deleted/file1.txt", ContentType: "text/plain", FileSizeBytes: 100, Etag: "etag1"},
				{ID: id2, Path: "deleted/file2.pdf", ContentType: "application/pdf", FileSizeBytes: 200, Etag: "etag2"},
				{ID: id3, Path: "deleted/file3.jpg", ContentType: "image/jpeg", FileSizeBytes: 300, Etag: "etag3"},
			},
			NextCursor: "",
		}

		repo.On("ListPendingCleanup", ctx, query).Return(pendingCleanup, nil)
		storage.On("Delete", ctx, "deleted/file1.txt").Return(nil)
		repo.On("MarkCleanedUp", ctx, id1).Return(nil)
		storage.On("Delete", ctx, "deleted/file2.pdf").Return(nil)
		repo.On("MarkCleanedUp", ctx, id2).Return(nil)
		storage.On("Delete", ctx, "deleted/file3.jpg").Return(nil)
		repo.On("MarkCleanedUp", ctx, id3).Return(nil)

		count, err := service.Tombstone(ctx, query)
		assert.NoError(t, err)
		assert.Equal(t, 3, count)

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
	})

	t.Run("success - empty list no files to tombstone", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{
			Limit: 10,
		}

		pendingCleanup := stowry.ListResult{
			Items:      []stowry.MetaData{},
			NextCursor: "",
		}

		repo.On("ListPendingCleanup", ctx, query).Return(pendingCleanup, nil)

		count, err := service.Tombstone(ctx, query)
		assert.NoError(t, err)
		assert.Equal(t, 0, count)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "Delete")
	})

	t.Run("success - tombstone single file", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{
			Limit: 10,
		}

		id1 := uuid.New()

		pendingCleanup := stowry.ListResult{
			Items: []stowry.MetaData{
				{ID: id1, Path: "deleted/single.txt", ContentType: "text/plain", FileSizeBytes: 50, Etag: "etag1"},
			},
			NextCursor: "",
		}

		repo.On("ListPendingCleanup", ctx, query).Return(pendingCleanup, nil)
		storage.On("Delete", ctx, "deleted/single.txt").Return(nil)
		repo.On("MarkCleanedUp", ctx, id1).Return(nil)

		count, err := service.Tombstone(ctx, query)
		assert.NoError(t, err)
		assert.Equal(t, 1, count)

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
	})

	t.Run("success - processes multiple pages", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{
			Limit: 2,
		}

		id1 := uuid.New()
		id2 := uuid.New()
		id3 := uuid.New()

		// First page
		page1 := stowry.ListResult{
			Items: []stowry.MetaData{
				{ID: id1, Path: "deleted/file1.txt", ContentType: "text/plain", FileSizeBytes: 100, Etag: "etag1"},
				{ID: id2, Path: "deleted/file2.txt", ContentType: "text/plain", FileSizeBytes: 200, Etag: "etag2"},
			},
			NextCursor: "cursor_page2",
		}

		// Second page
		page2 := stowry.ListResult{
			Items: []stowry.MetaData{
				{ID: id3, Path: "deleted/file3.txt", ContentType: "text/plain", FileSizeBytes: 300, Etag: "etag3"},
			},
			NextCursor: "",
		}

		repo.On("ListPendingCleanup", ctx, query).Return(page1, nil).Once()
		storage.On("Delete", ctx, "deleted/file1.txt").Return(nil)
		repo.On("MarkCleanedUp", ctx, id1).Return(nil)
		storage.On("Delete", ctx, "deleted/file2.txt").Return(nil)
		repo.On("MarkCleanedUp", ctx, id2).Return(nil)

		query2 := stowry.ListQuery{Limit: 2, Cursor: "cursor_page2"}
		repo.On("ListPendingCleanup", ctx, query2).Return(page2, nil).Once()
		storage.On("Delete", ctx, "deleted/file3.txt").Return(nil)
		repo.On("MarkCleanedUp", ctx, id3).Return(nil)

		count, err := service.Tombstone(ctx, query)
		assert.NoError(t, err)
		assert.Equal(t, 3, count)

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
	})

	t.Run("success - file already deleted from storage", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{Limit: 10}

		id1 := uuid.New()
		id2 := uuid.New()

		pendingCleanup := stowry.ListResult{
			Items: []stowry.MetaData{
				{ID: id1, Path: "deleted/file1.txt", ContentType: "text/plain", FileSizeBytes: 100, Etag: "etag1"},
				{ID: id2, Path: "deleted/file2.txt", ContentType: "text/plain", FileSizeBytes: 200, Etag: "etag2"},
			},
			NextCursor: "",
		}

		repo.On("ListPendingCleanup", ctx, query).Return(pendingCleanup, nil)
		// First file already deleted - should continue
		storage.On("Delete", ctx, "deleted/file1.txt").Return(stowry.ErrNotFound)
		repo.On("MarkCleanedUp", ctx, id1).Return(nil)
		storage.On("Delete", ctx, "deleted/file2.txt").Return(nil)
		repo.On("MarkCleanedUp", ctx, id2).Return(nil)

		count, err := service.Tombstone(ctx, query)
		assert.NoError(t, err)
		assert.Equal(t, 2, count)

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
	})

	t.Run("error - context cancelled before operation", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		query := stowry.ListQuery{Limit: 10}

		count, err := service.Tombstone(ctx, query)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, count)

		repo.AssertNotCalled(t, "ListPendingCleanup")
		storage.AssertNotCalled(t, "Delete")
	})

	t.Run("error - ListPendingCleanup fails", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{Limit: 10}

		dbErr := errors.New("database error")
		repo.On("ListPendingCleanup", ctx, query).Return(stowry.ListResult{}, dbErr)

		count, err := service.Tombstone(ctx, query)
		assert.Error(t, err)
		assert.Equal(t, 0, count)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "Delete")
	})

	t.Run("error - storage delete fails with non-NotFound error", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{Limit: 10}

		id1 := uuid.New()

		pendingCleanup := stowry.ListResult{
			Items: []stowry.MetaData{
				{ID: id1, Path: "deleted/file1.txt", ContentType: "text/plain", FileSizeBytes: 100, Etag: "etag1"},
				{Path: "deleted/file2.txt", ContentType: "text/plain", FileSizeBytes: 200, Etag: "etag2"},
			},
			NextCursor: "",
		}

		deleteErr := errors.New("storage error")
		repo.On("ListPendingCleanup", ctx, query).Return(pendingCleanup, nil)
		storage.On("Delete", ctx, "deleted/file1.txt").Return(deleteErr)

		count, err := service.Tombstone(ctx, query)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "deleted/file1.txt")
		assert.Equal(t, 0, count)

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
		storage.AssertNotCalled(t, "Delete", ctx, "deleted/file2.txt")
	})

	t.Run("error - storage delete fails on second file", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{Limit: 10}

		id1 := uuid.New()
		id2 := uuid.New()
		id3 := uuid.New()

		pendingCleanup := stowry.ListResult{
			Items: []stowry.MetaData{
				{ID: id1, Path: "deleted/file1.txt", ContentType: "text/plain", FileSizeBytes: 100, Etag: "etag1"},
				{ID: id2, Path: "deleted/file2.txt", ContentType: "text/plain", FileSizeBytes: 200, Etag: "etag2"},
				{ID: id3, Path: "deleted/file3.txt", ContentType: "text/plain", FileSizeBytes: 300, Etag: "etag3"},
			},
			NextCursor: "",
		}

		deleteErr := errors.New("storage error on second file")
		repo.On("ListPendingCleanup", ctx, query).Return(pendingCleanup, nil)
		storage.On("Delete", ctx, "deleted/file1.txt").Return(nil)
		repo.On("MarkCleanedUp", ctx, id1).Return(nil)
		storage.On("Delete", ctx, "deleted/file2.txt").Return(deleteErr)

		count, err := service.Tombstone(ctx, query)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "deleted/file2.txt")
		assert.Equal(t, 1, count) // First file was cleaned up before error

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
		storage.AssertNotCalled(t, "Delete", ctx, "deleted/file3.txt")
	})

	t.Run("error - MarkCleanedUp fails on first file", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{Limit: 10}

		id1 := uuid.New()
		id2 := uuid.New()

		pendingCleanup := stowry.ListResult{
			Items: []stowry.MetaData{
				{ID: id1, Path: "deleted/file1.txt", ContentType: "text/plain", FileSizeBytes: 100, Etag: "etag1"},
				{ID: id2, Path: "deleted/file2.txt", ContentType: "text/plain", FileSizeBytes: 200, Etag: "etag2"},
			},
			NextCursor: "",
		}

		markErr := errors.New("database error marking cleaned up")
		repo.On("ListPendingCleanup", ctx, query).Return(pendingCleanup, nil)
		storage.On("Delete", ctx, "deleted/file1.txt").Return(nil)
		repo.On("MarkCleanedUp", ctx, id1).Return(markErr)

		count, err := service.Tombstone(ctx, query)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "deleted/file1.txt")
		assert.Equal(t, 0, count)

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
		storage.AssertNotCalled(t, "Delete", ctx, "deleted/file2.txt")
	})

	t.Run("error - MarkCleanedUp fails on second file", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{Limit: 10}

		id1 := uuid.New()
		id2 := uuid.New()
		id3 := uuid.New()

		pendingCleanup := stowry.ListResult{
			Items: []stowry.MetaData{
				{ID: id1, Path: "deleted/file1.txt", ContentType: "text/plain", FileSizeBytes: 100, Etag: "etag1"},
				{ID: id2, Path: "deleted/file2.txt", ContentType: "text/plain", FileSizeBytes: 200, Etag: "etag2"},
				{ID: id3, Path: "deleted/file3.txt", ContentType: "text/plain", FileSizeBytes: 300, Etag: "etag3"},
			},
			NextCursor: "",
		}

		markErr := errors.New("database error on second file")
		repo.On("ListPendingCleanup", ctx, query).Return(pendingCleanup, nil)
		storage.On("Delete", ctx, "deleted/file1.txt").Return(nil)
		repo.On("MarkCleanedUp", ctx, id1).Return(nil)
		storage.On("Delete", ctx, "deleted/file2.txt").Return(nil)
		repo.On("MarkCleanedUp", ctx, id2).Return(markErr)

		count, err := service.Tombstone(ctx, query)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "deleted/file2.txt")
		assert.Equal(t, 1, count) // First file was cleaned up before error

		repo.AssertExpectations(t)
		storage.AssertExpectations(t)
		storage.AssertNotCalled(t, "Delete", ctx, "deleted/file3.txt")
	})

	t.Run("error - context cancelled during list", func(t *testing.T) {
		service, repo, storage := NewStowryService(t)
		ctx := context.Background()

		query := stowry.ListQuery{Limit: 10}

		repo.On("ListPendingCleanup", ctx, query).Return(stowry.ListResult{}, context.Canceled)

		count, err := service.Tombstone(ctx, query)
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, count)

		repo.AssertExpectations(t)
		storage.AssertNotCalled(t, "Delete")
	})
}
