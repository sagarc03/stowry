package http

import "errors"

// ErrUnauthorized is returned when authentication fails.
var ErrUnauthorized = errors.New("unauthorized")
