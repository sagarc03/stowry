# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **CORS Middleware** - Configurable CORS support for browser-based uploads
  - Configurable allowed origins, methods, headers
  - Exposed headers for ETag and Content-Length
  - Preflight caching with configurable max age

## [1.1.0] - 2025-01-16

### Added

- **Stowry Native Signing** - Simple, lightweight signing scheme as alternative to AWS Sig V4
- **Client SDKs** - Official SDKs for generating presigned URLs
  - [stowry-go](https://github.com/sagarc03/stowry-go) - Go SDK
  - [stowrypy](https://pypi.org/project/stowrypy/) - Python SDK
  - [stowryjs](https://www.npmjs.com/package/stowryjs) - JavaScript/TypeScript SDK
- **SDK Examples** - Examples for both native and AWS signing in Go, Python, and JavaScript

### Changed

- Server now auto-detects signing scheme from query parameters (X-Stowry-Signature vs X-Amz-Signature)
- Reorganized examples into `aws/` and `stowry/` folders

## [1.0.0] - 2025-01-15

### Added

- **HTTP REST API** - Full REST API with GET/PUT/DELETE operations for objects
- **Object Listing** - Cursor-based pagination with prefix filtering
- **AWS Signature V4 Authentication** - Presigned URL authentication
- **Multiple Access Keys** - Support for multiple access key/secret key pairs
- **Public Access Control** - Configurable public read/write modes
- **Three Server Modes**
  - `store` - Object storage API
  - `static` - Static file server with index.html fallback
  - `spa` - Single Page Application mode with client-side routing support
- **Database Support**
  - SQLite
  - PostgreSQL
- **CLI Commands**
  - `stowry serve` - Start HTTP server
  - `stowry init` - Initialize metadata from existing directory
  - `stowry cleanup` - Clean up soft-deleted files
- **Soft Deletion** - Files marked as deleted until cleanup runs
- **Atomic Writes** - Files written to temp file then renamed
- **ETag/Integrity** - SHA256 hash computed for every file
- **Conditional Requests** - Support for If-Match headers
- **Path Security** - Built-in path traversal protection
- **Auto Content-Type** - MIME type detection based on file extension
- **Configuration** - YAML config file and environment variables
- **Code-based Migrations** - Type-safe Go migrations with dynamic table names
- **Docker Support** - Minimal scratch-based image (~15MB)

[1.1.0]: https://github.com/sagarc03/stowry/releases/tag/v1.1.0
[1.0.0]: https://github.com/sagarc03/stowry/releases/tag/v1.0.0
