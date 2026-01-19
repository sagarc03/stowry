package http

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/sagarc03/stowry"
)

type Service interface {
	Get(ctx context.Context, path string) (stowry.MetaData, io.ReadSeekCloser, error)
	Create(ctx context.Context, obj stowry.CreateObject, content io.Reader) (stowry.MetaData, error)
	Delete(ctx context.Context, path string) error
	List(ctx context.Context, query stowry.ListQuery) (stowry.ListResult, error)
}

type CORSConfig struct {
	Enabled          bool
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

type HandlerConfig struct {
	Mode          stowry.ServerMode
	ReadVerifier  RequestVerifier
	WriteVerifier RequestVerifier
	CORS          CORSConfig
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
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = max(1, min(1000, parsed))
		}
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

func (h *Handler) handlePut(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	if path == "" || !stowry.IsValidPath(path) {
		WriteError(w, http.StatusBadRequest, "invalid_path", "Invalid path")
		return
	}

	contentType := r.Header.Get("Content-Type")

	ifMatch := r.Header.Get("If-Match")
	if ifMatch != "" {
		existing, content, err := h.service.Get(r.Context(), path)
		if err != nil && !errors.Is(err, stowry.ErrNotFound) {
			HandleError(w, err)
			return
		}
		if err == nil {
			_ = content.Close()
			if ifMatch != existing.Etag && ifMatch != `"`+existing.Etag+`"` {
				WriteError(w, http.StatusPreconditionFailed, "precondition_failed", "ETag mismatch")
				return
			}
		}
	}

	obj := stowry.CreateObject{
		Path:        path,
		ContentType: contentType,
	}

	metaData, err := h.service.Create(r.Context(), obj, r.Body)
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
