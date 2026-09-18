// Package response provides helpers for writing JSON response
// from HTTP handlers.
package response

import (
	"encoding/json/v2"
	"fmt"
	"net/http"

	"github.com/sagarc03/stowry/types"
)

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
	return JSON(w, status, types.ErrorBody{Code: code, Message: message})
}
