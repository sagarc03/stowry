package stowry

import "errors"

var (
	// ErrNotFound is returned when a resource is not found
	ErrNotFound = errors.New("not found")
	// ErrInvalidInput is returned when input validation fails
	ErrInvalidInput = errors.New("invalid input")
)
