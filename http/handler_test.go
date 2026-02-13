package http_test

import (
	"context"
	"encoding/json"
	"errors"
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

func (m *MockService) Info(ctx context.Context, path string) (stowry.MetaData, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(stowry.MetaData), args.Error(1)
}

func (m *MockService) List(ctx context.Context, query stowry.ListQuery) (stowry.ListResult, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(stowry.ListResult), args.Error(1)
}

func TestHandler_HandleList_StoreMode(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

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
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

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
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	service.On("List", mock.Anything, mock.MatchedBy(func(q stowry.ListQuery) bool {
		return q.Limit == 1000 // Max limit capped at 1000
	})).Return(stowry.ListResult{Items: []stowry.MetaData{}}, nil)

	req := httptest.NewRequest("GET", "/?limit=9999", nil) // Request more than max
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	service.AssertExpectations(t)
}

func TestHandler_HandleList_InvalidLimit(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	// No service call expected for invalid limit

	req := httptest.NewRequest("GET", "/?limit=abc", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_parameter")

	service.AssertExpectations(t)
}

func TestHandler_HandleGet_Success(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

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
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

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
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	// No service call expected for invalid path

	req := httptest.NewRequest("GET", "/../etc/passwd", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_path")

	service.AssertExpectations(t)
}

func TestHandler_HandleGet_IfNoneMatch_Match(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

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
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

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
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	req := httptest.NewRequest("PUT", "/../etc/passwd", strings.NewReader("hack"))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_path")

	service.AssertExpectations(t)
}

func TestHandler_HandlePut_EmptyPath(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	req := httptest.NewRequest("PUT", "/", strings.NewReader("content"))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	service.AssertExpectations(t)
}

func TestHandler_HandleDelete_Success(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	service.On("Delete", mock.Anything, "delete-me.txt").Return(nil)

	req := httptest.NewRequest("DELETE", "/delete-me.txt", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())

	service.AssertExpectations(t)
}

func TestHandler_HandleDelete_NotFound(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	service.On("Delete", mock.Anything, "missing.txt").Return(stowry.ErrNotFound)

	req := httptest.NewRequest("DELETE", "/missing.txt", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_found")

	service.AssertExpectations(t)
}

func TestHandler_HandleDelete_InvalidPath(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	req := httptest.NewRequest("DELETE", "/../etc/passwd", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	service.AssertExpectations(t)
}

func TestHandler_CORS_Disabled(t *testing.T) {
	config := &stowryhttp.HandlerConfig{
		Mode: stowry.ModeStore,
		CORS: stowryhttp.CORSConfig{Enabled: false},
	}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	service.On("List", mock.Anything, mock.Anything).Return(stowry.ListResult{Items: []stowry.MetaData{}}, nil)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestHandler_CORS_Enabled_Preflight(t *testing.T) {
	config := &stowryhttp.HandlerConfig{
		Mode: stowry.ModeStore,
		CORS: stowryhttp.CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "Authorization"},
			MaxAge:         300,
		},
	}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	req := httptest.NewRequest("OPTIONS", "/test.txt", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "PUT")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "PUT")
	assert.Equal(t, "300", rec.Header().Get("Access-Control-Max-Age"))
}

func TestHandler_CORS_Enabled_ActualRequest(t *testing.T) {
	config := &stowryhttp.HandlerConfig{
		Mode: stowry.ModeStore,
		CORS: stowryhttp.CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"http://localhost:3000"},
			AllowedMethods: []string{"GET", "PUT", "DELETE"},
			ExposedHeaders: []string{"ETag", "Content-Length"},
		},
	}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	service.On("List", mock.Anything, mock.Anything).Return(stowry.ListResult{Items: []stowry.MetaData{}}, nil)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Access-Control-Expose-Headers"), "Etag")
}

func TestHandler_StaticMode_RootPath_CallsGet(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStatic}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	content := "<html></html>"
	metadata := stowry.MetaData{
		ID:            uuid.New(),
		Path:          "index.html",
		ContentType:   "text/html",
		Etag:          "abc123",
		FileSizeBytes: int64(len(content)),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// In static mode, GET / should call Get with empty path (service handles index.html)
	service.On("Get", mock.Anything, "").Return(
		metadata,
		readSeekNopCloser{strings.NewReader(content)},
		nil,
	)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html", rec.Header().Get("Content-Type"))

	service.AssertExpectations(t)
	service.AssertNotCalled(t, "List")
}

func TestHandler_SPAMode_RootPath_CallsGet(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeSPA}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	content := "<html></html>"
	metadata := stowry.MetaData{
		ID:            uuid.New(),
		Path:          "index.html",
		ContentType:   "text/html",
		Etag:          "abc123",
		FileSizeBytes: int64(len(content)),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// In SPA mode, GET / should call Get with empty path (service handles index.html)
	service.On("Get", mock.Anything, "").Return(
		metadata,
		readSeekNopCloser{strings.NewReader(content)},
		nil,
	)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/html", rec.Header().Get("Content-Type"))

	service.AssertExpectations(t)
	service.AssertNotCalled(t, "List")
}

// MaxUploadSize tests

func TestHandler_HandlePut_MaxUploadSize_WithinLimit(t *testing.T) {
	config := &stowryhttp.HandlerConfig{
		Mode:          stowry.ModeStore,
		MaxUploadSize: 1024, // 1KB limit
	}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	content := strings.Repeat("x", 100) // 100 bytes, within limit
	metadata := stowry.MetaData{
		ID:            uuid.New(),
		Path:          "small.txt",
		ContentType:   "text/plain",
		Etag:          "abc123",
		FileSizeBytes: int64(len(content)),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	service.On("Create", mock.Anything, mock.MatchedBy(func(obj stowry.CreateObject) bool {
		return obj.Path == "small.txt"
	}), mock.Anything).Return(metadata, nil)

	req := httptest.NewRequest("PUT", "/small.txt", strings.NewReader(content))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	service.AssertExpectations(t)
}

func TestHandler_HandlePut_MaxUploadSize_NoLimit(t *testing.T) {
	config := &stowryhttp.HandlerConfig{
		Mode:          stowry.ModeStore,
		MaxUploadSize: 0, // No limit
	}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	content := strings.Repeat("x", 10000) // 10KB, should work with no limit
	metadata := stowry.MetaData{
		ID:            uuid.New(),
		Path:          "large.txt",
		ContentType:   "text/plain",
		Etag:          "abc123",
		FileSizeBytes: int64(len(content)),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	service.On("Create", mock.Anything, mock.MatchedBy(func(obj stowry.CreateObject) bool {
		return obj.Path == "large.txt"
	}), mock.Anything).Return(metadata, nil)

	req := httptest.NewRequest("PUT", "/large.txt", strings.NewReader(content))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	service.AssertExpectations(t)
}

// If-Match header tests for conditional updates

func TestHandler_HandlePut_IfMatch_Match(t *testing.T) {
	tests := []struct {
		name    string
		ifMatch string
	}{
		{"exact quoted", `"existing-etag"`},
		{"bare unquoted", `existing-etag`},
		{"wildcard", `*`},
		{"multiple with match", `"other", "existing-etag"`},
		{"multiple without match first", `"existing-etag", "other"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
			service := new(MockService)
			handler := stowryhttp.NewHandler(config, service)

			existingMetadata := stowry.MetaData{
				ID:            uuid.New(),
				Path:          "existing.txt",
				ContentType:   "text/plain",
				Etag:          "existing-etag",
				FileSizeBytes: 50,
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}

			newMetadata := stowry.MetaData{
				ID:            existingMetadata.ID,
				Path:          "existing.txt",
				ContentType:   "text/plain",
				Etag:          "new-etag",
				FileSizeBytes: 15,
				CreatedAt:     existingMetadata.CreatedAt,
				UpdatedAt:     time.Now(),
			}

			service.On("Info", mock.Anything, "existing.txt").Return(existingMetadata, nil)
			service.On("Create", mock.Anything, mock.MatchedBy(func(obj stowry.CreateObject) bool {
				return obj.Path == "existing.txt"
			}), mock.Anything).Return(newMetadata, nil)

			req := httptest.NewRequest("PUT", "/existing.txt", strings.NewReader("Updated content"))
			req.Header.Set("Content-Type", "text/plain")
			req.Header.Set("If-Match", tt.ifMatch)
			rec := httptest.NewRecorder()

			handler.Router().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			service.AssertExpectations(t)
		})
	}
}

func TestHandler_HandlePut_IfMatch_Mismatch(t *testing.T) {
	tests := []struct {
		name    string
		ifMatch string
	}{
		{"different etag", `"stale-etag"`},
		{"multiple without match", `"other", "nope"`},
		{"weak tag rejected", `W/"current-etag"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
			service := new(MockService)
			handler := stowryhttp.NewHandler(config, service)

			existingMetadata := stowry.MetaData{
				ID:            uuid.New(),
				Path:          "existing.txt",
				ContentType:   "text/plain",
				Etag:          "current-etag",
				FileSizeBytes: 50,
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}

			service.On("Info", mock.Anything, "existing.txt").Return(existingMetadata, nil)

			req := httptest.NewRequest("PUT", "/existing.txt", strings.NewReader("new content"))
			req.Header.Set("Content-Type", "text/plain")
			req.Header.Set("If-Match", tt.ifMatch)
			rec := httptest.NewRecorder()

			handler.Router().ServeHTTP(rec, req)

			assert.Equal(t, http.StatusPreconditionFailed, rec.Code)
			assert.Contains(t, rec.Body.String(), "precondition_failed")
			service.AssertNotCalled(t, "Create")
			service.AssertExpectations(t)
		})
	}
}

func TestHandler_HandlePut_IfMatch_FileNotExists(t *testing.T) {
	tests := []struct {
		name    string
		ifMatch string
	}{
		{"specific etag", `"any-etag"`},
		{"wildcard", `*`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
			service := new(MockService)
			handler := stowryhttp.NewHandler(config, service)

			service.On("Info", mock.Anything, "new.txt").Return(
				stowry.MetaData{},
				stowry.ErrNotFound,
			)

			req := httptest.NewRequest("PUT", "/new.txt", strings.NewReader("New file content"))
			req.Header.Set("Content-Type", "text/plain")
			req.Header.Set("If-Match", tt.ifMatch)
			rec := httptest.NewRecorder()

			handler.Router().ServeHTTP(rec, req)

			// RFC 9110 §13.1.1: If-Match is false when there is no current representation
			assert.Equal(t, http.StatusPreconditionFailed, rec.Code)
			service.AssertNotCalled(t, "Create")
			service.AssertExpectations(t)
		})
	}
}

// Limit edge cases

func TestHandler_HandleList_LimitZero(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	service.On("List", mock.Anything, mock.MatchedBy(func(q stowry.ListQuery) bool {
		return q.Limit == 1 // Zero or negative should be clamped to 1
	})).Return(stowry.ListResult{Items: []stowry.MetaData{}}, nil)

	req := httptest.NewRequest("GET", "/?limit=0", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	service.AssertExpectations(t)
}

func TestHandler_HandleList_NegativeLimit(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	service.On("List", mock.Anything, mock.MatchedBy(func(q stowry.ListQuery) bool {
		return q.Limit == 1 // Negative should be clamped to 1
	})).Return(stowry.ListResult{Items: []stowry.MetaData{}}, nil)

	req := httptest.NewRequest("GET", "/?limit=-10", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	service.AssertExpectations(t)
}

// Internal error tests - testing non-sentinel errors that trigger 500 responses

func TestHandler_HandleList_InternalError(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	service.On("List", mock.Anything, mock.Anything).Return(
		stowry.ListResult{},
		errors.New("database connection failed"),
	)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal_error")

	service.AssertExpectations(t)
}

func TestHandler_HandleGet_InternalError(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	service.On("Get", mock.Anything, "file.txt").Return(
		stowry.MetaData{},
		nil,
		errors.New("storage read failed"),
	)

	req := httptest.NewRequest("GET", "/file.txt", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal_error")

	service.AssertExpectations(t)
}

func TestHandler_HandlePut_InternalError(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	service.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(
		stowry.MetaData{},
		errors.New("storage write failed"),
	)

	req := httptest.NewRequest("PUT", "/file.txt", strings.NewReader("content"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal_error")

	service.AssertExpectations(t)
}

func TestHandler_HandlePut_IfMatch_InfoInternalError(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	// When checking If-Match, Info returns an internal error
	service.On("Info", mock.Anything, "file.txt").Return(
		stowry.MetaData{},
		errors.New("database error"),
	)

	req := httptest.NewRequest("PUT", "/file.txt", strings.NewReader("content"))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("If-Match", "some-etag")
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal_error")

	// Create should NOT be called due to Info error
	service.AssertNotCalled(t, "Create")
	service.AssertExpectations(t)
}

func TestHandler_HandleDelete_InternalError(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	service.On("Delete", mock.Anything, "file.txt").Return(
		errors.New("storage delete failed"),
	)

	req := httptest.NewRequest("DELETE", "/file.txt", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal_error")

	service.AssertExpectations(t)
}

// HEAD handler tests

func TestHandler_HandleHead_Success(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	updatedAt := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	metadata := stowry.MetaData{
		ID:            uuid.New(),
		Path:          "test.txt",
		ContentType:   "text/plain",
		Etag:          "abc123",
		FileSizeBytes: 1024,
		CreatedAt:     time.Now(),
		UpdatedAt:     updatedAt,
	}

	service.On("Info", mock.Anything, "test.txt").Return(metadata, nil)

	req := httptest.NewRequest("HEAD", "/test.txt", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
	assert.Equal(t, `"abc123"`, rec.Header().Get("ETag"))
	assert.Equal(t, "1024", rec.Header().Get("Content-Length"))
	assert.Equal(t, "bytes", rec.Header().Get("Accept-Ranges"))
	assert.Contains(t, rec.Header().Get("Last-Modified"), "Sun, 15 Jun 2025")
	assert.Empty(t, rec.Body.String())

	service.AssertExpectations(t)
}

func TestHandler_HandleHead_NotFound(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	service.On("Info", mock.Anything, "missing.txt").Return(
		stowry.MetaData{},
		stowry.ErrNotFound,
	)

	req := httptest.NewRequest("HEAD", "/missing.txt", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	service.AssertExpectations(t)
}

func TestHandler_HandleHead_InvalidPath(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	req := httptest.NewRequest("HEAD", "/../etc/passwd", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	service.AssertExpectations(t)
}

func TestHandler_HandleHead_InternalError(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	service.On("Info", mock.Anything, "file.txt").Return(
		stowry.MetaData{},
		errors.New("database error"),
	)

	req := httptest.NewRequest("HEAD", "/file.txt", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	service.AssertExpectations(t)
}

func TestHandler_HandleHead_AuthRequired(t *testing.T) {
	verifier := &mockVerifier{err: errors.New("unauthorized")}
	config := &stowryhttp.HandlerConfig{
		Mode:         stowry.ModeStore,
		ReadVerifier: verifier,
	}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	req := httptest.NewRequest("HEAD", "/test.txt", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	service.AssertNotCalled(t, "Info")
}

func TestHandler_HandleHead_IfNoneMatch(t *testing.T) {
	tests := []struct {
		name           string
		ifNoneMatch    string
		expectedStatus int
	}{
		{"exact match", `"abc123"`, http.StatusNotModified},
		{"no match", `"different"`, http.StatusOK},
		{"wildcard", `*`, http.StatusNotModified},
		{"multiple with match", `"other", "abc123"`, http.StatusNotModified},
		{"multiple without match", `"other", "nope"`, http.StatusOK},
		{"weak tag match", `W/"abc123"`, http.StatusNotModified},
		{"weak tag no match", `W/"different"`, http.StatusOK},
		{"multiple with weak match", `"other", W/"abc123"`, http.StatusNotModified},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
			service := new(MockService)
			handler := stowryhttp.NewHandler(config, service)

			metadata := stowry.MetaData{
				ID:            uuid.New(),
				Path:          "test.txt",
				ContentType:   "text/plain",
				Etag:          "abc123",
				FileSizeBytes: 1024,
				UpdatedAt:     time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
			}

			service.On("Info", mock.Anything, "test.txt").Return(metadata, nil)

			req := httptest.NewRequest("HEAD", "/test.txt", nil)
			req.Header.Set("If-None-Match", tt.ifNoneMatch)
			rec := httptest.NewRecorder()

			handler.Router().ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Equal(t, `"abc123"`, rec.Header().Get("ETag"))
			assert.Empty(t, rec.Body.String())

			service.AssertExpectations(t)
		})
	}
}

func TestHandler_HandleHead_IfModifiedSinceNotModified(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	updatedAt := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	metadata := stowry.MetaData{
		ID:            uuid.New(),
		Path:          "test.txt",
		ContentType:   "text/plain",
		Etag:          "abc123",
		FileSizeBytes: 1024,
		UpdatedAt:     updatedAt,
	}

	service.On("Info", mock.Anything, "test.txt").Return(metadata, nil)

	req := httptest.NewRequest("HEAD", "/test.txt", nil)
	req.Header.Set("If-Modified-Since", updatedAt.UTC().Format(http.TimeFormat))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotModified, rec.Code)
	assert.Empty(t, rec.Body.String())

	service.AssertExpectations(t)
}

func TestHandler_HandleHead_IfModifiedSinceModified(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	updatedAt := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	metadata := stowry.MetaData{
		ID:            uuid.New(),
		Path:          "test.txt",
		ContentType:   "text/plain",
		Etag:          "abc123",
		FileSizeBytes: 1024,
		UpdatedAt:     updatedAt,
	}

	service.On("Info", mock.Anything, "test.txt").Return(metadata, nil)

	req := httptest.NewRequest("HEAD", "/test.txt", nil)
	// One hour before the object was updated
	req.Header.Set("If-Modified-Since", updatedAt.Add(-1*time.Hour).UTC().Format(http.TimeFormat))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	service.AssertExpectations(t)
}

func TestHandler_HandleHead_IfNoneMatchTakesPrecedence(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStore}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	updatedAt := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	metadata := stowry.MetaData{
		ID:            uuid.New(),
		Path:          "test.txt",
		ContentType:   "text/plain",
		Etag:          "abc123",
		FileSizeBytes: 1024,
		UpdatedAt:     updatedAt,
	}

	service.On("Info", mock.Anything, "test.txt").Return(metadata, nil)

	req := httptest.NewRequest("HEAD", "/test.txt", nil)
	// ETag matches (→ 304) but If-Modified-Since is old (→ 200 if evaluated alone)
	// Per RFC 7232, If-None-Match takes precedence
	req.Header.Set("If-None-Match", `"abc123"`)
	req.Header.Set("If-Modified-Since", updatedAt.Add(-1*time.Hour).UTC().Format(http.TimeFormat))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotModified, rec.Code)

	service.AssertExpectations(t)
}

func TestHandler_StaticMode_PutReturns405(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStatic}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	req := httptest.NewRequest("PUT", "/test.txt", strings.NewReader("content"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	service.AssertNotCalled(t, "Create")
}

func TestHandler_StaticMode_DeleteReturns405(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeStatic}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	req := httptest.NewRequest("DELETE", "/test.txt", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	service.AssertNotCalled(t, "Delete")
}

func TestHandler_SPAMode_PutReturns405(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeSPA}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	req := httptest.NewRequest("PUT", "/test.txt", strings.NewReader("content"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	service.AssertNotCalled(t, "Create")
}

func TestHandler_SPAMode_DeleteReturns405(t *testing.T) {
	config := &stowryhttp.HandlerConfig{Mode: stowry.ModeSPA}
	service := new(MockService)
	handler := stowryhttp.NewHandler(config, service)

	req := httptest.NewRequest("DELETE", "/test.txt", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	service.AssertNotCalled(t, "Delete")
}

type mockVerifier struct {
	err error
}

func (m *mockVerifier) Verify(_ *http.Request) error {
	return m.err
}
