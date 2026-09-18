package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func serve(t *testing.T, h http.Handler) (*httptest.ResponseRecorder, []map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	req := httptest.NewRequest(http.MethodGet, "/things", nil)
	req = req.WithContext(ContextWithRequestID(req.Context(), "rid-1"))
	rec := httptest.NewRecorder()
	WithLogging(logger, h).ServeHTTP(rec, req)

	var logs []map[string]any
	dec := json.NewDecoder(&buf)
	for dec.More() {
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			t.Fatalf("bad log line: %v", err)
		}
		logs = append(logs, m)
	}
	return rec, logs
}

func TestWithLoggingRecordsRequest(t *testing.T) {
	rec, logs := serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hello"))
	}))

	if rec.Code != http.StatusCreated || rec.Body.String() != "hello" {
		t.Fatalf("response not passed through: %d %q", rec.Code, rec.Body.String())
	}
	if len(logs) != 1 {
		t.Fatalf("want 1 log record, got %d: %v", len(logs), logs)
	}
	l := logs[0]
	want := map[string]any{
		"level": "INFO", "msg": "request",
		"method": "GET", "path": "/things", "status": float64(201), "bytes": float64(5),
	}
	for k, v := range want {
		if l[k] != v {
			t.Errorf("%s = %v, want %v", k, l[k], v)
		}
	}
	if _, ok := l["duration"]; !ok {
		t.Error("missing duration")
	}
}

func TestWithLoggingDefaultsTo200OnImplicitWrite(t *testing.T) {
	_, logs := serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	if logs[0]["status"] != float64(200) {
		t.Errorf("status = %v, want 200", logs[0]["status"])
	}
}

func TestWithLogging5xxLogsError(t *testing.T) {
	_, logs := serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	if logs[0]["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", logs[0]["level"])
	}
}

func TestWithLoggingRecoversPanic(t *testing.T) {
	rec, logs := serve(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"error":"internal server error"`)) {
		t.Errorf("body = %q", rec.Body.String())
	}
	if len(logs) != 2 {
		t.Fatalf("want panic + request records, got %d: %v", len(logs), logs)
	}
	p, r := logs[0], logs[1]
	if p["msg"] != "panic recovered" || p["level"] != "ERROR" {
		t.Errorf("panic record: %v", p)
	}
	if s, _ := p["stack"].(string); s == "" {
		t.Error("panic record missing stack")
	}
	if r["status"] != float64(500) || r["level"] != "ERROR" {
		t.Errorf("request record: %v", r)
	}
}

func TestWithLoggingPanicAfterHeaderDoesNotRewrite(t *testing.T) {
	rec, _ := serve(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		panic("boom")
	}))
	if rec.Code != http.StatusAccepted {
		t.Errorf("code = %d, want 202 (header already sent)", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body should be untouched, got %q", rec.Body.String())
	}
}

func TestWithLoggingRethrowsErrAbortHandler(t *testing.T) {
	defer func() {
		p := recover()
		if err, ok := p.(error); !ok || !errors.Is(err, http.ErrAbortHandler) {
			t.Fatalf("recovered %v, want http.ErrAbortHandler", p)
		}
	}()
	serve(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))
}

func TestResponseRecorderIgnoresSecondWriteHeader(t *testing.T) {
	rr := &responseRecorder{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}
	rr.WriteHeader(http.StatusNotFound)
	rr.WriteHeader(http.StatusTeapot)
	if rr.status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.status)
	}
}
