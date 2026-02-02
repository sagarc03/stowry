package clientcli

import (
	"time"

	"github.com/google/uuid"
)

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

// ListResult contains paginated list results.
type ListResult struct {
	Items      []ObjectInfo `json:"items"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

// ObjectInfo represents metadata for a single object.
type ObjectInfo struct {
	ID          uuid.UUID `json:"id"`
	Path        string    `json:"path"`
	ContentType string    `json:"content_type"`
	ETag        string    `json:"etag"`
	Size        int64     `json:"file_size_bytes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// serverMetaData mirrors the JSON response from the server.
// Used for unmarshaling server responses.
type serverMetaData struct {
	ID            uuid.UUID `json:"id"`
	Path          string    `json:"path"`
	ContentType   string    `json:"content_type"`
	ETag          string    `json:"etag"`
	FileSizeBytes int64     `json:"file_size_bytes"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// serverListResult mirrors the JSON response from the server for list operations.
type serverListResult struct {
	Items      []serverMetaData `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
}
