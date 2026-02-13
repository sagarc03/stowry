package http

import (
	"io"
	"net/http"
)

const defaultNotFoundHTML = `<html>
<head><title>404 Not Found</title></head>
<body>
<center><h1>404 Not Found</h1></center>
<hr><center>stowry</center>
</body>
</html>`

func writeDefaultNotFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = io.WriteString(w, defaultNotFoundHTML)
}
