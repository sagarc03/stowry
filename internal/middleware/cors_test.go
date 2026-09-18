package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// These tests cover what WithCORS adds over rs/cors: option mapping, the
// defaults, and preflight ordering. rs/cors matching rules are not retested.

const (
	headerOrigin           = "Origin"
	headerACRequestMethod  = "Access-Control-Request-Method"
	headerACRequestHeaders = "Access-Control-Request-Headers"
	headerACAllowOrigin    = "Access-Control-Allow-Origin"
	headerACAllowMethods   = "Access-Control-Allow-Methods"
	headerACAllowHeaders   = "Access-Control-Allow-Headers"
	headerACExposeHeaders  = "Access-Control-Expose-Headers"
	headerACAllowCreds     = "Access-Control-Allow-Credentials"
	headerACMaxAge         = "Access-Control-Max-Age"
)

// corsOK is the config most tests start from: one exact origin, the rest
// left at its default.
func corsOK() CORSConfig {
	return CORSConfig{AllowedOrigins: []string{"https://example.com"}}
}

// serveCORS runs req through WithCORS(cfg) and reports whether the wrapped
// handler ran.
func serveCORS(cfg CORSConfig, req *http.Request) (*httptest.ResponseRecorder, bool) {
	var called bool
	h := WithCORS(cfg)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec, called
}

func request(method, origin string) *http.Request {
	req := httptest.NewRequest(method, "/things", nil)
	if origin != "" {
		req.Header.Set(headerOrigin, origin)
	}
	return req
}

func preflight(origin, method, headers string) *http.Request {
	req := request(http.MethodOptions, origin)
	req.Header.Set(headerACRequestMethod, method)
	if headers != "" {
		req.Header.Set(headerACRequestHeaders, headers)
	}
	return req
}

func TestWithCORSAllowedOrigin(t *testing.T) {
	rec, called := serveCORS(corsOK(), request(http.MethodGet, "https://example.com"))
	if !called {
		t.Fatal("handler not called")
	}
	if got := rec.Header().Get(headerACAllowOrigin); got != "https://example.com" {
		t.Errorf("allow-origin = %q", got)
	}
	if got := rec.Header().Get(headerACAllowCreds); got != "" {
		t.Errorf("allow-credentials = %q, want none", got)
	}
}

func TestWithCORSDisallowedOriginIsStillServed(t *testing.T) {
	rec, called := serveCORS(corsOK(), request(http.MethodGet, "https://evil.com"))
	if !called {
		t.Fatal("handler not called: CORS is enforced by the browser, not the server")
	}
	if got := rec.Header().Get(headerACAllowOrigin); got != "" {
		t.Errorf("allow-origin = %q, want none", got)
	}
}

func TestWithCORSSameOriginRequestUntouched(t *testing.T) {
	rec, called := serveCORS(corsOK(), request(http.MethodGet, ""))
	if !called {
		t.Fatal("handler not called")
	}
	if got := rec.Header().Get(headerACAllowOrigin); got != "" {
		t.Errorf("allow-origin = %q, want none for a request without Origin", got)
	}
}

func TestWithCORSWildcardOrigin(t *testing.T) {
	cfg := CORSConfig{AllowedOrigins: []string{"https://*.example.com"}}

	rec, _ := serveCORS(cfg, request(http.MethodGet, "https://app.example.com"))
	if got := rec.Header().Get(headerACAllowOrigin); got != "https://app.example.com" {
		t.Errorf("allow-origin = %q, want the wildcard to match", got)
	}

	rec, _ = serveCORS(cfg, request(http.MethodGet, "https://app.example.com.evil.net"))
	if got := rec.Header().Get(headerACAllowOrigin); got != "" {
		t.Errorf("allow-origin = %q, want none: the suffix does not match", got)
	}
}

func TestWithCORSCredentials(t *testing.T) {
	cfg := corsOK()
	cfg.AllowCredentials = true

	rec, _ := serveCORS(cfg, request(http.MethodGet, "https://example.com"))
	if got := rec.Header().Get(headerACAllowCreds); got != "true" {
		t.Errorf("allow-credentials = %q, want true", got)
	}
}

func TestWithCORSExposedHeaders(t *testing.T) {
	cfg := corsOK()
	cfg.ExposedHeaders = []string{"ETag", RequestIDHeader}

	// rs/cors canonicalizes the names, so "ETag" goes out as "Etag". Header
	// names are case-insensitive, so browsers treat the two alike.
	rec, _ := serveCORS(cfg, request(http.MethodGet, "https://example.com"))
	if got := rec.Header().Get(headerACExposeHeaders); got != "Etag, "+RequestIDHeader {
		t.Errorf("expose-headers = %q", got)
	}
}

func TestWithCORSPreflightShortCircuits(t *testing.T) {
	cfg := corsOK()
	cfg.MaxAge = 600

	rec, called := serveCORS(cfg, preflight("https://example.com", http.MethodPut, "content-type"))
	if called {
		t.Fatal("preflight must not reach the handler")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("code = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
	if got := rec.Header().Get(headerACAllowOrigin); got != "https://example.com" {
		t.Errorf("allow-origin = %q", got)
	}
	if got := rec.Header().Get(headerACMaxAge); got != "600" {
		t.Errorf("max-age = %q", got)
	}
}

// failingVerifier rejects every request.
type failingVerifier struct{}

func (failingVerifier) Verify(*http.Request) error { return errors.New("no signature") }

// Preflights carry no credentials, so CORS must answer before AuthMiddleware
// verifies a signature.
func TestWithCORSPreflightIsNotAuthenticated(t *testing.T) {
	h := WithCORS(corsOK())(AuthMiddleware(failingVerifier{})(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("handler should not be called")
		})))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, preflight("https://example.com", http.MethodPut, "authorization"))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("code = %d, want 204: CORS must answer before AuthMiddleware", rec.Code)
	}
	if got := rec.Header().Get(headerACAllowOrigin); got != "https://example.com" {
		t.Errorf("allow-origin = %q", got)
	}
}

// rs/cors defaults to GET, POST and HEAD, which would block store-mode writes.
func TestWithCORSDefaultMethodsCoverWrites(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			rec, _ := serveCORS(corsOK(), preflight("https://example.com", method, ""))
			if got := rec.Header().Get(headerACAllowMethods); got != method {
				t.Errorf("allow-methods = %q, want %q", got, method)
			}
		})
	}
}

// rs/cors defaults to accept, content-type and x-requested-with, which would
// reject a header-signed SigV4 request at the preflight.
func TestWithCORSDefaultHeadersCoverSigV4(t *testing.T) {
	// Lowercase, sorted and unique. rs/cors relies on the order the Fetch
	// standard guarantees browsers send, and rejects any other.
	const signing = "authorization, x-amz-content-sha256, x-amz-date"

	rec, _ := serveCORS(corsOK(), preflight("https://example.com", http.MethodPut, signing))
	if got := rec.Header().Get(headerACAllowHeaders); got != signing {
		t.Errorf("allow-headers = %q, want the signing headers echoed back", got)
	}
}

func TestWithCORSExplicitListsOverrideDefaults(t *testing.T) {
	cfg := corsOK()
	cfg.AllowedMethods = []string{http.MethodGet}
	cfg.AllowedHeaders = []string{"Content-Type"}

	rec, _ := serveCORS(cfg, preflight("https://example.com", http.MethodDelete, ""))
	if got := rec.Header().Get(headerACAllowOrigin); got != "" {
		t.Errorf("allow-origin = %q, want none: DELETE is not in the configured methods", got)
	}

	rec, _ = serveCORS(cfg, preflight("https://example.com", http.MethodGet, "authorization"))
	if got := rec.Header().Get(headerACAllowHeaders); got != "" {
		t.Errorf("allow-headers = %q, want none: authorization is not in the configured headers", got)
	}
}
