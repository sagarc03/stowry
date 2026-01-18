package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sagarc03/stowry"
	stowryhttp "github.com/sagarc03/stowry/http"
	"github.com/sagarc03/stowry/keybackend"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware_PublicAccess(t *testing.T) {
	// Handler that just writes OK
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with auth middleware (nil verifier = public access)
	wrapped := stowryhttp.AuthMiddleware(nil)(handler)

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

	// Create verifier with auth required
	store := keybackend.NewMapSecretStore(map[string]string{
		"AKIATEST": "testsecret",
	})
	verifier := stowry.NewSignatureVerifier("us-east-1", "s3", store)
	wrapped := stowryhttp.AuthMiddleware(verifier)(handler)

	req := httptest.NewRequest("GET", "/test.txt", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "unauthorized")
}

func TestAuthMiddleware_RequiresAuth_InvalidSignature(t *testing.T) {
	// Handler that shouldn't be reached
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	// Create verifier with auth required
	store := keybackend.NewMapSecretStore(map[string]string{
		"AKIATEST": "testsecret",
	})
	verifier := stowry.NewSignatureVerifier("us-east-1", "s3", store)
	wrapped := stowryhttp.AuthMiddleware(verifier)(handler)

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
