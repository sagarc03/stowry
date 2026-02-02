package clientcli

import "errors"

// Errors for profile operations.
var (
	ErrProfileNotFound = errors.New("profile not found")
	ErrNoProfiles      = errors.New("no profiles configured")
	ErrProfileExists   = errors.New("profile already exists")
)

// Errors for configuration validation.
var (
	ErrAccessKeyRequired = errors.New("access key is required")
	ErrSecretKeyRequired = errors.New("secret key is required")
	ErrConfigRequired    = errors.New("config is required")
)

// Errors for input validation.
var (
	ErrNoPaths   = errors.New("no paths provided")
	ErrEmptyPath = errors.New("path is required")
)
