package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/sagarc03/stowry/internal/response"
	"github.com/sagarc03/stowry/internal/service"
	"github.com/sagarc03/stowry/types"
)

const defaultNotFoundHTML = `<html>
<head><title>404 Not Found</title></head>
<body>
<center><h1>404 Not Found</h1></center>
<hr><center>stowry</center>
</body>
</html>`

// NotFoundHandler serves the 404 for the server mode: JSON in store mode, the
// custom error document if one is configured, the built-in page otherwise.
func NotFoundHandler(logger *slog.Logger, svc Service, mode types.ServerMode, errorDocument string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if mode == types.ModeStore {
			_ = response.Error(w, http.StatusNotFound, "not_found", "Object not found")
			return
		}

		// A missing error document falls through to the built-in page; any other
		// failure is logged rather than hidden behind the 404.
		if errorDocument != "" {
			obj, content, err := svc.Get(r.Context(), errorDocument)
			switch {
			case err == nil:
				defer func() { _ = content.Close() }()
				w.Header().Set("Content-Type", obj.ContentType)
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.Copy(w, content)
				return
			case !errors.Is(err, service.ErrNotFound):
				logger.Error("serve custom error document",
					"error", err, "error_document", errorDocument, "path", r.URL.Path)
			}
		}

		writeDefaultNotFound(w)
	}
}

func writeDefaultNotFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = io.WriteString(w, defaultNotFoundHTML)
}
