// Package handler provides the HTTP handlers for the object service.
package handler

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/sagarc03/stowry/internal/middleware"
	"github.com/sagarc03/stowry/internal/response"
	"github.com/sagarc03/stowry/internal/service"
	"github.com/sagarc03/stowry/types"
)

// Service is the part of the object service these handlers use.
// *service.Service satisfies it implicitly and validates its own paths.
type Service interface {
	Get(ctx context.Context, path string) (types.MetaData, io.ReadSeekCloser, error)
	Info(ctx context.Context, path string) (types.MetaData, error)
	Create(ctx context.Context, obj types.CreateObject, content io.Reader) (types.MetaData, error)
	Delete(ctx context.Context, path string) error
	List(ctx context.Context, q types.ListQuery) (types.ListResult, error)
}

const (
	defaultListLimit = 100
	maxListLimit     = 1000
)

// Opts configures Register.
type Opts struct {
	Mode          types.ServerMode
	ErrorDocument string
	// MaxUploadSize caps a PUT body in bytes. Zero means no limit.
	MaxUploadSize int64
	Mux           *http.ServeMux
	Svc           Service
	Logger        *slog.Logger
	// ReadVerifier authenticates GET and HEAD, WriteVerifier PUT and DELETE.
	// A nil verifier leaves those methods unauthenticated.
	ReadVerifier  middleware.RequestVerifier
	WriteVerifier middleware.RequestVerifier
	// Middleware wraps every route, outermost first.
	Middleware []func(next http.Handler) http.Handler
}

// Register installs the routes on opts.Mux. Only store mode registers the root
// listing and the write methods; ServeMux answers the rest with 405.
func Register(opts *Opts) {
	logger := cmp.Or(opts.Logger, slog.Default())

	readAuth := middleware.AuthMiddleware(opts.ReadVerifier)
	writeAuth := middleware.AuthMiddleware(opts.WriteVerifier)

	wrap := func(auth func(http.Handler) http.Handler, h http.HandlerFunc) http.Handler {
		wrapped := auth(h)
		for _, m := range slices.Backward(opts.Middleware) {
			wrapped = m(wrapped)
		}
		return wrapped
	}

	notFound := NotFoundHandler(logger, opts.Svc, opts.Mode, opts.ErrorDocument)

	if opts.Mode == types.ModeStore {
		opts.Mux.Handle("GET /{$}", wrap(readAuth, HandleList(logger, opts.Svc)))
		opts.Mux.Handle("PUT /", wrap(writeAuth, HandlePut(logger, opts.Svc, opts.MaxUploadSize)))
		opts.Mux.Handle("DELETE /", wrap(writeAuth, HandleDelete(logger, opts.Svc)))
	}

	read := byMethod(HandleGet(logger, opts.Svc, notFound), HandleHead(logger, opts.Svc, notFound))
	opts.Mux.Handle("GET /", wrap(readAuth, read))
}

// byMethod sends HEAD to head and everything else to get. ServeMux cannot do
// this: a GET pattern also matches HEAD, so "HEAD /" overlaps "GET /{$}" with
// neither more specific, which it rejects as a conflict.
func byMethod(get, head http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			head(w, r)
			return
		}

		get(w, r)
	}
}

// HandleList serves a page of object metadata, paged by the prefix, limit and
// cursor query parameters.
func HandleList(logger *slog.Logger, svc Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		limit := defaultListLimit
		if raw := query.Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				_ = response.Error(w, http.StatusBadRequest, "invalid_parameter", "limit must be a valid integer")
				return
			}
			limit = min(max(parsed, 1), maxListLimit)
		}

		result, err := svc.List(r.Context(), types.ListQuery{
			PathPrefix: query.Get("prefix"),
			Limit:      limit,
			Cursor:     query.Get("cursor"),
		})
		if err != nil {
			HandleError(logger, w, err)
			return
		}

		_ = response.JSON(w, http.StatusOK, result)
	}
}

// HandleGet serves an object's content. ServeContent handles Last-Modified,
// Content-Length, ranges and the conditional headers.
func HandleGet(logger *slog.Logger, svc Service, notFound http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := requestPath(r)

		obj, content, err := svc.Get(r.Context(), path)
		if err != nil {
			respondReadError(logger, w, r, err, notFound)
			return
		}
		defer func() { _ = content.Close() }()

		w.Header().Set("ETag", `"`+obj.Etag+`"`)
		w.Header().Set("Content-Type", obj.ContentType)

		http.ServeContent(w, r, path, obj.UpdatedAt, content)
	}
}

// HandleHead answers with an object's headers and no body, reading metadata
// only. It hands the response to ServeContent as HandleGet does, so the two
// cannot disagree.
func HandleHead(logger *slog.Logger, svc Service, notFound http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := requestPath(r)

		obj, err := svc.Info(r.Context(), path)
		if err != nil {
			respondReadError(logger, w, r, err, notFound)
			return
		}

		w.Header().Set("ETag", `"`+obj.Etag+`"`)
		w.Header().Set("Content-Type", obj.ContentType)

		http.ServeContent(w, r, path, obj.UpdatedAt, &headContent{size: obj.FileSizeBytes})
	}
}

// headContent reports a size without holding the object. ServeContent needs a
// reader only to measure what it sends, and it sends no body for HEAD.
type headContent struct {
	size   int64
	offset int64
}

func (c *headContent) Read([]byte) (int, error) { return 0, io.EOF }

func (c *headContent) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		c.offset = offset
	case io.SeekCurrent:
		c.offset += offset
	case io.SeekEnd:
		c.offset = c.size + offset
	default:
		return 0, fmt.Errorf("seek: invalid whence %d", whence)
	}

	if c.offset < 0 {
		return 0, errors.New("seek: negative position")
	}

	return c.offset, nil
}

// HandlePut stores the request body at the request path, honouring If-Match.
func HandlePut(logger *slog.Logger, svc Service, maxUploadSize int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := requestPath(r)

		if ifMatch := r.Header.Get("If-Match"); ifMatch != "" {
			existing, err := svc.Info(r.Context(), path)
			switch {
			// RFC 9110 §13.1.1: If-Match fails when nothing is stored.
			case errors.Is(err, service.ErrNotFound):
				_ = response.Error(w, http.StatusPreconditionFailed, "precondition_failed", "ETag mismatch")
				return
			case err != nil:
				HandleError(logger, w, err)
				return
			case !etagStrongMatch(ifMatch, `"`+existing.Etag+`"`):
				_ = response.Error(w, http.StatusPreconditionFailed, "precondition_failed", "ETag mismatch")
				return
			}
		}

		body := io.Reader(r.Body)
		if maxUploadSize > 0 {
			body = http.MaxBytesReader(w, r.Body, maxUploadSize)
		}

		obj := types.CreateObject{Path: path, ContentType: r.Header.Get("Content-Type")}

		metaData, err := svc.Create(r.Context(), obj, body)
		if err != nil {
			HandleError(logger, w, err)
			return
		}

		_ = response.JSON(w, http.StatusOK, metaData)
	}
}

// HandleDelete soft-deletes the object at the request path.
func HandleDelete(logger *slog.Logger, svc Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Delete(r.Context(), requestPath(r)); err != nil {
			HandleError(logger, w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// etagStrongMatch reports whether an If-Match header matches etag under the
// strong comparison of RFC 9110 §8.8.3.2, tolerating an unquoted tag. Only PUT
// needs it; ServeContent evaluates conditional reads itself.
func etagStrongMatch(headerVal, etag string) bool {
	if headerVal == "*" {
		return true
	}
	if strings.HasPrefix(etag, `W/`) {
		return false
	}

	opaqueTag := strings.Trim(etag, `"`)

	for raw := range strings.SplitSeq(headerVal, ",") {
		candidate := strings.TrimSpace(raw)
		if strings.HasPrefix(candidate, `W/`) {
			continue
		}
		if candidate == etag || strings.Trim(candidate, `"`) == opaqueTag {
			return true
		}
	}

	return false
}

func requestPath(r *http.Request) string {
	return strings.TrimPrefix(r.URL.Path, "/")
}

// respondReadError sends a missing object to notFound, anything else to
// HandleError.
func respondReadError(logger *slog.Logger, w http.ResponseWriter, r *http.Request, err error, notFound http.HandlerFunc) {
	if errors.Is(err, service.ErrNotFound) {
		notFound(w, r)
		return
	}

	HandleError(logger, w, err)
}
