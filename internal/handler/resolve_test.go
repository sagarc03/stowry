package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sagarc03/stowry/internal/handler"
	"github.com/sagarc03/stowry/internal/service"
	"github.com/sagarc03/stowry/types"
	"github.com/stretchr/testify/assert"
)

// staticSite is laid out the way a static host expects.
func staticSite() map[string]stubObject {
	return map[string]stubObject{
		"index.html":      object("index.html", "home", "text/html", "e1"),
		"docs/index.html": object("docs/index.html", "docs index", "text/html", "e2"),
		"about.html":      object("about.html", "about", "text/html", "e3"),
		"app.js":          object("app.js", "console.log(1)", "text/javascript", "e4"),
	}
}

func TestResolveByMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mode     types.ServerMode
		target   string
		wantCode int
		wantBody string
	}{
		{name: "store serves an exact path", mode: types.ModeStore, target: "/app.js", wantCode: 200, wantBody: "console.log(1)"},
		{name: "store does not guess at .html", mode: types.ModeStore, target: "/about", wantCode: 404},
		{name: "store does not resolve a directory", mode: types.ModeStore, target: "/docs/", wantCode: 404},

		{name: "static serves the root index", mode: types.ModeStatic, target: "/", wantCode: 200, wantBody: "home"},
		{name: "static serves a directory index", mode: types.ModeStatic, target: "/docs/", wantCode: 200, wantBody: "docs index"},
		{name: "static resolves a bare name to .html", mode: types.ModeStatic, target: "/about", wantCode: 200, wantBody: "about"},
		{name: "static resolves a bare name to a directory index", mode: types.ModeStatic, target: "/docs", wantCode: 200, wantBody: "docs index"},
		{name: "static serves an exact path", mode: types.ModeStatic, target: "/app.js", wantCode: 200, wantBody: "console.log(1)"},
		{name: "static 404s an unknown path", mode: types.ModeStatic, target: "/nope", wantCode: 404},

		{name: "spa serves the root shell", mode: types.ModeSPA, target: "/", wantCode: 200, wantBody: "home"},
		{name: "spa serves an exact path", mode: types.ModeSPA, target: "/app.js", wantCode: 200, wantBody: "console.log(1)"},
		{name: "spa falls back to the shell", mode: types.ModeSPA, target: "/some/client/route", wantCode: 200, wantBody: "home"},
		{name: "spa serves a directory index", mode: types.ModeSPA, target: "/docs/", wantCode: 200, wantBody: "docs index"},
		{name: "spa falls back to the shell for an unknown directory", mode: types.ModeSPA, target: "/nope/", wantCode: 200, wantBody: "home"},
		{name: "static 404s an unknown directory", mode: types.ModeStatic, target: "/nope/", wantCode: 404},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := &handler.Opts{Mode: tt.mode, Svc: &stubService{objects: staticSite()}}

			w := serve(t, opts, httptest.NewRequest(http.MethodGet, tt.target, nil))

			assert.Equal(t, tt.wantCode, w.Code)
			if tt.wantBody != "" {
				assert.Equal(t, tt.wantBody, w.Body.String())
			}
		})
	}
}

// The response describes the object found, not the path asked for.
func TestResolveSetsContentTypeOfTheResolvedObject(t *testing.T) {
	t.Parallel()

	opts := &handler.Opts{Mode: types.ModeStatic, Svc: &stubService{objects: staticSite()}}

	w := serve(t, opts, httptest.NewRequest(http.MethodGet, "/about", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html", w.Header().Get("Content-Type"))
	assert.Equal(t, `"e3"`, w.Header().Get("ETag"))
}

func TestResolveAppliesToHead(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"/", "/docs/", "/about"} {
		opts := &handler.Opts{Mode: types.ModeStatic, Svc: &stubService{objects: staticSite()}}

		w := serve(t, opts, httptest.NewRequest(http.MethodHead, target, nil))

		assert.Equal(t, http.StatusOK, w.Code, target)
	}
}

func TestResolveStopsOnInvalidPath(t *testing.T) {
	t.Parallel()

	svc := &stubService{err: service.ErrInvalidInput}
	opts := &handler.Opts{Mode: types.ModeStatic, Svc: svc}

	w := serve(t, opts, httptest.NewRequest(http.MethodGet, "/bad", nil))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, 1, svc.opens, "a rejected path must not be retried as another candidate")
}
