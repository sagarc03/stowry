---
title: Architecture
description: Technical architecture and design decisions in Stowry.
---

# Architecture

This document describes Stowry's technical architecture and design decisions.

## Overview

Stowry follows a clean architecture with clear separation between:

- **Core domain logic** (root package)
- **Configuration** (config package)
- **Infrastructure adapters** (database, filesystem, keybackend, http packages)
- **CLI application** (cmd/stowry)

```mermaid
flowchart TB
    subgraph Clients["🖥️ Clients"]
        cli["stowry-cli"]
        sdk["SDK / HTTP"]
    end

    subgraph Server["⚙️ Server"]
        direction TB
        httpLayer["HTTP Layer"]
        auth["Auth Middleware"]
        service["StowryService"]
        httpLayer --> auth --> service
    end

    subgraph Storage["💾 Storage"]
        direction LR
        metadata[("Metadata")]
        files[("Files")]
    end

    cli & sdk --> httpLayer
    service --> metadata & files

    style Clients fill:#e0f2fe,stroke:#0284c7,color:#0c4a6e
    style Server fill:#fef3c7,stroke:#d97706,color:#78350f
    style Storage fill:#dcfce7,stroke:#16a34a,color:#14532d
```

## Core Package

The root `stowry` package contains:

### Interfaces

```go
// MetaDataRepo persists object metadata
type MetaDataRepo interface {
    Get(ctx context.Context, path string) (MetaData, error)
    Upsert(ctx context.Context, entry MetaData) error
    Delete(ctx context.Context, path string) error
    List(ctx context.Context, query ListQuery) (ListResult, error)
    ListPendingCleanup(ctx context.Context, query ListQuery) (ListResult, error)
    MarkCleanedUp(ctx context.Context, id uuid.UUID) error
}

// FileStorage handles file operations
type FileStorage interface {
    Get(ctx context.Context, path string) (io.ReadSeekCloser, error)
    Write(ctx context.Context, path string, content io.Reader) (FileInfo, error)
    Delete(ctx context.Context, path string) error
    List(ctx context.Context) ([]FileInfo, error)
}
```

### StowryService

The main service orchestrates metadata and storage operations:

```go
type StowryService struct {
    repo    MetaDataRepo
    storage FileStorage
    mode    ServerMode
}

func (s *StowryService) Create(ctx context.Context, obj CreateObject, content io.Reader) (MetaData, error)
func (s *StowryService) Get(ctx context.Context, path string) (MetaData, io.ReadSeekCloser, error)
func (s *StowryService) Info(ctx context.Context, path string) (MetaData, error)
func (s *StowryService) Delete(ctx context.Context, path string) error
func (s *StowryService) List(ctx context.Context, query ListQuery) (ListResult, error)
func (s *StowryService) Populate(ctx context.Context) error
func (s *StowryService) Tombstone(ctx context.Context, query ListQuery) (int, error)
```

### Domain Types

```go
type MetaData struct {
    ID            uuid.UUID
    Path          string
    ContentType   string
    Etag          string      // SHA256 hash
    FileSizeBytes int64
    CreatedAt     time.Time
    UpdatedAt     time.Time
    DeletedAt     *time.Time  // Soft delete timestamp
    CleanedUpAt   *time.Time  // Tombstone timestamp
}

type ServerMode string
const (
    ModeStore  ServerMode = "store"
    ModeStatic ServerMode = "static"
    ModeSPA    ServerMode = "spa"
)
```

## Database Backends

The `database` package provides a unified interface for connecting to metadata backends.

### Unified Connection

```go
import (
    "github.com/sagarc03/stowry"
    "github.com/sagarc03/stowry/database"
)

cfg := database.Config{
    Type:   "sqlite",  // or "postgres"
    DSN:    "stowry.db",
    Tables: stowry.Tables{MetaData: "stowry_metadata"},
}

db, err := database.Connect(ctx, cfg)
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// Run migrations or validate existing schema
if err := db.Migrate(ctx); err != nil {
    log.Fatal(err)
}

repo := db.GetRepo()
```

### SQLite (database/sqlite/)

- Uses `modernc.org/sqlite` (pure Go implementation)
- Single-file database, no external process
- Code-based migrations
- Ideal for development and small deployments

### PostgreSQL (database/postgres/)

- Uses `pgx/v5` with connection pooling
- Code-based migrations
- Suitable for production and high concurrency

### Schema

Both backends use equivalent schemas with database-specific types. See [Configuration - Database](/configuration#database) for the full SQL if you prefer manual migrations.

**PostgreSQL** uses `UUID`, `TIMESTAMPTZ`, and `BIGINT` types with partial indexes for performance.

**SQLite** uses `TEXT` for all types (UUID as string, timestamps as ISO8601 strings).

### Migrations

Migrations are code-based, not SQL files. The database provides explicit methods for migration and validation:

```go
db, err := database.Connect(ctx, cfg)
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// Run migrations (creates tables if they don't exist)
if err := db.Migrate(ctx); err != nil {
    log.Fatal(err)
}

// Validate schema matches expected structure
if err := db.Validate(ctx); err != nil {
    log.Fatal(err)
}

repo := db.GetRepo()
```

The CLI handles this via the `stowry init` command which runs migrations and populates metadata from existing files.

## File Storage (filesystem/)

### Atomic Writes

Files are written atomically to prevent corruption:

1. Write content to temporary file
2. Calculate SHA256 hash
3. Rename temp file to final path

```go
func (s *FileStorage) Write(ctx context.Context, path string, content io.Reader) (FileInfo, error) {
    // 1. Create temp file
    temp, err := os.CreateTemp(dir, ".stowry-*")

    // 2. Write content, calculate hash
    hasher := sha256.New()
    writer := io.MultiWriter(temp, hasher)
    io.Copy(writer, content)

    // 3. Atomic rename
    os.Rename(temp.Name(), finalPath)
}
```

### Path Sandboxing

Uses `os.Root` to prevent path traversal attacks:

```go
root, err := os.OpenRoot(storagePath)
storage := filesystem.NewFileStorage(root)
```

### Content Type Detection

MIME types are detected from file extensions using Go's `mime` package.

## Key Backend (keybackend/)

The `keybackend` package provides pluggable secret key storage for signature verification.

### SecretStore Interface

```go
// SecretStore provides access key lookup for signature verification.
type SecretStore interface {
    Lookup(accessKey string) (secretKey string, err error)
}
```

### MapSecretStore

In-memory implementation suitable for configuration file-based key storage:

```go
store := keybackend.NewMapSecretStore(map[string]string{
    "AKIAIOSFODNN7EXAMPLE": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
})

cfg := stowry.AuthConfig{AWS: stowry.AWSConfig{Region: "us-east-1", Service: "s3"}}
verifier := stowry.NewSignatureVerifier(cfg, store)
```

## HTTP Layer (http/)

### Router

Uses Chi router with middleware:

```go
r := chi.NewRouter()
r.Use(PathValidationMiddleware)

r.Group(func(r chi.Router) {
    r.Use(AuthMiddleware(readConfig))
    r.Get("/", h.handleList)
    r.Get("/*", h.handleGet)
    r.Head("/*", h.handleHead)
})

r.Group(func(r chi.Router) {
    r.Use(AuthMiddleware(writeConfig))
    r.Put("/*", h.handlePut)
    r.Delete("/*", h.handleDelete)
})
```

### Authentication Middleware

Supports two signing schemes:

1. **Stowry Native** - `X-Stowry-*` query parameters
2. **AWS Signature V4** - `X-Amz-*` query parameters

The middleware auto-detects which scheme to use based on query parameters.

## Two-Phase Deletion

Stowry uses soft deletion with cleanup:

### Phase 1: Soft Delete

```
DELETE /file.txt
```

1. Sets `deleted_at` timestamp
2. File remains in storage
3. Object excluded from listings

### Phase 2: Tombstone (Cleanup)

```bash
stowry cleanup
```

1. Queries objects where `deleted_at` is set but `cleaned_up_at` is not
2. Deletes physical file from storage
3. Sets `cleaned_up_at` timestamp
4. Metadata retained for audit trail

### Benefits

- Recoverable deletes (before cleanup runs)
- Consistent state between metadata and storage
- Audit trail of all operations
- Graceful handling of concurrent operations

## Error Handling

### Sentinel Errors

```go
// errors.go (root package)
var (
    ErrNotFound     = errors.New("not found")
    ErrInvalidInput = errors.New("invalid input")
)

// http/errors.go
var ErrUnauthorized = errors.New("unauthorized")

// keybackend/errors.go
var ErrKeyNotFound = errors.New("access key not found")
```

### Error Wrapping

Errors are wrapped with context for debugging:

```go
return fmt.Errorf("create object %s: %w", path, err)
```

### HTTP Error Responses

```json
{
  "error": "not_found",
  "message": "Object not found"
}
```

## Pagination

Uses cursor-based pagination for consistent results:

### Cursor Format

```
Base64(updatedAt|path)
```

### Implementation

```go
type ListQuery struct {
    PathPrefix string
    Limit      int
    Cursor     string  // Encoded cursor
}

type ListResult struct {
    Items      []MetaData
    NextCursor string  // For next page
}
```

### Benefits

- Handles concurrent modifications gracefully
- No offset skipping issues
- Efficient for large datasets

## Configuration

### Precedence

1. Default values
2. Config file (config.yaml)
3. Environment variables (STOWRY\_\*)
4. Command-line flags

### Architecture Separation

Configuration logic is handled by the `config` package, which the CLI uses:

```go
// config/config.go - Configuration loading
cfg, err := config.Load(configFiles, cmd.Flags())
// Returns typed Config struct with validation

// Config is passed through context
cmd.SetContext(config.WithContext(ctx, cfg))

// Core package - no config awareness
service, err := stowry.NewStowryService(repo, storage, mode)
```

## Testing

### Unit Tests

- Mock interfaces with `testify/mock`
- Black-box testing (`stowry_test` package)
- Table-driven tests

### Integration Tests

- PostgreSQL: `testcontainers-go` for real database
- SQLite: In-memory database (`:memory:`)

### Example Test Structure

```go
func TestStowryService_Create(t *testing.T) {
    tests := []struct {
        name    string
        setup   func(*SpyMetaDataRepo, *SpyFileStorage)
        obj     CreateObject
        content string
        wantErr error
    }{
        // Test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup, execute, assert
        })
    }
}
```

## Design Decisions

### Why Not S3 API Compatible?

Stowry intentionally doesn't implement the full S3 API:

1. **Simplicity** - Smaller codebase, easier to maintain
2. **Focused** - Only presigned URL authentication
3. **Flexibility** - Can evolve independently

### Why Presigned URLs?

1. **Security** - Credentials never exposed to clients
2. **Scalability** - No session state on server
3. **Compatibility** - Works with AWS SDK presigning
