package http_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sagarc03/stowry"
	stowryhttp "github.com/sagarc03/stowry/http"
	"github.com/stretchr/testify/assert"
)

func TestHandleError_NotFound(t *testing.T) {
	rec := httptest.NewRecorder()

	stowryhttp.HandleError(rec, stowry.ErrNotFound)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_found")
}

func TestHandleError_InvalidInput(t *testing.T) {
	rec := httptest.NewRecorder()

	stowryhttp.HandleError(rec, stowry.ErrInvalidInput)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_path")
}

func TestHandleError_Unauthorized(t *testing.T) {
	rec := httptest.NewRecorder()

	stowryhttp.HandleError(rec, stowryhttp.ErrUnauthorized)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "unauthorized")
}

func TestHandleError_InternalError(t *testing.T) {
	rec := httptest.NewRecorder()

	stowryhttp.HandleError(rec, errors.New("some unexpected error"))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "internal_error")
}

func TestHandleError_WrappedNotFound(t *testing.T) {
	rec := httptest.NewRecorder()

	wrappedErr := errors.Join(errors.New("context"), stowry.ErrNotFound)
	stowryhttp.HandleError(rec, wrappedErr)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_found")
}

func TestWriteError_Success(t *testing.T) {
	rec := httptest.NewRecorder()

	stowryhttp.WriteError(rec, http.StatusBadRequest, "bad_request", "Invalid request")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), `"error":"bad_request"`)
	assert.Contains(t, rec.Body.String(), `"message":"Invalid request"`)
}

func TestWriteJSON_Success(t *testing.T) {
	rec := httptest.NewRecorder()

	data := map[string]string{"key": "value"}
	err := stowryhttp.WriteJSON(rec, http.StatusOK, data)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), `"key":"value"`)
}

func TestWriteJSON_EncodingError(t *testing.T) {
	rec := httptest.NewRecorder()

	// Channels cannot be JSON encoded
	data := make(chan int)
	err := stowryhttp.WriteJSON(rec, http.StatusOK, data)

	assert.Error(t, err)
}
