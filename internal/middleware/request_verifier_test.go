package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sagarc03/stowry/internal/keybackend"
	"github.com/sagarc03/stowry/internal/middleware"
	"github.com/sagarc03/stowry/sign"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()

	verifier := sign.NewSignatureVerifier(
		sign.AuthConfig{AWS: sign.AWSConfig{Region: "us-east-1", Service: "s3"}},
		keybackend.NewMapSecretStore(map[string]string{"AKIATEST": "testsecret"}),
	)

	// Presigned with a key the store does not hold.
	const wrongKey = "/test.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256" +
		"&X-Amz-Credential=WRONGKEY/20260112/us-east-1/s3/aws4_request" +
		"&X-Amz-Date=20260112T070000Z&X-Amz-Expires=3600" +
		"&X-Amz-SignedHeaders=host&X-Amz-Signature=invalid"

	tests := []struct {
		name       string
		verifier   middleware.RequestVerifier
		target     string
		wantStatus int
		wantNext   bool
	}{
		{
			name:       "a nil verifier leaves the route public",
			target:     "/test.txt",
			wantStatus: http.StatusOK,
			wantNext:   true,
		},
		{
			name:       "an unsigned request is rejected",
			verifier:   verifier,
			target:     "/test.txt",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "an unknown access key is rejected",
			verifier:   verifier,
			target:     wrongKey,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var reached bool
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				reached = true
				_, _ = io.WriteString(w, "OK")
			})

			rec := httptest.NewRecorder()
			middleware.AuthMiddleware(tt.verifier)(next).
				ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.target, nil))

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantNext, reached, "whether the request reached the next handler")

			if !tt.wantNext {
				assert.Contains(t, rec.Body.String(), "unauthorized")
			}
		})
	}
}
