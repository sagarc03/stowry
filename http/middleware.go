package http

import (
	"net/http"
	"strings"

	"github.com/sagarc03/stowry"
	stowrysign "github.com/sagarc03/stowry-go"
)

type AuthMiddlewareConfig struct {
	AuthRequired bool
	Region       string
	Service      string
	AccessKeys   map[string]string
}

// AuthMiddleware creates middleware that enforces signature authentication.
// Supports both Stowry native signing and AWS Signature V4.
// Pass nil for accessKeys to disable authentication (public access).
func AuthMiddleware(cfg AuthMiddlewareConfig) func(http.Handler) http.Handler {
	if !cfg.AuthRequired {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	lookup := func(accessKey string) (string, bool) {
		secretKey, found := cfg.AccessKeys[accessKey]
		return secretKey, found
	}

	// Create verifiers once
	stowryVerifier := stowrysign.NewVerifier(lookup)
	awsVerifier := stowry.NewSignatureVerifier(cfg.Region, cfg.Service, lookup)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			var err error

			switch detectSigningScheme(query) {
			case "stowry":
				err = stowryVerifier.Verify(r.Method, r.URL.Path, query)
			case "aws-v4":
				// Copy headers and add Host (Go stores Host separately from Header)
				headers := r.Header.Clone()
				headers.Set("Host", r.Host)
				err = awsVerifier.Verify(r.Method, r.URL.Path, query, headers)
			default:
				err = stowry.ErrUnauthorized
			}

			if err != nil {
				HandleError(w, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// detectSigningScheme determines which signing scheme is used based on query parameters.
func detectSigningScheme(query map[string][]string) string {
	if _, ok := query["X-Stowry-Signature"]; ok {
		return "stowry"
	}
	if _, ok := query["X-Amz-Signature"]; ok {
		return "aws-v4"
	}
	return "none"
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
