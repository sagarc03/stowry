package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/sagarc03/stowry"
)

type Service interface {
	Get(ctx context.Context, path string) (stowry.MetaData, io.ReadSeekCloser, error)
	Info(ctx context.Context, path string) (stowry.MetaData, error)
	Create(ctx context.Context, obj stowry.CreateObject, content io.Reader) (stowry.MetaData, error)
	Delete(ctx context.Context, path string) error
	List(ctx context.Context, query stowry.ListQuery) (stowry.ListResult, error)
}

type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	ExposedHeaders   []string `mapstructure:"exposed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

type HandlerConfig struct {
	Mode          stowry.ServerMode
	ReadVerifier  RequestVerifier
	WriteVerifier RequestVerifier
	CORS          CORSConfig
	MaxUploadSize int64  // Maximum upload size in bytes. 0 means no limit.
	ErrorDocument string // Path to custom error page in storage. Empty uses default.
}

// Handler provides HTTP handlers for object storage operations.
type Handler struct {
	config  HandlerConfig
	service Service
}

// NewHandler creates a new Handler with the given configuration and service.
func NewHandler(config *HandlerConfig, service Service) *Handler {
	return &Handler{
		config:  *config,
		service: service,
	}
}

// Router returns an http.Handler with routes configured based on mode.
// In store mode, GET / returns a list of objects.
// In static/SPA modes, GET / is handled by the get handler (serves index.html via service).
func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()

	if h.config.CORS.Enabled {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   h.config.CORS.AllowedOrigins,
			AllowedMethods:   h.config.CORS.AllowedMethods,
			AllowedHeaders:   h.config.CORS.AllowedHeaders,
			ExposedHeaders:   h.config.CORS.ExposedHeaders,
			AllowCredentials: h.config.CORS.AllowCredentials,
			MaxAge:           h.config.CORS.MaxAge,
		}))
	}

	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(h.config.ReadVerifier))
		if h.config.Mode == stowry.ModeStore {
			r.Get("/", h.handleList)
			r.Head("/", h.handleList)
		}
		r.Get("/*", h.handleGet)
		r.Head("/*", h.handleHead)
	})

	if h.config.Mode == stowry.ModeStore {
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(h.config.WriteVerifier))
			r.Put("/*", h.handlePut)
			r.Delete("/*", h.handleDelete)
		})
	}

	return r
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	limitStr := r.URL.Query().Get("limit")
	cursor := r.URL.Query().Get("cursor")

	limit := 100
	if limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid_parameter", "limit must be a valid integer")
			return
		}
		limit = max(1, min(1000, parsed))
	}

	query := stowry.ListQuery{
		PathPrefix: prefix,
		Limit:      limit,
		Cursor:     cursor,
	}

	result, err := h.service.List(r.Context(), query)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	if path != "" && !h.isValidRequestPath(path) {
		WriteError(w, http.StatusBadRequest, "invalid_path", "Invalid path")
		return
	}

	obj, content, err := h.service.Get(r.Context(), path)
	if err != nil {
		if errors.Is(err, stowry.ErrNotFound) {
			h.handleNotFound(w, r)
		} else {
			HandleError(w, err)
		}
		return
	}
	defer func() { _ = content.Close() }()

	w.Header().Set("ETag", `"`+obj.Etag+`"`)
	w.Header().Set("Content-Type", obj.ContentType)

	http.ServeContent(w, r, path, obj.UpdatedAt, content)
}

func (h *Handler) handleHead(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	if path != "" && !h.isValidRequestPath(path) {
		WriteError(w, http.StatusBadRequest, "invalid_path", "Invalid path")
		return
	}

	// Range requests need the 206/Content-Range handling that handleGet gets
	// for free from http.ServeContent. Serve them through the GET path so the
	// two methods cannot disagree; net/http suppresses the body for HEAD.
	if r.Header.Get("Range") != "" {
		h.handleGet(w, r)
		return
	}

	obj, err := h.service.Info(r.Context(), path)
	if err != nil {
		if errors.Is(err, stowry.ErrNotFound) {
			h.handleNotFound(w, r)
		} else {
			HandleError(w, err)
		}
		return
	}

	etag := `"` + obj.Etag + `"`
	modTime := obj.UpdatedAt.UTC()

	w.Header().Set("ETag", etag)
	if !isZeroTime(modTime) {
		w.Header().Set("Last-Modified", modTime.Format(http.TimeFormat))
	}

	// Evaluated before Content-Length is set: unlike 304, a 412 response is not
	// stripped of entity headers by net/http, and advertising the object size
	// on an empty body would leave the client waiting for content.
	switch checkPreconditions(r, etag, modTime) {
	case preconditionFailed:
		w.WriteHeader(http.StatusPreconditionFailed)
		return
	case preconditionNotModified:
		w.WriteHeader(http.StatusNotModified)
		return
	case preconditionNone:
		// No conditional header applied: serve the metadata below.
	}

	w.Header().Set("Content-Type", obj.ContentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", obj.FileSizeBytes))
	w.Header().Set("Accept-Ranges", "bytes")

	w.WriteHeader(http.StatusOK)
}

// preconditionResult is the outcome of evaluating conditional request headers.
type preconditionResult int

const (
	// preconditionNone means the request should be served normally.
	preconditionNone preconditionResult = iota
	// preconditionFailed means the request must be answered with 412.
	preconditionFailed
	// preconditionNotModified means the request must be answered with 304.
	preconditionNotModified
)

// checkPreconditions evaluates the conditional headers of a GET or HEAD request
// in the order required by RFC 9110 §13.2.2: If-Match, then If-Unmodified-Since
// only when If-Match is absent, then If-None-Match, then If-Modified-Since only
// when If-None-Match is absent.
//
// This mirrors what http.ServeContent applies on the GET path, so that HEAD and
// GET answer an identical conditional request identically (RFC 9110 §9.3.2).
func checkPreconditions(r *http.Request, etag string, modTime time.Time) preconditionResult {
	if im := r.Header.Get("If-Match"); im != "" {
		if !etagStrongMatchRFC(im, etag) {
			return preconditionFailed
		}
	} else if ius := r.Header.Get("If-Unmodified-Since"); ius != "" {
		// A zero modification time carries no information, so ServeContent
		// ignores the date checks entirely rather than comparing against it.
		if t, err := http.ParseTime(ius); err == nil && !isZeroTime(modTime) {
			if modTime.Truncate(time.Second).After(t.Truncate(time.Second)) {
				return preconditionFailed
			}
		}
	}

	if inm := r.Header.Get("If-None-Match"); inm != "" {
		if etagWeakMatch(inm, etag) {
			return preconditionNotModified
		}
	} else if ims := r.Header.Get("If-Modified-Since"); ims != "" {
		if t, err := http.ParseTime(ims); err == nil && !isZeroTime(modTime) {
			if !modTime.Truncate(time.Second).After(t.Truncate(time.Second)) {
				return preconditionNotModified
			}
		}
	}

	return preconditionNone
}

// unixEpochTime is the other value net/http treats as "no modification time".
var unixEpochTime = time.Unix(0, 0)

// isZeroTime reports whether t is one of the values that mean "unknown", using
// the same definition as net/http's isZeroTime.
func isZeroTime(t time.Time) bool {
	return t.IsZero() || t.Equal(unixEpochTime)
}

// etagStrongMatchRFC reports whether headerVal matches etag under the strong
// comparison rules http.ServeContent applies on the GET path: an opaque-tag
// must be quoted (net/http's scanETag rejects anything else outright), and a
// weak tag never participates in a strong comparison.
//
// This is deliberately stricter than etagStrongMatch, which tolerates unquoted
// tags for conditional writes - see TestHandler_HandlePut_IfMatch_Match. Using
// the lenient form here would make HEAD answer an unquoted If-Match with 200
// while GET answered 412.
func etagStrongMatchRFC(headerVal, etag string) bool {
	if headerVal == "*" {
		return true
	}
	if strings.HasPrefix(etag, `W/`) {
		return false
	}

	for _, raw := range strings.Split(headerVal, ",") {
		candidate := strings.TrimSpace(raw)
		if len(candidate) < 2 || !strings.HasPrefix(candidate, `"`) || !strings.HasSuffix(candidate, `"`) {
			continue
		}
		if candidate == etag {
			return true
		}
	}

	return false
}

func (h *Handler) handlePut(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	if path == "" || !stowry.IsValidPath(path) {
		WriteError(w, http.StatusBadRequest, "invalid_path", "Invalid path")
		return
	}

	contentType := r.Header.Get("Content-Type")

	ifMatch := r.Header.Get("If-Match")
	if ifMatch != "" {
		existing, err := h.service.Info(r.Context(), path)
		if err != nil && !errors.Is(err, stowry.ErrNotFound) {
			HandleError(w, err)
			return
		}
		// RFC 9110 §13.1.1: If-Match is false when there is no current representation
		if errors.Is(err, stowry.ErrNotFound) {
			WriteError(w, http.StatusPreconditionFailed, "precondition_failed", "ETag mismatch")
			return
		}
		if !etagStrongMatch(ifMatch, `"`+existing.Etag+`"`) {
			WriteError(w, http.StatusPreconditionFailed, "precondition_failed", "ETag mismatch")
			return
		}
	}

	obj := stowry.CreateObject{
		Path:        path,
		ContentType: contentType,
	}

	body := io.Reader(r.Body)
	if h.config.MaxUploadSize > 0 {
		body = http.MaxBytesReader(w, r.Body, h.config.MaxUploadSize)
	}

	metaData, err := h.service.Create(r.Context(), obj, body)
	if err != nil {
		HandleError(w, err)
		return
	}

	_ = WriteJSON(w, http.StatusOK, metaData)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	if path == "" || !stowry.IsValidPath(path) {
		WriteError(w, http.StatusBadRequest, "invalid_path", "Invalid path")
		return
	}

	err := h.service.Delete(r.Context(), path)
	if err != nil {
		if errors.Is(err, stowry.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "not_found", "Object not found")
		} else {
			HandleError(w, err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// isValidRequestPath validates the request path, allowing trailing slashes in static/SPA modes
// for directory-style URLs (e.g., /docs/).
func (h *Handler) isValidRequestPath(path string) bool {
	// In static/SPA modes, allow trailing slashes for directory index resolution.
	// Validate the path without the trailing slash.
	if h.config.Mode != stowry.ModeStore && strings.HasSuffix(path, "/") {
		trimmed := strings.TrimSuffix(path, "/")
		if trimmed == "" {
			return true
		}
		return stowry.IsValidPath(trimmed)
	}
	return stowry.IsValidPath(path)
}

// handleNotFound serves the appropriate 404 response based on server mode.
// In store mode, returns a JSON error. In static/SPA modes, tries the custom
// error document first, then falls back to a default HTML 404 page.
func (h *Handler) handleNotFound(w http.ResponseWriter, r *http.Request) {
	if h.config.Mode == stowry.ModeStore {
		WriteError(w, http.StatusNotFound, "not_found", "Object not found")
		return
	}

	// Try custom error document if configured. A missing error document falls
	// through to the built-in page, but any other failure (database down,
	// unreadable blob) is logged rather than silently hidden behind a 404.
	if h.config.ErrorDocument != "" {
		obj, content, err := h.service.Get(r.Context(), h.config.ErrorDocument)
		switch {
		case err == nil:
			defer func() { _ = content.Close() }()
			w.Header().Set("Content-Type", obj.ContentType)
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.Copy(w, content)
			return
		case !errors.Is(err, stowry.ErrNotFound):
			slog.Error("failed to serve custom error document",
				"error", err, "error_document", h.config.ErrorDocument, "path", r.URL.Path)
		}
	}

	// Default HTML 404
	writeDefaultNotFound(w)
}

// etagStrongMatch checks if the If-Match header value matches the given ETag
// using strong comparison per RFC 9110 §8.8.3.2.
// Both ETags must not be weak, and their opaque-tags must be identical.
// Handles * and comma-separated lists.
func etagStrongMatch(headerVal, etag string) bool {
	if headerVal == "*" {
		return true
	}
	// Our etag must not be weak for strong comparison
	if strings.HasPrefix(etag, `W/`) {
		return false
	}
	// Extract opaque value without quotes for lenient matching
	opaqueTag := strings.Trim(etag, `"`)

	for _, raw := range strings.Split(headerVal, ",") {
		candidate := strings.TrimSpace(raw)
		// Reject weak ETags in strong comparison
		if strings.HasPrefix(candidate, `W/`) {
			continue
		}
		// Match quoted or bare values
		if candidate == etag || strings.Trim(candidate, `"`) == opaqueTag {
			return true
		}
	}
	return false
}

// etagWeakMatch checks if the If-None-Match header value matches the given ETag
// using weak comparison per RFC 9110 §8.8.3.2.
// Handles *, comma-separated lists, and W/ prefixes.
func etagWeakMatch(headerVal, etag string) bool {
	if headerVal == "*" {
		return true
	}
	// Strip W/ prefix from our etag for comparison
	opaqueTag := strings.TrimPrefix(etag, `W/`)

	for _, raw := range strings.Split(headerVal, ",") {
		candidate := strings.TrimSpace(raw)
		// Strip W/ prefix from candidate for weak comparison
		candidate = strings.TrimPrefix(candidate, `W/`)
		if candidate == opaqueTag {
			return true
		}
	}
	return false
}
