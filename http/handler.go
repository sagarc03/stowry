package http

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	MaxUploadSize int64 // Maximum upload size in bytes. 0 means no limit.
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
		}
		r.Get("/*", h.handleGet)
		r.Head("/*", h.handleHead)
	})

	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(h.config.WriteVerifier))
		r.Put("/*", h.handlePut)
		r.Delete("/*", h.handleDelete)
	})

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

	if path != "" && !stowry.IsValidPath(path) {
		WriteError(w, http.StatusBadRequest, "invalid_path", "Invalid path")
		return
	}

	obj, content, err := h.service.Get(r.Context(), path)
	if err != nil {
		if errors.Is(err, stowry.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "not_found", "Object not found")
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

	if path != "" && !stowry.IsValidPath(path) {
		WriteError(w, http.StatusBadRequest, "invalid_path", "Invalid path")
		return
	}

	obj, err := h.service.Info(r.Context(), path)
	if err != nil {
		if errors.Is(err, stowry.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "not_found", "Object not found")
		} else {
			HandleError(w, err)
		}
		return
	}

	etag := `"` + obj.Etag + `"`
	modTime := obj.UpdatedAt.UTC()

	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", obj.ContentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", obj.FileSizeBytes))
	w.Header().Set("Last-Modified", modTime.Format(http.TimeFormat))
	w.Header().Set("Accept-Ranges", "bytes")

	// If-None-Match takes precedence per RFC 7232
	if inm := r.Header.Get("If-None-Match"); inm != "" {
		if etagWeakMatch(inm, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	} else if ims := r.Header.Get("If-Modified-Since"); ims != "" {
		if t, err := http.ParseTime(ims); err == nil {
			if !modTime.Truncate(time.Second).After(t.Truncate(time.Second)) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}

	w.WriteHeader(http.StatusOK)
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
