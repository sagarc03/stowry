package stowry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// MetaDataRepo defines the interface for managing object metadata persistence.
// Implementations must handle concurrent access safely and ensure data consistency.
//
// All methods accept a context for cancellation and timeout control.
// Implementations should respect context cancellation and return appropriate errors.
type MetaDataRepo interface {
	// Get retrieves metadata for a specific object by its path.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - path: The object path to look up
	//
	// Returns:
	//   - MetaData: The metadata entry if found
	//   - error: ErrNotFound if path doesn't exist, or other database errors
	Get(ctx context.Context, path string) (MetaData, error)

	// Upsert creates or updates metadata for an object.
	// If an entry with the same path exists, it updates the existing entry.
	// If no entry exists, it creates a new one.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - entry: ObjectEntry containing path, size, ETag, and content type
	//
	// Returns:
	//   - MetaData: The created or updated metadata entry with ID and timestamps
	//   - bool: true if a new entry was created, false if existing entry was updated
	//   - error: Any database or validation error
	Upsert(ctx context.Context, entry ObjectEntry) (MetaData, bool, error)

	// Delete removes metadata for a specific object by its path.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - path: The object path to delete
	//
	// Returns:
	//   - error: ErrNotFound if path doesn't exist, or other database errors
	Delete(ctx context.Context, path string) error

	// List retrieves a paginated list of metadata entries matching the query criteria.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - q: ListQuery with optional path prefix filter, limit, and cursor for pagination
	//
	// Returns:
	//   - ListResult: Contains matching metadata items and cursor for next page
	//   - error: Any database error
	List(ctx context.Context, q ListQuery) (ListResult, error)

	// ListPendingCleanup retrieves a paginated list of soft-deleted metadata entries
	// that have not yet been cleaned up (deleted_at IS NOT NULL AND cleaned_up_at IS NULL).
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - q: ListQuery with optional path prefix filter, limit, and cursor for pagination
	//
	// Returns:
	//   - ListResult: Contains matching metadata items and cursor for next page
	//   - error: Any database error
	ListPendingCleanup(ctx context.Context, q ListQuery) (ListResult, error)

	// MarkCleanedUp marks a soft-deleted metadata entry as cleaned up by setting cleaned_up_at.
	// This should be called after the physical file has been deleted.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - id: The UUID of the metadata entry to mark as cleaned up
	//
	// Returns:
	//   - error: ErrNotFound if entry doesn't exist or isn't pending cleanup, or other database errors
	MarkCleanedUp(ctx context.Context, id uuid.UUID) error
}

// FileStorage defines the interface for physical file storage operations.
// Implementations can use local filesystem, S3, GCS, or any other storage backend.
//
// All methods accept a context for cancellation and timeout control.
// Implementations should respect context cancellation during long-running operations
// like large file uploads or downloads.
type FileStorage interface {
	// Get retrieves a file from storage for reading.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - path: The object path to retrieve
	//
	// Returns:
	//   - io.ReadSeekCloser: Reader for file content with seek capability
	//   - error: ErrNotFound if file doesn't exist, or other storage errors
	//
	// The caller is responsible for closing the returned ReadSeekCloser.
	// Implementations should return a ReadSeekCloser to support range reads
	// and efficient streaming.
	Get(ctx context.Context, path string) (io.ReadSeekCloser, error)

	// Write stores content to a file at the specified path.
	// If a file already exists at the path, it should be overwritten.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - path: The destination path for the file
	//   - content: io.Reader providing the data to write
	//
	// Returns:
	//   - SaveResult: Contains bytes written and computed ETag/hash
	//   - error: Any storage or I/O error
	//
	// Implementations should:
	//   - Write atomically when possible (e.g., write to temp file then rename)
	//   - Compute an ETag or hash during write for integrity verification
	//   - Return accurate byte count of data written
	//   - Handle context cancellation gracefully and clean up partial writes
	//   - Create parent directories if they don't exist
	Write(ctx context.Context, path string, content io.Reader) (SaveResult, error)

	// Delete removes a file from storage.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - path: The object path to delete
	//
	// Returns:
	//   - error: ErrNotFound if file doesn't exist, or other storage errors
	//
	// Note: This only deletes the physical file, not its metadata.
	// Callers are responsible for coordinating file and metadata deletion.
	Delete(ctx context.Context, path string) error

	// List returns all objects currently in storage with their metadata.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//
	// Returns:
	//   - []ObjectEntry: Slice of all objects with path, size, ETag, and content type
	//   - error: Any storage or I/O error
	//
	// This method is typically used for:
	//   - Synchronizing metadata with physical storage (see StowryService.Populate)
	//   - Recovery operations after metadata loss
	//   - Storage health checks and auditing
	//
	// Implementations should:
	//   - Walk the entire storage tree recursively
	//   - Detect content type from file extensions or content inspection
	//   - Compute ETag/hash for each file
	//   - Return an empty slice (not nil) when storage is empty
	//
	// Warning: This can be expensive for large storage volumes. Use with caution
	// in production and consider implementing pagination for very large datasets.
	List(ctx context.Context) ([]ObjectEntry, error)
}

type StowryService struct {
	repo           MetaDataRepo
	storage        FileStorage
	mode           ServerMode
	cleanupTimeout time.Duration
}

// ServiceConfig holds configuration options for StowryService.
type ServiceConfig struct {
	Mode           ServerMode
	CleanupTimeout time.Duration // Timeout for cleanup operations (default: 30s)
}

func NewStowryService(repo MetaDataRepo, storage FileStorage, cfg ServiceConfig) (*StowryService, error) {
	if !cfg.Mode.IsValid() {
		return nil, fmt.Errorf("new stowry service: invalid mode: %s", cfg.Mode)
	}
	cleanupTimeout := cfg.CleanupTimeout
	if cleanupTimeout <= 0 {
		cleanupTimeout = 30 * time.Second
	}
	return &StowryService{
		repo:           repo,
		storage:        storage,
		mode:           cfg.Mode,
		cleanupTimeout: cleanupTimeout,
	}, nil
}

// Populate synchronizes metadata from physical storage files.
// It lists all files in storage and creates or updates their corresponding metadata entries.
//
// This method is typically used during initialization or recovery to ensure the metadata
// repository is in sync with actual files in storage. It processes all files sequentially
// and stops at the first error encountered.
//
// Returns an error if:
//   - Storage listing fails
//   - Any metadata upsert operation fails
//   - Context is cancelled during processing
//
// Note: This operation is not atomic. If it fails partway through, some files may have
// been processed while others remain unprocessed.
func (s *StowryService) Populate(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("populate: %w", err)
	}

	files, listErr := s.storage.List(ctx)
	if listErr != nil {
		return fmt.Errorf("populate: %w", listErr)
	}

	for _, file := range files {
		_, _, upsertErr := s.repo.Upsert(ctx, file)
		if upsertErr != nil {
			return fmt.Errorf("populate '%s': %w", file.Path, upsertErr)
		}
	}

	return nil
}

// Create stores a new object in storage and creates its metadata entry.
// It performs comprehensive validation, writes the content to storage, and creates
// a corresponding metadata entry. If the metadata creation fails, the stored file
// is automatically cleaned up to prevent orphaned data.
//
// The method performs the following steps:
//  1. Validates context is not cancelled
//  2. Validates input parameters (path, content type)
//  3. Validates path using IsValidPath (prevents path traversal attacks)
//  4. Writes content to storage and computes ETag
//  5. Creates metadata entry
//  6. On metadata failure, automatically deletes the stored file
//
// Parameters:
//   - ctx: Context for cancellation and timeout. If cancelled during storage write,
//     the operation may still complete. Cleanup uses a separate background context.
//   - obj: CreateObject containing path and content type
//   - content: io.Reader providing the object data to store
//
// Returns:
//   - MetaData: The created metadata entry with ID, timestamps, and computed ETag
//   - error: Any error encountered, including validation, storage, or metadata errors
//
// Error types returned:
//   - ErrInvalidInput: Empty path or content type
//   - ErrInvalidInput: Path fails validation (contains .., //, invalid chars, etc.)
//   - context.Canceled or context.DeadlineExceeded: Context was cancelled
//   - Wrapped storage errors: Issues writing to storage
//   - Wrapped metadata errors: Issues creating metadata entry
//
// Concurrency safety: Safe for concurrent calls with different paths.
// Data consistency: If metadata creation fails, the stored file is automatically deleted
// using a background context with the configured cleanup timeout to ensure cleanup completes
// even if the original context is cancelled.
func (s *StowryService) Create(ctx context.Context, obj CreateObject, content io.Reader) (MetaData, error) {
	// Early context check - fail fast before expensive operations
	if err := ctx.Err(); err != nil {
		return MetaData{}, fmt.Errorf("create object: %w", err)
	}

	// Input validation
	if obj.Path == "" {
		return MetaData{}, fmt.Errorf("create object: %w: path cannot be empty", ErrInvalidInput)
	}

	if obj.ContentType == "" {
		return MetaData{}, fmt.Errorf("create object: %w: content type cannot be empty", ErrInvalidInput)
	}

	// Path validation using IsValidPath
	if !IsValidPath(obj.Path) {
		return MetaData{}, fmt.Errorf("create object %s: %w", obj.Path, ErrInvalidInput)
	}

	// Write to storage
	saveResult, writeErr := s.storage.Write(ctx, obj.Path, content)
	if writeErr != nil {
		return MetaData{}, fmt.Errorf("create object %s: write failed: %w", obj.Path, writeErr)
	}

	// Create metadata entry
	oe := ObjectEntry{
		Path:        obj.Path,
		Size:        saveResult.BytesWritten,
		ETag:        saveResult.Etag,
		ContentType: obj.ContentType,
	}

	metaData, _, upsertErr := s.repo.Upsert(ctx, oe)
	if upsertErr != nil {
		// Use background context for cleanup since original context may be cancelled
		cleanupCtx, cancel := context.WithTimeout(context.Background(), s.cleanupTimeout)
		defer cancel()

		if delErr := s.storage.Delete(cleanupCtx, obj.Path); delErr != nil {
			return MetaData{}, fmt.Errorf("create object %s: metadata upsert failed (%w) and cleanup failed: %w", obj.Path, upsertErr, delErr)
		}
		return MetaData{}, fmt.Errorf("create object %s: metadata upsert failed: %w", obj.Path, upsertErr)
	}

	return metaData, nil
}

func (s *StowryService) Get(ctx context.Context, path string) (MetaData, io.ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return MetaData{}, nil, fmt.Errorf("get object: %w", err)
	}

	// Handle root path based on mode
	if path == "" {
		switch s.mode {
		case ModeStore:
			return MetaData{}, nil, fmt.Errorf("get object: %w", ErrNotFound)
		case ModeStatic, ModeSPA:
			path = "index.html"
		}
	}

	m, err := s.repo.Get(ctx, path)

	if errors.Is(err, ErrNotFound) {
		switch s.mode {
		case ModeStore:
			// No fallback in store mode
		case ModeStatic:
			m, err = s.repo.Get(ctx, filepath.Join(path, "index.html"))
		case ModeSPA:
			m, err = s.repo.Get(ctx, "index.html")
		}
	}

	if err != nil {
		return MetaData{}, nil, fmt.Errorf("get object: %w", err)
	}

	f, err := s.storage.Get(ctx, m.Path)
	if err != nil {
		return MetaData{}, nil, fmt.Errorf("get object: %w", err)
	}

	return m, f, nil
}

func (s *StowryService) Delete(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete object: %w", err)
	}

	if path == "" {
		return fmt.Errorf("delete object: %w: path cannot be empty", ErrInvalidInput)
	}

	err := s.repo.Delete(ctx, path)
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}

	return nil
}

func (s *StowryService) List(ctx context.Context, q ListQuery) (ListResult, error) {
	if err := ctx.Err(); err != nil {
		return ListResult{}, fmt.Errorf("list object: %w", err)
	}

	result, err := s.repo.List(ctx, q)
	if err != nil {
		return ListResult{}, fmt.Errorf("list object: %w", err)
	}

	return result, nil
}

// Tombstone permanently removes all soft-deleted files from storage and marks them as cleaned up.
// It processes all pending cleanup items by paginating through until none remain.
//
// The method performs the following for each soft-deleted file:
//  1. Deletes the physical file from storage
//  2. Marks the metadata entry as cleaned up (sets cleaned_up_at)
//
// If a file has already been deleted from storage (ErrNotFound), the method continues
// and marks it as cleaned up anyway - this handles the case where a previous cleanup
// attempt deleted the file but failed to mark the metadata.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - q: ListQuery with optional path prefix filter and limit (cursor is managed internally)
//
// Returns:
//   - int: Total number of items cleaned up
//   - error: Any error encountered during cleanup
func (s *StowryService) Tombstone(ctx context.Context, q ListQuery) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("tombstone: %w", err)
	}

	totalCleaned := 0
	cursor := q.Cursor

	for {
		if err := ctx.Err(); err != nil {
			return totalCleaned, fmt.Errorf("tombstone: %w", err)
		}

		query := ListQuery{
			PathPrefix: q.PathPrefix,
			Limit:      q.Limit,
			Cursor:     cursor,
		}

		result, listErr := s.repo.ListPendingCleanup(ctx, query)
		if listErr != nil {
			return totalCleaned, fmt.Errorf("tombstone: %w", listErr)
		}

		if len(result.Items) == 0 {
			break
		}

		for _, file := range result.Items {
			deleteErr := s.storage.Delete(ctx, file.Path)
			// Ignore ErrNotFound - file may have been deleted already
			if deleteErr != nil && !errors.Is(deleteErr, ErrNotFound) {
				return totalCleaned, fmt.Errorf("tombstone '%s': %w", file.Path, deleteErr)
			}

			updateErr := s.repo.MarkCleanedUp(ctx, file.ID)
			if updateErr != nil {
				return totalCleaned, fmt.Errorf("tombstone '%s': %w", file.Path, updateErr)
			}

			totalCleaned++
		}

		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	return totalCleaned, nil
}
