// Package types provides domain types for stowry.
package types

import (
	"errors"
	"fmt"
	"regexp"
	"time"
	"uuid"
)

type MetaData struct {
	ID            uuid.UUID
	Path          string
	ContentType   string
	Etag          string
	FileSizeBytes int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ObjectEntry struct {
	Path        string
	Size        int64
	ETag        string
	ContentType string
}

type ListQuery struct {
	PathPrefix string
	Limit      int
	Cursor     string
}

type ListResult struct {
	Items      []MetaData
	NextCursor string
}

type SaveResult struct {
	BytesWritten int64
	Etag         string
}

type CreateObject struct {
	Path        string
	ContentType string
}

type ServerMode string

const (
	ModeStore  ServerMode = "store"
	ModeStatic ServerMode = "static"
	ModeSPA    ServerMode = "spa"
)

func (m ServerMode) IsValid() bool {
	switch m {
	case ModeStore, ModeStatic, ModeSPA:
		return true
	default:
		return false
	}
}

func ParseServerMode(s string) (ServerMode, error) {
	mode := ServerMode(s)
	if !mode.IsValid() {
		return "", fmt.Errorf("invalid server mode: %s (valid modes: store, static, spa)", s)
	}
	return mode, nil
}

// Tables holds configurable table names for metadata storage.
// This allows multi-tenant deployments to use different table names.
type Tables struct {
	MetaData string
}

var validTableNameRegex = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// IsValidTableName checks if a table name is valid (lowercase, alphanumeric with underscores, max 63 chars).
func IsValidTableName(name string) bool {
	return validTableNameRegex.MatchString(name) && len(name) <= 63
}

// Validate checks that all required table names are set and valid.
func (t Tables) Validate() error {
	if t.MetaData == "" {
		return errors.New("validate tables: metadata table name cannot be empty")
	}

	if !IsValidTableName(t.MetaData) {
		return fmt.Errorf("validate tables: invalid metadata table name: %s (must match ^[a-z_][a-z0-9_]*$ and be <= 63 chars)", t.MetaData)
	}

	return nil
}

// UploadOptions configures an upload operation.
type UploadOptions struct {
	LocalPath   string
	RemotePath  string
	ContentType string // optional, auto-detect if empty
	Recursive   bool
}

// UploadResult represents the result of uploading a single file.
type UploadResult struct {
	LocalPath   string    `json:"local_path"`
	RemotePath  string    `json:"remote_path"`
	ID          uuid.UUID `json:"id"`
	ContentType string    `json:"content_type"`
	ETag        string    `json:"etag"`
	Size        int64     `json:"size_bytes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Err         error     `json:"-"` // nil on success
}

// DownloadOptions configures a download operation.
type DownloadOptions struct {
	RemotePath string
	LocalPath  string // empty = derive from remote, "-" = stdout
}

// DownloadResult represents the result of downloading a file.
type DownloadResult struct {
	RemotePath  string `json:"remote_path"`
	LocalPath   string `json:"local_path"`
	ETag        string `json:"etag"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size_bytes"`
}

// DeleteOptions configures a delete operation.
type DeleteOptions struct {
	Paths []string
}

// DeleteResult represents the result of deleting a single file.
type DeleteResult struct {
	Path    string `json:"path"`
	Deleted bool   `json:"deleted"`
	Err     error  `json:"-"` // nil on success
}

// ListOptions configures a list operation.
type ListOptions struct {
	Prefix string
	Limit  int
	Cursor string
	All    bool // auto-paginate through all results
}

// ObjectListResult contains paginated list results. It is named apart from
// ListResult, which pages the metadata the repository holds rather than the
// objects a client sees.
type ObjectListResult struct {
	Items      []ObjectInfo `json:"items"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

// ObjectInfo represents metadata for a single object.
type ObjectInfo struct {
	ID          uuid.UUID `json:"id"`
	Path        string    `json:"path"`
	ContentType string    `json:"content_type"`
	ETag        string    `json:"etag"`
	Size        int64     `json:"size_bytes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TotalSize is the size of every item in the page, in bytes.
func (r *ObjectListResult) TotalSize() int64 {
	var total int64
	for _, item := range r.Items {
		total += item.Size
	}

	return total
}
