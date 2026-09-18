// Package response provides helpers for writing JSON response
// from HTTP handlers.
package response

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
)

// ErrorBody is the wire shape for an error response. Code is a stable,
// machine-readable identifier; Message is prose for a human.
type ErrorBody struct {
	Code    string `json:"error"`
	Message string `json:"message"`
}

// JSON encodes v as JSON and writes it with the given status code.
func JSON(w http.ResponseWriter, status int, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(b)
	return err
}

// Error writes a JSON error response: {"error": code, "message": message}
func Error(w http.ResponseWriter, status int, code, message string) error {
	return JSON(w, status, ErrorBody{Code: code, Message: message})
}
