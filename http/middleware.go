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
	Store        stowry.SecretStore
}

// AuthMiddleware creates middleware that enforces signature authentication.
// Supports both Stowry native signing and AWS Signature V4.
func AuthMiddleware(cfg AuthMiddlewareConfig) func(http.Handler) http.Handler {
	if !cfg.AuthRequired {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	verifier := stowry.NewSignatureVerifier(cfg.Region, cfg.Service, cfg.Store)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := verifier.Verify(r); err != nil {
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
		path := strings.TrimPrefix(r.URL.Path, "/")

		if path == "" {
			next.ServeHTTP(w, r)
			return
		}

		if !stowry.IsValidPath(path) {
			WriteError(w, http.StatusBadRequest, "invalid_path", "Invalid path format")
			return
		}

		next.ServeHTTP(w, r)
	})
}
