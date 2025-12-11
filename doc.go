// Package stowry provides a lightweight object storage library with pluggable
// metadata backends and AWS Signature V4 authentication.
//
// Stowry implements core object storage operations (create, get, delete, list)
// with support for soft deletion, atomic writes, and ETag-based integrity checks.
//
// # Key Components
//
//   - StowryService: Main service combining metadata repository and file storage
//   - MetaDataRepo: Interface for metadata persistence (PostgreSQL, SQLite)
//   - FileStorage: Interface for file operations (filesystem, extensible to S3/GCS)
//   - SignatureVerifier: AWS Signature V4 presigned URL verification
//
// # Server Modes
//
// The library supports three server modes for different use cases:
//
//   - ModeStore: Object storage API returning exact paths or 404
//   - ModeStatic: Static file server with index.html fallback for directories
//   - ModeSPA: Single Page Application mode returning /index.html for 404s
//
// # Example Usage
//
//	service, err := stowry.NewStowryService(repo, storage, stowry.ModeStore)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create an object
//	metadata, err := service.Create(ctx, "path/to/file.txt", contentType, reader)
//
//	// Get an object
//	obj, err := service.Get(ctx, "path/to/file.txt")
//
// See the http package for REST API implementation and the postgres/sqlite
// packages for metadata backend implementations.
package stowry
