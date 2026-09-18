package handler_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/sagarc03/stowry/internal/handler"
	"github.com/sagarc03/stowry/internal/response"
	"github.com/sagarc03/stowry/internal/service"
	"github.com/sagarc03/stowry/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errBoom = errors.New("boom")

// stubService records what it was asked for. An object it does not hold is
// reported as ErrNotFound.
type stubService struct {
	objects map[string]stubObject
	listed  types.ListResult
	// err fails every call; errFor fails only the paths it names.
	err    error
	errFor map[string]error

	gotQuery  types.ListQuery
	gotCreate types.CreateObject
	gotBody   []byte
	deleted   []string
	// opens counts calls to Get, the only method that opens content.
	opens int
}

type stubObject struct {
	meta    types.MetaData
	content string
}

func (s *stubService) Get(_ context.Context, path string) (types.MetaData, io.ReadSeekCloser, error) {
	s.opens++

	if err := s.failure(path); err != nil {
		return types.MetaData{}, nil, err
	}

	obj, ok := s.objects[path]
	if !ok {
		return types.MetaData{}, nil, service.ErrNotFound
	}

	return obj.meta, nopCloser{strings.NewReader(obj.content)}, nil
}

func (s *stubService) Info(_ context.Context, path string) (types.MetaData, error) {
	if err := s.failure(path); err != nil {
		return types.MetaData{}, err
	}

	obj, ok := s.objects[path]
	if !ok {
		return types.MetaData{}, service.ErrNotFound
	}

	return obj.meta, nil
}

func (s *stubService) Create(_ context.Context, obj types.CreateObject, content io.Reader) (types.MetaData, error) {
	s.gotCreate = obj

	body, err := io.ReadAll(content)
	if err != nil {
		return types.MetaData{}, err
	}
	s.gotBody = body

	if s.err != nil {
		return types.MetaData{}, s.err
	}

	return types.MetaData{ID: uuid.New(), Path: obj.Path, ContentType: obj.ContentType, FileSizeBytes: int64(len(body))}, nil
}

func (s *stubService) Delete(_ context.Context, path string) error {
	s.deleted = append(s.deleted, path)
	if s.err != nil {
		return s.err
	}
	if _, ok := s.objects[path]; !ok {
		return service.ErrNotFound
	}

	return nil
}

func (s *stubService) List(_ context.Context, q types.ListQuery) (types.ListResult, error) {
	s.gotQuery = q
	if s.err != nil {
		return types.ListResult{}, s.err
	}

	return s.listed, nil
}

func (s *stubService) failure(path string) error {
	if err, ok := s.errFor[path]; ok {
		return err
	}

	return s.err
}

type nopCloser struct{ io.ReadSeeker }

func (nopCloser) Close() error { return nil }

var modTime = time.Date(2024, 3, 10, 8, 0, 0, 0, time.UTC)

func object(path, content, contentType, etag string) stubObject {
	return stubObject{
		meta: types.MetaData{
			ID:            uuid.New(),
			Path:          path,
			ContentType:   contentType,
			Etag:          etag,
			FileSizeBytes: int64(len(content)),
			UpdatedAt:     modTime,
		},
		content: content,
	}
}

// serve registers the routes for opts and runs one request through them.
func serve(t *testing.T, opts *handler.Opts, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	opts.Mux = http.NewServeMux()
	opts.Logger = slog.New(slog.DiscardHandler)
	handler.Register(opts)

	w := httptest.NewRecorder()
	opts.Mux.ServeHTTP(w, r)

	return w
}

func storeOpts(svc handler.Service) *handler.Opts {
	return &handler.Opts{Mode: types.ModeStore, Svc: svc}
}

func decodeError(t *testing.T, w *httptest.ResponseRecorder) response.ErrorBody {
	t.Helper()

	var body response.ErrorBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	return body
}

func TestHandleGet(t *testing.T) {
	t.Parallel()

	svc := &stubService{objects: map[string]stubObject{
		"docs/readme.md": object("docs/readme.md", "hello", "text/markdown", "etag123"),
	}}

	t.Run("serves content with etag and content type", func(t *testing.T) {
		w := serve(t, storeOpts(svc), httptest.NewRequest(http.MethodGet, "/docs/readme.md", nil))

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "hello", w.Body.String())
		assert.Equal(t, `"etag123"`, w.Header().Get("ETag"))
		assert.Equal(t, "text/markdown", w.Header().Get("Content-Type"))
		assert.Equal(t, modTime.Format(http.TimeFormat), w.Header().Get("Last-Modified"))
	})

	t.Run("reports a missing object as 404", func(t *testing.T) {
		w := serve(t, storeOpts(svc), httptest.NewRequest(http.MethodGet, "/nope.txt", nil))

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "not_found", decodeError(t, w).Code)
	})

	t.Run("reports a path the service rejects as 400", func(t *testing.T) {
		svc := &stubService{err: service.ErrInvalidInput}

		w := serve(t, storeOpts(svc), httptest.NewRequest(http.MethodGet, "/bad%5Cpath", nil))

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "invalid_path", decodeError(t, w).Code)
	})

	t.Run("reports an unexpected failure as 500", func(t *testing.T) {
		w := serve(t, storeOpts(&stubService{err: errBoom}), httptest.NewRequest(http.MethodGet, "/a.txt", nil))

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, "internal_error", decodeError(t, w).Code)
	})
}

func TestHandleGetConditional(t *testing.T) {
	t.Parallel()

	svc := &stubService{objects: map[string]stubObject{
		"a.txt": object("a.txt", "hello world", "text/plain", "etag123"),
	}}

	tests := []struct {
		name   string
		header map[string]string
		want   int
	}{
		{name: "matching if-none-match", header: map[string]string{"If-None-Match": `"etag123"`}, want: http.StatusNotModified},
		{name: "wildcard if-none-match", header: map[string]string{"If-None-Match": "*"}, want: http.StatusNotModified},
		{name: "other if-none-match", header: map[string]string{"If-None-Match": `"other"`}, want: http.StatusOK},
		{name: "failing if-match", header: map[string]string{"If-Match": `"other"`}, want: http.StatusPreconditionFailed},
		{name: "matching if-match", header: map[string]string{"If-Match": `"etag123"`}, want: http.StatusOK},
		{
			name:   "if-modified-since after modtime",
			header: map[string]string{"If-Modified-Since": modTime.Add(time.Hour).Format(http.TimeFormat)},
			want:   http.StatusNotModified,
		},
		{
			name:   "if-modified-since before modtime",
			header: map[string]string{"If-Modified-Since": modTime.Add(-time.Hour).Format(http.TimeFormat)},
			want:   http.StatusOK,
		},
		{
			name:   "if-unmodified-since before modtime",
			header: map[string]string{"If-Unmodified-Since": modTime.Add(-time.Hour).Format(http.TimeFormat)},
			want:   http.StatusPreconditionFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, method := range []string{http.MethodGet, http.MethodHead} {
				r := httptest.NewRequest(method, "/a.txt", nil)
				for k, v := range tt.header {
					r.Header.Set(k, v)
				}

				w := serve(t, storeOpts(svc), r)
				assert.Equal(t, tt.want, w.Code, "%s should answer the same as the other method", method)
			}
		})
	}
}

func TestHandleGetRange(t *testing.T) {
	t.Parallel()

	svc := &stubService{objects: map[string]stubObject{
		"a.txt": object("a.txt", "hello world", "text/plain", "etag123"),
	}}

	r := httptest.NewRequest(http.MethodGet, "/a.txt", nil)
	r.Header.Set("Range", "bytes=0-4")

	w := serve(t, storeOpts(svc), r)

	assert.Equal(t, http.StatusPartialContent, w.Code)
	assert.Equal(t, "hello", w.Body.String())
	assert.Equal(t, "bytes 0-4/11", w.Header().Get("Content-Range"))
}

func TestHandleHead(t *testing.T) {
	t.Parallel()

	svc := &stubService{objects: map[string]stubObject{
		"a.txt": object("a.txt", "hello world", "text/plain", "etag123"),
	}}

	t.Run("answers with metadata and no body", func(t *testing.T) {
		w := serve(t, storeOpts(svc), httptest.NewRequest(http.MethodHead, "/a.txt", nil))

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, `"etag123"`, w.Header().Get("ETag"))
		assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
		assert.Equal(t, "11", w.Header().Get("Content-Length"))
		assert.Equal(t, "bytes", w.Header().Get("Accept-Ranges"))
		assert.Equal(t, modTime.Format(http.TimeFormat), w.Header().Get("Last-Modified"))
	})

	t.Run("reports a missing object as 404", func(t *testing.T) {
		w := serve(t, storeOpts(svc), httptest.NewRequest(http.MethodHead, "/nope.txt", nil))

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("serves a range request", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodHead, "/a.txt", nil)
		r.Header.Set("Range", "bytes=0-4")

		w := serve(t, storeOpts(svc), r)

		assert.Equal(t, http.StatusPartialContent, w.Code)
		assert.Equal(t, "bytes 0-4/11", w.Header().Get("Content-Range"))
	})

	t.Run("does not open the object", func(t *testing.T) {
		counted := &stubService{objects: map[string]stubObject{
			"a.txt": object("a.txt", "hello world", "text/plain", "etag123"),
		}}

		w := serve(t, storeOpts(counted), httptest.NewRequest(http.MethodHead, "/a.txt", nil))

		require.Equal(t, http.StatusOK, w.Code)
		assert.Zero(t, counted.opens, "a HEAD is answered from metadata alone")
		assert.Equal(t, "11", w.Header().Get("Content-Length"), "the size still comes back")
	})

	// httptest.ResponseRecorder does not strip a HEAD body; a real server does.
	t.Run("sends no body but the full content length", func(t *testing.T) {
		mux := http.NewServeMux()
		handler.Register(&handler.Opts{
			Mode:   types.ModeStore,
			Svc:    svc,
			Mux:    mux,
			Logger: slog.New(slog.DiscardHandler),
		})

		srv := httptest.NewServer(mux)
		defer srv.Close()

		resp, err := srv.Client().Head(srv.URL + "/a.txt")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Empty(t, body)
		assert.Equal(t, "11", resp.Header.Get("Content-Length"))
		assert.Equal(t, `"etag123"`, resp.Header.Get("ETag"))
	})
}

func TestHandleList(t *testing.T) {
	t.Parallel()

	listed := types.ListResult{
		Items:      []types.MetaData{{Path: "a.txt"}, {Path: "b.txt"}},
		NextCursor: "next",
	}

	t.Run("returns the page as JSON", func(t *testing.T) {
		svc := &stubService{listed: listed}

		w := serve(t, storeOpts(svc), httptest.NewRequest(http.MethodGet, "/", nil))

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var got types.ListResult
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
		assert.Equal(t, listed.NextCursor, got.NextCursor)
		assert.Len(t, got.Items, 2)
	})

	t.Run("passes prefix and cursor through", func(t *testing.T) {
		svc := &stubService{listed: listed}

		serve(t, storeOpts(svc), httptest.NewRequest(http.MethodGet, "/?prefix=images/&cursor=abc", nil))

		assert.Equal(t, "images/", svc.gotQuery.PathPrefix)
		assert.Equal(t, "abc", svc.gotQuery.Cursor)
	})

	t.Run("clamps the limit", func(t *testing.T) {
		tests := []struct {
			query string
			want  int
		}{
			{query: "", want: 100},
			{query: "?limit=50", want: 50},
			{query: "?limit=5000", want: 1000},
			{query: "?limit=0", want: 1},
			{query: "?limit=-10", want: 1},
		}

		for _, tt := range tests {
			svc := &stubService{listed: listed}

			serve(t, storeOpts(svc), httptest.NewRequest(http.MethodGet, "/"+tt.query, nil))

			assert.Equal(t, tt.want, svc.gotQuery.Limit, "limit for %q", tt.query)
		}
	})

	t.Run("rejects a non-numeric limit", func(t *testing.T) {
		w := serve(t, storeOpts(&stubService{}), httptest.NewRequest(http.MethodGet, "/?limit=abc", nil))

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "invalid_parameter", decodeError(t, w).Code)
	})

	t.Run("reports an unexpected failure as 500", func(t *testing.T) {
		w := serve(t, storeOpts(&stubService{err: errBoom}), httptest.NewRequest(http.MethodGet, "/", nil))

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("answers a HEAD of the root", func(t *testing.T) {
		w := serve(t, storeOpts(&stubService{listed: listed}), httptest.NewRequest(http.MethodHead, "/", nil))

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandlePut(t *testing.T) {
	t.Parallel()

	put := func(t *testing.T, svc handler.Service, path, body string, header map[string]string, maxUpload int64) *httptest.ResponseRecorder {
		t.Helper()

		r := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
		for k, v := range header {
			r.Header.Set(k, v)
		}

		opts := storeOpts(svc)
		opts.MaxUploadSize = maxUpload

		return serve(t, opts, r)
	}

	t.Run("stores the body", func(t *testing.T) {
		svc := &stubService{}

		w := put(t, svc, "/docs/a.txt", "hello", map[string]string{"Content-Type": "text/plain"}, 0)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "docs/a.txt", svc.gotCreate.Path)
		assert.Equal(t, "text/plain", svc.gotCreate.ContentType)
		assert.Equal(t, "hello", string(svc.gotBody))
	})

	t.Run("reports a path the service rejects as 400", func(t *testing.T) {
		w := put(t, &stubService{err: service.ErrInvalidInput}, "/bad%5Cpath", "x", nil, 0)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "invalid_path", decodeError(t, w).Code)
	})

	t.Run("rejects a body over the upload limit", func(t *testing.T) {
		w := put(t, &stubService{}, "/a.txt", strings.Repeat("x", 20), nil, 10)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("accepts a body within the upload limit", func(t *testing.T) {
		svc := &stubService{}

		w := put(t, svc, "/a.txt", "hello", nil, 1024)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "hello", string(svc.gotBody))
	})

	t.Run("reports an unexpected failure as 500", func(t *testing.T) {
		w := put(t, &stubService{err: errBoom}, "/a.txt", "x", nil, 0)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("if-match", func(t *testing.T) {
		existing := map[string]stubObject{"a.txt": object("a.txt", "old", "text/plain", "etag123")}

		tests := []struct {
			name    string
			objects map[string]stubObject
			ifMatch string
			want    int
		}{
			{name: "quoted match", objects: existing, ifMatch: `"etag123"`, want: http.StatusOK},
			{name: "unquoted match", objects: existing, ifMatch: "etag123", want: http.StatusOK},
			{name: "wildcard", objects: existing, ifMatch: "*", want: http.StatusOK},
			{name: "one of a list matches", objects: existing, ifMatch: `"other", "etag123"`, want: http.StatusOK},
			{name: "mismatch", objects: existing, ifMatch: `"other"`, want: http.StatusPreconditionFailed},
			{name: "weak tag never matches", objects: existing, ifMatch: `W/"etag123"`, want: http.StatusPreconditionFailed},
			{name: "object does not exist", objects: nil, ifMatch: `"etag123"`, want: http.StatusPreconditionFailed},
			{name: "wildcard on a missing object", objects: nil, ifMatch: "*", want: http.StatusPreconditionFailed},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				svc := &stubService{objects: tt.objects}

				w := put(t, svc, "/a.txt", "new", map[string]string{"If-Match": tt.ifMatch}, 0)

				assert.Equal(t, tt.want, w.Code)
			})
		}
	})

	t.Run("reports a failed if-match lookup as 500", func(t *testing.T) {
		w := put(t, &stubService{err: errBoom}, "/a.txt", "x", map[string]string{"If-Match": `"etag123"`}, 0)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestHandleDelete(t *testing.T) {
	t.Parallel()

	t.Run("removes the object", func(t *testing.T) {
		svc := &stubService{objects: map[string]stubObject{
			"docs/a.txt": object("docs/a.txt", "x", "text/plain", "e"),
		}}

		w := serve(t, storeOpts(svc), httptest.NewRequest(http.MethodDelete, "/docs/a.txt", nil))

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, w.Body.String())
		assert.Equal(t, []string{"docs/a.txt"}, svc.deleted)
	})

	t.Run("reports a missing object as 404", func(t *testing.T) {
		w := serve(t, storeOpts(&stubService{}), httptest.NewRequest(http.MethodDelete, "/nope.txt", nil))

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "not_found", decodeError(t, w).Code)
	})

	t.Run("reports an empty path as 400", func(t *testing.T) {
		w := serve(t, storeOpts(&stubService{err: service.ErrInvalidInput}), httptest.NewRequest(http.MethodDelete, "/", nil))

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("reports an unexpected failure as 500", func(t *testing.T) {
		w := serve(t, storeOpts(&stubService{err: errBoom}), httptest.NewRequest(http.MethodDelete, "/a.txt", nil))

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestRegisterRoutesByMode(t *testing.T) {
	t.Parallel()

	index := map[string]stubObject{
		"index.html": object("index.html", "<html>index</html>", "text/html", "idx"),
	}

	t.Run("store mode lists at the root", func(t *testing.T) {
		w := serve(t, storeOpts(&stubService{}), httptest.NewRequest(http.MethodGet, "/", nil))

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	})

	for _, mode := range []types.ServerMode{types.ModeStatic, types.ModeSPA} {
		t.Run(string(mode)+" mode serves the root through get", func(t *testing.T) {
			opts := &handler.Opts{Mode: mode, Svc: &stubService{objects: index}}

			w := serve(t, opts, httptest.NewRequest(http.MethodGet, "/", nil))

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "<html>index</html>", w.Body.String())
		})

		for _, method := range []string{http.MethodPut, http.MethodDelete} {
			t.Run(string(mode)+" mode rejects "+method, func(t *testing.T) {
				opts := &handler.Opts{Mode: mode, Svc: &stubService{objects: index}}

				w := serve(t, opts, httptest.NewRequest(method, "/a.txt", strings.NewReader("x")))

				assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
			})
		}
	}
}

func TestRegisterMiddleware(t *testing.T) {
	t.Parallel()

	var order []string

	tag := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	opts := storeOpts(&stubService{})
	opts.Middleware = []func(http.Handler) http.Handler{tag("outer"), tag("inner")}

	serve(t, opts, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, []string{"outer", "inner"}, order, "Middleware[0] runs outermost")
}

func TestRegisterAuth(t *testing.T) {
	t.Parallel()

	svc := &stubService{objects: map[string]stubObject{"a.txt": object("a.txt", "x", "text/plain", "e")}}

	reject := verifierFunc(func(*http.Request) error { return errors.New("nope") })

	t.Run("read verifier guards GET and HEAD", func(t *testing.T) {
		opts := storeOpts(svc)
		opts.ReadVerifier = reject

		for _, method := range []string{http.MethodGet, http.MethodHead} {
			w := serve(t, opts, httptest.NewRequest(method, "/a.txt", nil))
			assert.Equal(t, http.StatusUnauthorized, w.Code, method)
		}
	})

	t.Run("write verifier guards PUT and DELETE but not reads", func(t *testing.T) {
		opts := storeOpts(svc)
		opts.WriteVerifier = reject

		for _, method := range []string{http.MethodPut, http.MethodDelete} {
			w := serve(t, opts, httptest.NewRequest(method, "/a.txt", strings.NewReader("x")))
			assert.Equal(t, http.StatusUnauthorized, w.Code, method)
		}

		w := serve(t, opts, httptest.NewRequest(http.MethodGet, "/a.txt", nil))
		assert.Equal(t, http.StatusOK, w.Code, "a write verifier must not guard reads")
	})
}

type verifierFunc func(*http.Request) error

func (f verifierFunc) Verify(r *http.Request) error { return f(r) }

func TestNotFoundHandler(t *testing.T) {
	t.Parallel()

	errorDoc := object("404.html", "<html>custom</html>", "text/html", "e404")

	t.Run("store mode answers with JSON", func(t *testing.T) {
		w := serve(t, storeOpts(&stubService{}), httptest.NewRequest(http.MethodGet, "/nope.txt", nil))

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "not_found", decodeError(t, w).Code)
	})

	t.Run("static mode serves the built-in page", func(t *testing.T) {
		opts := &handler.Opts{Mode: types.ModeStatic, Svc: &stubService{}}

		w := serve(t, opts, httptest.NewRequest(http.MethodGet, "/nope.txt", nil))

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Body.String(), "404 Not Found")
	})

	t.Run("static mode serves the custom error document", func(t *testing.T) {
		opts := &handler.Opts{
			Mode:          types.ModeStatic,
			ErrorDocument: "404.html",
			Svc:           &stubService{objects: map[string]stubObject{"404.html": errorDoc}},
		}

		w := serve(t, opts, httptest.NewRequest(http.MethodGet, "/nope.txt", nil))

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Equal(t, "text/html", w.Header().Get("Content-Type"))
		assert.Equal(t, "<html>custom</html>", w.Body.String())
	})

	t.Run("falls back when the custom error document is missing", func(t *testing.T) {
		opts := &handler.Opts{Mode: types.ModeStatic, ErrorDocument: "404.html", Svc: &stubService{}}

		w := serve(t, opts, httptest.NewRequest(http.MethodGet, "/nope.txt", nil))

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "404 Not Found")
	})

	t.Run("logs and falls back when the error document cannot be read", func(t *testing.T) {
		var logs bytes.Buffer
		opts := &handler.Opts{
			Mode:          types.ModeStatic,
			ErrorDocument: "404.html",
			Svc:           &stubService{errFor: map[string]error{"404.html": errBoom}},
			Mux:           http.NewServeMux(),
			Logger:        slog.New(slog.NewTextHandler(&logs, nil)),
		}
		handler.Register(opts)

		w := httptest.NewRecorder()
		opts.Mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/nope.txt", nil))

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "404 Not Found")
		assert.Contains(t, logs.String(), "serve custom error document")
	})
}
