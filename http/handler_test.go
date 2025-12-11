package http_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sagarc03/stowry"
	stowryhttp "github.com/sagarc03/stowry/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// readSeekNopCloser wraps an io.ReadSeeker to add a no-op Close method
type readSeekNopCloser struct {
	io.ReadSeeker
}

func (r readSeekNopCloser) Close() error { return nil }

// MockService is a mock implementation of http.Service
type MockService struct {
	mock.Mock
}

func (m *MockService) Get(ctx context.Context, path string) (stowry.MetaData, io.ReadSeekCloser, error) {
	args := m.Called(ctx, path)
	if args.Get(1) == nil {
		return args.Get(0).(stowry.MetaData), nil, args.Error(2)
	}
	return args.Get(0).(stowry.MetaData), args.Get(1).(io.ReadSeekCloser), args.Error(2)
}

func (m *MockService) Create(ctx context.Context, obj stowry.CreateObject, content io.Reader) (stowry.MetaData, error) {
	args := m.Called(ctx, obj, content)
	return args.Get(0).(stowry.MetaData), args.Error(1)
}

func (m *MockService) Delete(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockService) List(ctx context.Context, query stowry.ListQuery) (stowry.ListResult, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(stowry.ListResult), args.Error(1)
}

func TestHandler_HandleList_StoreMode(t *testing.T) {
	config := &stowryhttp.HandlerConfig{PublicRead: true, Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(*config, service)

	// Mock list response
	expectedResult := stowry.ListResult{
		Items: []stowry.MetaData{
			{
				ID:            uuid.New(),
				Path:          "file1.txt",
				ContentType:   "text/plain",
				Etag:          "abc123",
				FileSizeBytes: 100,
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			},
		},
		NextCursor: "cursor123",
	}

	service.On("List", mock.Anything, mock.MatchedBy(func(q stowry.ListQuery) bool {
		return q.PathPrefix == "docs/" && q.Limit == 50
	})).Return(expectedResult, nil)

	req := httptest.NewRequest("GET", "/?prefix=docs/&limit=50", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var result stowry.ListResult
	err := json.NewDecoder(rec.Body).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result.Items))
	assert.Equal(t, "file1.txt", result.Items[0].Path)

	service.AssertExpectations(t)
}

func TestHandler_HandleList_DefaultLimit(t *testing.T) {
	config := &stowryhttp.HandlerConfig{PublicRead: true, Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(*config, service)

	service.On("List", mock.Anything, mock.MatchedBy(func(q stowry.ListQuery) bool {
		return q.Limit == 100 // Default limit
	})).Return(stowry.ListResult{Items: []stowry.MetaData{}}, nil)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	service.AssertExpectations(t)
}

func TestHandler_HandleList_MaxLimit(t *testing.T) {
	config := &stowryhttp.HandlerConfig{PublicRead: true, Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(*config, service)

	service.On("List", mock.Anything, mock.MatchedBy(func(q stowry.ListQuery) bool {
		return q.Limit == 1000 // Max limit capped at 1000
	})).Return(stowry.ListResult{Items: []stowry.MetaData{}}, nil)

	req := httptest.NewRequest("GET", "/?limit=9999", nil) // Request more than max
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	service.AssertExpectations(t)
}

func TestHandler_HandleGet_Success(t *testing.T) {
	config := &stowryhttp.HandlerConfig{PublicRead: true, Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(*config, service)

	content := "Hello, World!"
	metadata := stowry.MetaData{
		ID:            uuid.New(),
		Path:          "test.txt",
		ContentType:   "text/plain",
		Etag:          "abc123",
		FileSizeBytes: int64(len(content)),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	service.On("Get", mock.Anything, "test.txt").Return(
		metadata,
		readSeekNopCloser{strings.NewReader(content)},
		nil,
	)

	req := httptest.NewRequest("GET", "/test.txt", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	assert.Equal(t, `"abc123"`, rec.Header().Get("ETag"))
	assert.Equal(t, "13", rec.Header().Get("Content-Length"))
	assert.Equal(t, content, rec.Body.String())

	service.AssertExpectations(t)
}

func TestHandler_HandleGet_NotFound(t *testing.T) {
	config := &stowryhttp.HandlerConfig{PublicRead: true, Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(*config, service)

	service.On("Get", mock.Anything, "missing.txt").Return(
		stowry.MetaData{},
		nil,
		stowry.ErrNotFound,
	)

	req := httptest.NewRequest("GET", "/missing.txt", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_found")

	service.AssertExpectations(t)
}

func TestHandler_HandleGet_InvalidPath(t *testing.T) {
	config := &stowryhttp.HandlerConfig{PublicRead: true, Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(*config, service)

	// No service call expected for invalid path

	req := httptest.NewRequest("GET", "/../etc/passwd", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_path")

	service.AssertExpectations(t)
}

func TestHandler_HandleGet_IfNoneMatch_Match(t *testing.T) {
	config := &stowryhttp.HandlerConfig{PublicRead: true, Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(*config, service)

	metadata := stowry.MetaData{
		ID:            uuid.New(),
		Path:          "test.txt",
		ContentType:   "text/plain",
		Etag:          "abc123",
		FileSizeBytes: 100,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	service.On("Get", mock.Anything, "test.txt").Return(
		metadata,
		readSeekNopCloser{strings.NewReader("content")},
		nil,
	)

	req := httptest.NewRequest("GET", "/test.txt", nil)
	req.Header.Set("If-None-Match", `"abc123"`)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotModified, rec.Code)
	assert.Empty(t, rec.Body.String())

	service.AssertExpectations(t)
}

func TestHandler_HandlePut_Success(t *testing.T) {
	config := &stowryhttp.HandlerConfig{PublicWrite: true, Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(*config, service)

	content := "New file content"
	metadata := stowry.MetaData{
		ID:            uuid.New(),
		Path:          "new.txt",
		ContentType:   "text/plain",
		Etag:          "def456",
		FileSizeBytes: int64(len(content)),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	service.On("Create", mock.Anything, mock.MatchedBy(func(obj stowry.CreateObject) bool {
		return obj.Path == "new.txt" && obj.ContentType == "text/plain"
	}), mock.Anything).Return(metadata, nil)

	req := httptest.NewRequest("PUT", "/new.txt", strings.NewReader(content))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result stowry.MetaData
	err := json.NewDecoder(rec.Body).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, "new.txt", result.Path)
	assert.Equal(t, "def456", result.Etag)

	service.AssertExpectations(t)
}

func TestHandler_HandlePut_InvalidPath(t *testing.T) {
	config := &stowryhttp.HandlerConfig{PublicWrite: true, Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(*config, service)

	req := httptest.NewRequest("PUT", "/../etc/passwd", strings.NewReader("hack"))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_path")

	service.AssertExpectations(t)
}

func TestHandler_HandlePut_EmptyPath(t *testing.T) {
	config := &stowryhttp.HandlerConfig{PublicWrite: true, Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(*config, service)

	req := httptest.NewRequest("PUT", "/", strings.NewReader("content"))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	service.AssertExpectations(t)
}

func TestHandler_HandleDelete_Success(t *testing.T) {
	config := &stowryhttp.HandlerConfig{PublicWrite: true, Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(*config, service)

	service.On("Delete", mock.Anything, "delete-me.txt").Return(nil)

	req := httptest.NewRequest("DELETE", "/delete-me.txt", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())

	service.AssertExpectations(t)
}

func TestHandler_HandleDelete_NotFound(t *testing.T) {
	config := &stowryhttp.HandlerConfig{PublicWrite: true, Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(*config, service)

	service.On("Delete", mock.Anything, "missing.txt").Return(stowry.ErrNotFound)

	req := httptest.NewRequest("DELETE", "/missing.txt", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_found")

	service.AssertExpectations(t)
}

func TestHandler_HandleDelete_InvalidPath(t *testing.T) {
	config := &stowryhttp.HandlerConfig{PublicWrite: true, Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(*config, service)

	req := httptest.NewRequest("DELETE", "/../etc/passwd", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	service.AssertExpectations(t)
}
