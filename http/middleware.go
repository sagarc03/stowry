package http

import "net/http"

// RequestVerifier verifies HTTP requests for authentication.
// Implementations should return nil if the request is valid,
// or an error (typically ErrUnauthorized) if verification fails.
type RequestVerifier interface {
	Verify(r *http.Request) error
}

// AuthMiddleware creates middleware that enforces signature authentication.
// If verifier is nil, requests pass through without authentication.
func AuthMiddleware(verifier RequestVerifier) func(http.Handler) http.Handler {
	if verifier == nil {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := verifier.Verify(r); err != nil {
				// TODO: log error
				HandleError(w, ErrUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
