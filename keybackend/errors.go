package keybackend

import "errors"

// ErrKeyNotFound is returned when the access key does not exist in the store.
var ErrKeyNotFound = errors.New("access key not found")
