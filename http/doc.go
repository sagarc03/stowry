// Package http provides HTTP server functionality for Stowry object storage.
//
// This package implements a RESTful API for object storage with signature-based
// authentication, supporting both AWS Signature V4 and Stowry native signing.
//
// # Features
//
//   - AWS Signature V4 authentication (HMAC-SHA256)
//   - Stowry native signing (lightweight alternative)
//   - Pluggable key backends via SecretStore interface
//   - ETag-based conditional requests
//   - Three server modes: Store (API), Static (static website), SPA (single page app)
//   - Path traversal protection
//   - JSON error responses
//   - Configurable CORS support
//
// # Server Modes
//
// Store Mode: Full object storage API with GET, PUT, DELETE operations and listing.
//
// Static Mode: Serves static websites with automatic index.html fallback for directories.
//
// SPA Mode: Single Page Application mode that returns index.html for 404s to support
// client-side routing.
//
// # Authentication
//
// The package uses RequestVerifier interface for authentication. Pass a verifier
// to AuthMiddleware, or nil for public access:
//
//	// Create a secret store and verifier
//	store := keybackend.NewMapSecretStore(map[string]string{
//	    "AKIAIOSFODNN7EXAMPLE": "wJalrXUt...",
//	})
//	verifier := stowry.NewSignatureVerifier("us-east-1", "s3", store)
//
//	// Apply middleware (nil = public access)
//	router.Use(http.AuthMiddleware(verifier))
//
// # Usage
//
// Create a handler with HandlerConfig:
//
//	store := keybackend.NewMapSecretStore(accessKeys)
//	verifier := stowry.NewSignatureVerifier("us-east-1", "s3", store)
//
//	handlerCfg := http.HandlerConfig{
//	    Mode:          stowry.ModeStore,
//	    ReadVerifier:  verifier,  // nil for public read
//	    WriteVerifier: verifier,  // nil for public write
//	}
//	handler := http.NewHandler(&handlerCfg, service)
//	router := handler.Router()
//	http.ListenAndServe(":8080", router)
//
// The service parameter must implement the Service interface with Get, Create,
// Delete, and List methods.
//
// # Middleware
//
// The package provides two middleware functions:
//
// AuthMiddleware - Signature verification (AWS V4 or Stowry native):
//
//	router.Use(http.AuthMiddleware(verifier))  // authenticated
//	router.Use(http.AuthMiddleware(nil))       // public access
//
// PathValidationMiddleware - Path traversal protection:
//
//	router.Use(http.PathValidationMiddleware)
//
// Both middleware are automatically applied by Handler.Router().
package http
