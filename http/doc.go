// Package http provides HTTP server functionality for Stowry object storage.
//
// This package implements a RESTful API for object storage with AWS Signature V4
// authentication, supporting both public and authenticated access modes.
//
// # Features
//
//   - AWS Signature V4 authentication (HMAC-SHA256)
//   - Multiple access key support
//   - Public/private read and write modes
//   - ETag-based conditional requests
//   - Three server modes: Store (API), Static (static website), SPA (single page app)
//   - Path traversal protection
//   - JSON error responses
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
// The package uses AWS Signature V4 authentication for request verification.
// Authentication is handled by the AuthMiddleware with a simple configuration:
//
//	authCfg := http.AuthMiddlewareConfig{
//	    AuthRequired: true,
//	    Region:       "us-east-1",
//	    Service:      "s3",
//	    AccessKeys: map[string]string{
//	        "AKIAIOSFODNN7EXAMPLE": "wJalrXUt...",
//	    },
//	}
//	router.Use(http.AuthMiddleware(authCfg))
//
// For public access, set AuthRequired to false or pass nil for AccessKeys.
//
// # Usage
//
// Create a handler with HandlerConfig:
//
//	handlerCfg := http.HandlerConfig{
//	    PublicRead:  false,
//	    PublicWrite: false,
//	    Mode:        stowry.ModeStore,
//	    Region:      "us-east-1",
//	    Service:     "s3",
//	    AccessKeys: map[string]string{
//	        "AKIAIOSFODNN7EXAMPLE": "wJalrXUt...",
//	    },
//	}
//	handler := http.NewHandler(handlerCfg, service)
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
// AuthMiddleware - AWS Signature V4 verification:
//
//	cfg := http.AuthMiddlewareConfig{
//	    AuthRequired: true,
//	    Region:       "us-east-1",
//	    Service:      "s3",
//	    AccessKeys:   accessKeys,
//	}
//	router.Use(http.AuthMiddleware(cfg))
//
// PathValidationMiddleware - Path traversal protection:
//
//	router.Use(http.PathValidationMiddleware)
//
// Both middleware are automatically applied by Handler.Router().
package http
