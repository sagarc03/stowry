package middleware

import (
	"context"
	"net/http"
	"uuid"
)

// RequestIDHeader is the canonical header name for per-request correlation IDs
const RequestIDHeader = "X-Stowry-Request-Id"

type requestIDCtxKey struct{}

// RequestID return request ID from ctx. Empty string when missing.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDCtxKey{}).(string)
	return id
}

// ContextWithRequestID returns ctx carrying id, for RequestID to read back.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDCtxKey{}, id)
}

// WithRequestID stores id on ctx for downstream lookup.
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = uuid.NewV7().String()

			w.Header().Set(RequestIDHeader, id)
		}
		next.ServeHTTP(w, r.WithContext(ContextWithRequestID(r.Context(), id)))
	})
}
