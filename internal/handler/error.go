package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/sagarc03/stowry/internal/response"
	"github.com/sagarc03/stowry/internal/service"
)

// HandleError writes the response for err: 404 for a missing object, 400 for
// input the service rejected, 500 otherwise.
func HandleError(logger *slog.Logger, w http.ResponseWriter, err error) {
	logger.Error("request error", "error", err)

	switch {
	case errors.Is(err, service.ErrNotFound):
		_ = response.Error(w, http.StatusNotFound, "not_found", "Object not found")
	case errors.Is(err, service.ErrInvalidInput):
		_ = response.Error(w, http.StatusBadRequest, "invalid_path", "Invalid path")
	default:
		_ = response.Error(w, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}
