package response

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONWrites(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := Error(rec, http.StatusNotFound, "not_found", "Object not found"); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusNotFound || rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("code=%d ct=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != `{"error":"not_found","message":"Object not found"}` {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestJSONReturnsErrorWithoutWriting(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := JSON(rec, http.StatusOK, make(chan int)); err == nil {
		t.Fatal("expected marshal error")
	}
	if rec.Header().Get("Content-Type") != "" || rec.Body.Len() != 0 {
		t.Errorf("headers/body written before marshal succeeded: %v %q", rec.Header(), rec.Body.String())
	}
}
