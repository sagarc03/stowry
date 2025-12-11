package http

import (
	"net/http"
	"strings"

	"github.com/sagarc03/stowry"
)

type AuthMiddlewareConfig struct {
	AuthRequired bool
	Region       string
	Service      string
	AccessKeys   map[string]string
}

// AuthMiddleware creates middleware that enforces AWS Signature V4 authentication.
// Pass nil for accessKeys to disable authentication (public access).
func AuthMiddleware(cfg AuthMiddlewareConfig) func(http.Handler) http.Handler {
	if !cfg.AuthRequired {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// Create verifier once
	verifier := stowry.NewSignatureVerifier(cfg.Region, cfg.Service, func(accessKey string) (string, bool) {
		secretKey, found := cfg.AccessKeys[accessKey]
		return secretKey, found
	})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Copy headers and add Host (Go stores Host separately from Header)
			headers := r.Header.Clone()
			headers.Set("Host", r.Host)

			if err := verifier.Verify(r.Method, r.URL.Path, r.URL.Query(), headers); err != nil {
				HandleError(w, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// PathValidationMiddleware validates request paths
func PathValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract path (remove leading slash)
		path := strings.TrimPrefix(r.URL.Path, "/")

		// Root path is always valid
		if path == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Validate path using stowry's validation
		if !stowry.IsValidPath(path) {
			WriteError(w, http.StatusBadRequest, "invalid_path", "Invalid path format")
			return
		}

		next.ServeHTTP(w, r)
	})
}
