package middleware

import (
	"net/http"

	"github.com/rs/cors"
)

// Defaults for an empty config list. rs/cors would otherwise fall back to the
// spec's simple methods (GET, POST, HEAD) and to accept, content-type and
// x-requested-with. Store mode serves PUT and DELETE, and SigV4 clients sign
// their credentials into request headers.
var (
	defaultAllowedMethods = []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete}
	defaultAllowedHeaders = []string{
		"Accept",
		"Content-Type",
		"Authorization",
		"X-Amz-Date",
		"X-Amz-Content-Sha256",
		"X-Amz-Security-Token",
	}
)

// CORSConfig holds the cross-origin policy. An AllowedOrigins entry is "*", an
// exact origin, or an origin with one wildcard, such as "https://*.example.com".
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int // preflight cache lifetime in seconds
}

// WithCORS answers preflights and sets CORS headers on cross-origin responses.
// It must run before AuthMiddleware: browsers omit credentials from a
// preflight, so a signature check would reject every OPTIONS request.
func WithCORS(cfg CORSConfig) func(http.Handler) http.Handler { //nolint:gocritic // hugeParam: called once at startup, and a value keeps the caller from mutating it later
	return cors.New(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins, // rs/cors defaults this to ["*"]
		AllowedMethods:   orDefault(cfg.AllowedMethods, defaultAllowedMethods),
		AllowedHeaders:   orDefault(cfg.AllowedHeaders, defaultAllowedHeaders),
		ExposedHeaders:   cfg.ExposedHeaders,
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           cfg.MaxAge,
	}).Handler
}

func orDefault(v, def []string) []string {
	if len(v) == 0 {
		return def
	}
	return v
}
