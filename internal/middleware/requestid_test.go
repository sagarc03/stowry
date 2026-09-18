package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"
)

func TestRequestIDEmptyWhenMissing(t *testing.T) {
	if got := RequestID(context.Background()); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestWithRequestIDGenerates(t *testing.T) {
	var seen string
	h := WithRequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = RequestID(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if _, err := uuid.Parse(seen); err != nil {
		t.Fatalf("ctx id %q is not a uuid: %v", seen, err)
	}
	if got := rec.Header().Get(RequestIDHeader); got != seen {
		t.Errorf("response header %q != ctx id %q", got, seen)
	}
}

func TestWithRequestIDPropagatesIncoming(t *testing.T) {
	const want = "client-supplied-id"
	var seen string
	h := WithRequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = RequestID(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, want)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen != want {
		t.Errorf("ctx id %q, want %q", seen, want)
	}
	if got := rec.Header().Get(RequestIDHeader); got != "" {
		t.Errorf("response header should be unset for client-supplied id, got %q", got)
	}
}
