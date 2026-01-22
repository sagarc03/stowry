package http

import (
	"bytes"
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
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(ErrorResponse{
		Error:   errCode,
		Message: message,
	}); err != nil {
		slog.Error("failed to encode error response", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(buf.Bytes())
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
		WriteError(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}

	// Default internal error
	WriteError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
}

// WriteJSON writes a JSON response
func WriteJSON(w http.ResponseWriter, code int, data any) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(buf.Bytes())
	return nil
}
