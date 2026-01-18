package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/sagarc03/stowry"
)

// ErrorResponse represents a JSON error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// WriteError writes a JSON error response
func WriteError(w http.ResponseWriter, code int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(ErrorResponse{
		Error:   errCode,
		Message: message,
	}); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}

// HandleError writes appropriate error response based on error type
func HandleError(w http.ResponseWriter, err error) {
	slog.Error("request error", "error", err)

	if errors.Is(err, stowry.ErrNotFound) {
		WriteError(w, http.StatusNotFound, "not_found", "Object not found")
		return
	}

	if errors.Is(err, stowry.ErrInvalidInput) {
		WriteError(w, http.StatusBadRequest, "invalid_path", "Invalid path")
		return
	}

	if errors.Is(err, ErrUnauthorized) {
		WriteError(w, http.StatusForbidden, "unauthorized", err.Error())
		return
	}

	// Default internal error
	WriteError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
}

// WriteJSON writes a JSON response
func WriteJSON(w http.ResponseWriter, code int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	return json.NewEncoder(w).Encode(data)
}
