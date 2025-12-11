package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	stowryhttp "github.com/sagarc03/stowry/http"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware_PublicAccess(t *testing.T) {
	// Handler that just writes OK
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with auth middleware (auth not required = public access)
	cfg := stowryhttp.AuthMiddlewareConfig{
		AuthRequired: false,
		Region:       "us-east-1",
		Service:      "s3",
		AccessKeys:   nil,
	}
	wrapped := stowryhttp.AuthMiddleware(cfg)(handler)

	req := httptest.NewRequest("GET", "/test.txt", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())
}

func TestAuthMiddleware_RequiresAuth_NoSignature(t *testing.T) {
	// Handler that shouldn't be reached
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	// Wrap with auth middleware (requires auth)
	cfg := stowryhttp.AuthMiddlewareConfig{
		AuthRequired: true,
		Region:       "us-east-1",
		Service:      "s3",
		AccessKeys: map[string]string{
			"AKIATEST": "testsecret",
		},
	}
	wrapped := stowryhttp.AuthMiddleware(cfg)(handler)

	req := httptest.NewRequest("GET", "/test.txt", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing required signature parameters")
}

func TestAuthMiddleware_RequiresAuth_InvalidSignature(t *testing.T) {
	// Handler that shouldn't be reached
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	// Wrap with auth middleware (requires auth)
	cfg := stowryhttp.AuthMiddlewareConfig{
		AuthRequired: true,
		Region:       "us-east-1",
		Service:      "s3",
		AccessKeys: map[string]string{
			"AKIATEST": "testsecret",
		},
	}
	wrapped := stowryhttp.AuthMiddleware(cfg)(handler)

	// Request with invalid signature
	req := httptest.NewRequest("GET", "/test.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=WRONGKEY/20260112/us-east-1/s3/aws4_request&X-Amz-Date=20260112T070000Z&X-Amz-Expires=3600&X-Amz-SignedHeaders=host&X-Amz-Signature=invalid", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestPathValidationMiddleware(t *testing.T) {
	// Handler that just writes OK
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrapped := stowryhttp.PathValidationMiddleware(handler)

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "root path",
			path:       "/",
			wantStatus: http.StatusOK,
		},
		{
			name:       "valid path",
			path:       "/test.txt",
			wantStatus: http.StatusOK,
		},
		{
			name:       "nested path",
			path:       "/dir/file.txt",
			wantStatus: http.StatusOK,
		},
		{
			name:       "path traversal",
			path:       "/../etc/passwd",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "double slash",
			path:       "/dir//file.txt",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "trailing slash",
			path:       "/dir/",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "absolute path",
			path:       "//absolute",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}
