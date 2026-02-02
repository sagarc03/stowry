package clientcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sagarc03/stowry-go"
)

const (
	// DefaultTimeout is the default HTTP client timeout.
	DefaultTimeout = 30 * time.Second

	// DefaultExpires is the default presigned URL expiry in seconds (15 minutes).
	DefaultExpires = 900
)

// Client performs operations against a Stowry server.
type Client struct {
	config     *Config
	httpClient *http.Client
	signer     *stowry.Client
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// New creates a new Client with the given config and options.
func New(cfg *Config, opts ...Option) (*Client, error) {
	if cfg == nil {
		return nil, ErrConfigRequired
	}

	// Apply defaults
	cfg = cfg.WithDefaults()

	// Normalize endpoint URL (remove trailing slash)
	endpoint := strings.TrimSuffix(cfg.Endpoint, "/")

	c := &Client{
		config: &Config{
			Endpoint:  endpoint,
			AccessKey: cfg.AccessKey,
			SecretKey: cfg.SecretKey,
		},
		httpClient: &http.Client{Timeout: DefaultTimeout},
		signer:     stowry.NewClient(endpoint, cfg.AccessKey, cfg.SecretKey),
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// Upload uploads file(s) to the server.
// For recursive uploads, walks directory and preserves relative paths.
func (c *Client) Upload(ctx context.Context, opts UploadOptions) ([]UploadResult, error) {
	if opts.LocalPath == "" {
		return nil, fmt.Errorf("upload: %w", ErrEmptyPath)
	}
	if opts.Recursive {
		return c.uploadRecursive(ctx, opts)
	}
	result, err := c.uploadSingle(ctx, opts.LocalPath, opts.RemotePath, opts.ContentType)
	if err != nil {
		return nil, err
	}
	return []UploadResult{result}, nil
}

// uploadRecursive walks a directory and uploads all files.
func (c *Client) uploadRecursive(ctx context.Context, opts UploadOptions) ([]UploadResult, error) {
	info, err := os.Stat(opts.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("stat local path: %w", err)
	}

	if !info.IsDir() {
		// Not a directory, just upload single file
		result, uploadErr := c.uploadSingle(ctx, opts.LocalPath, opts.RemotePath, opts.ContentType)
		if uploadErr != nil {
			return nil, uploadErr
		}
		return []UploadResult{result}, nil
	}

	var results []UploadResult
	baseDir := opts.LocalPath
	remotePrefix := strings.TrimSuffix(opts.RemotePath, "/")

	walkErr := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, fileErr error) error {
		if fileErr != nil {
			return fileErr
		}

		// Check context cancellation
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Calculate relative path
		relPath, relErr := filepath.Rel(baseDir, path)
		if relErr != nil {
			results = append(results, UploadResult{
				LocalPath: path,
				Err:       fmt.Errorf("calculate relative path: %w", relErr),
			})
			return nil
		}

		// Convert to forward slashes for remote path
		relPath = filepath.ToSlash(relPath)
		remotePath := remotePrefix + "/" + relPath

		result, uploadErr := c.uploadSingle(ctx, path, remotePath, "")
		if uploadErr != nil {
			result = UploadResult{
				LocalPath:  path,
				RemotePath: remotePath,
				Err:        uploadErr,
			}
		}
		results = append(results, result)
		return nil
	})

	if walkErr != nil {
		return results, fmt.Errorf("walk directory: %w", walkErr)
	}

	return results, nil
}

// uploadSingle uploads a single file to the server.
func (c *Client) uploadSingle(ctx context.Context, localPath, remotePath, contentType string) (UploadResult, error) {
	// Open the file
	file, err := os.Open(localPath) //#nosec G304 -- localPath is user-provided input
	if err != nil {
		return UploadResult{}, fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Get file info for size
	info, err := file.Stat()
	if err != nil {
		return UploadResult{}, fmt.Errorf("stat file: %w", err)
	}

	// Auto-detect content type if not provided
	if contentType == "" {
		contentType = detectContentType(localPath)
	}

	// Normalize remote path
	remotePath = normalizePath(remotePath)

	// Generate presigned URL
	presignURL := c.signer.PresignPut(remotePath, DefaultExpires)

	// Create request with file as body (streaming, no memory copy)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presignURL, file)
	if err != nil {
		return UploadResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = info.Size()

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return UploadResult{}, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return UploadResult{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return UploadResult{}, parseServerError(resp.StatusCode, body)
	}

	// Parse response
	var meta serverMetaData
	if err := json.Unmarshal(body, &meta); err != nil {
		return UploadResult{}, fmt.Errorf("parse response: %w", err)
	}

	return UploadResult{
		LocalPath:   localPath,
		RemotePath:  meta.Path,
		ID:          meta.ID,
		ContentType: meta.ContentType,
		ETag:        meta.ETag,
		Size:        meta.FileSizeBytes,
		CreatedAt:   meta.CreatedAt,
		UpdatedAt:   meta.UpdatedAt,
	}, nil
}

// Download downloads a file from the server.
// If opts.LocalPath is "-", the content is returned via the io.ReadCloser and must be closed by the caller.
// Otherwise, the content is written to the file and the io.ReadCloser is nil.
func (c *Client) Download(ctx context.Context, opts DownloadOptions) (*DownloadResult, io.ReadCloser, error) {
	if opts.RemotePath == "" {
		return nil, nil, fmt.Errorf("download: %w", ErrEmptyPath)
	}
	remotePath := normalizePath(opts.RemotePath)

	// Generate presigned URL
	presignURL := c.signer.PresignGet(remotePath, DefaultExpires)

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, presignURL, http.NoBody)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, nil, parseServerError(resp.StatusCode, body)
	}

	// Extract metadata from headers
	etag := strings.Trim(resp.Header.Get("ETag"), `"`)
	contentType := resp.Header.Get("Content-Type")

	result := &DownloadResult{
		RemotePath:  strings.TrimPrefix(remotePath, "/"),
		ETag:        etag,
		ContentType: contentType,
		Size:        resp.ContentLength,
	}

	// If stdout requested, return the body for the caller to handle
	if opts.LocalPath == "-" {
		result.LocalPath = "-"
		return result, resp.Body, nil
	}

	// Determine local path
	localPath := opts.LocalPath
	if localPath == "" {
		// Derive from remote path
		localPath = filepath.Base(remotePath)
	}
	result.LocalPath = localPath

	// Create parent directories if needed
	dir := filepath.Dir(localPath)
	if dir != "" && dir != "." {
		if mkdirErr := os.MkdirAll(dir, 0o750); mkdirErr != nil {
			_ = resp.Body.Close()
			return nil, nil, fmt.Errorf("create directory: %w", mkdirErr)
		}
	}

	// Create the file
	file, createErr := os.Create(localPath) //#nosec G304 -- localPath is user-provided input
	if createErr != nil {
		_ = resp.Body.Close()
		return nil, nil, fmt.Errorf("create file: %w", createErr)
	}

	// Copy content to file
	written, copyErr := io.Copy(file, resp.Body)
	_ = resp.Body.Close()
	if copyErr != nil {
		_ = file.Close()
		return nil, nil, fmt.Errorf("write file: %w", copyErr)
	}

	if closeErr := file.Close(); closeErr != nil {
		return nil, nil, fmt.Errorf("close file: %w", closeErr)
	}

	result.Size = written
	return result, nil, nil
}

// Delete deletes one or more files from the server.
// Continues on error, collecting results for all paths.
func (c *Client) Delete(ctx context.Context, opts DeleteOptions) ([]DeleteResult, error) {
	if len(opts.Paths) == 0 {
		return nil, ErrNoPaths
	}

	results := make([]DeleteResult, 0, len(opts.Paths))

	for _, path := range opts.Paths {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return results, err
		}

		result := c.deleteSingle(ctx, path)
		results = append(results, result)
	}

	return results, nil
}

// deleteSingle deletes a single file from the server.
func (c *Client) deleteSingle(ctx context.Context, path string) DeleteResult {
	remotePath := normalizePath(path)

	// Generate presigned URL
	presignURL := c.signer.PresignDelete(remotePath, DefaultExpires)

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, presignURL, http.NoBody)
	if err != nil {
		return DeleteResult{
			Path:    path,
			Deleted: false,
			Err:     fmt.Errorf("create request: %w", err),
		}
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DeleteResult{
			Path:    path,
			Deleted: false,
			Err:     fmt.Errorf("do request: %w", err),
		}
	}
	defer func() { _ = resp.Body.Close() }()

	// 204 No Content is success
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return DeleteResult{
			Path:    path,
			Deleted: true,
		}
	}

	body, _ := io.ReadAll(resp.Body)
	return DeleteResult{
		Path:    path,
		Deleted: false,
		Err:     parseServerError(resp.StatusCode, body),
	}
}

// HasDeleteErrors returns true if any delete operation failed.
func HasDeleteErrors(results []DeleteResult) bool {
	for _, r := range results {
		if r.Err != nil {
			return true
		}
	}
	return false
}

// List lists objects on the server (store mode only).
// If opts.All is true, paginates through all results.
func (c *Client) List(ctx context.Context, opts ListOptions) (*ListResult, error) {
	if opts.All {
		return c.listAll(ctx, opts)
	}
	return c.listPage(ctx, opts)
}

// listPage fetches a single page of results.
func (c *Client) listPage(ctx context.Context, opts ListOptions) (*ListResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	// Generate presigned URL
	presignURL := c.presignList(opts.Prefix, limit, opts.Cursor, DefaultExpires)

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, presignURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, parseServerError(resp.StatusCode, body)
	}

	// Parse response
	var serverResult serverListResult
	if err := json.Unmarshal(body, &serverResult); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// Convert to client types
	items := make([]ObjectInfo, len(serverResult.Items))
	for i, item := range serverResult.Items {
		items[i] = ObjectInfo{
			ID:          item.ID,
			Path:        item.Path,
			ContentType: item.ContentType,
			ETag:        item.ETag,
			Size:        item.FileSizeBytes,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		}
	}

	return &ListResult{
		Items:      items,
		NextCursor: serverResult.NextCursor,
	}, nil
}

// listAll fetches all pages of results.
func (c *Client) listAll(ctx context.Context, opts ListOptions) (*ListResult, error) {
	var allItems []ObjectInfo
	cursor := opts.Cursor

	for {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		pageOpts := ListOptions{
			Prefix: opts.Prefix,
			Limit:  opts.Limit,
			Cursor: cursor,
			All:    false, // Prevent recursion
		}

		page, err := c.listPage(ctx, pageOpts)
		if err != nil {
			return nil, err
		}

		allItems = append(allItems, page.Items...)

		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}

	return &ListResult{
		Items:      allItems,
		NextCursor: "", // All pages fetched
	}, nil
}

// TotalSize calculates the total size of all items in bytes.
func (r *ListResult) TotalSize() int64 {
	var total int64
	for _, item := range r.Items {
		total += item.Size
	}
	return total
}

// presignList generates a presigned URL for list operations.
// This is implemented manually since stowry-go doesn't have PresignList.
func (c *Client) presignList(prefix string, limit int, cursor string, expires int) string {
	if expires <= 0 {
		expires = DefaultExpires
	}

	timestamp := time.Now().Unix()
	path := "/"
	sig := stowry.Sign(c.config.SecretKey, http.MethodGet, path, timestamp, int64(expires))

	query := url.Values{}
	query.Set(stowry.StowryCredentialParam, c.config.AccessKey)
	query.Set(stowry.StowryDateParam, strconv.FormatInt(timestamp, 10))
	query.Set(stowry.StowryExpiresParam, strconv.Itoa(expires))
	query.Set(stowry.StowrySignatureParam, sig)

	if prefix != "" {
		query.Set("prefix", prefix)
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		query.Set("cursor", cursor)
	}

	return c.config.Endpoint + path + "?" + query.Encode()
}

// normalizePath ensures path has leading slash and no trailing slash.
func normalizePath(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimSuffix(path, "/")
}

// NormalizeLocalToRemotePath converts a local path to a clean remote path.
// It handles:
//   - Leading "./" is stripped (./foo/bar.txt -> foo/bar.txt)
//   - Leading "/" is stripped (/abs/path/file.txt -> abs/path/file.txt)
//   - Parent traversal is resolved (../sibling/file.txt -> sibling/file.txt)
//   - Multiple slashes are collapsed
//   - Backslashes are converted to forward slashes (Windows)
func NormalizeLocalToRemotePath(localPath string) string {
	// Convert to forward slashes (Windows compatibility)
	path := filepath.ToSlash(localPath)

	// Clean the path (resolves . and .. segments)
	path = filepath.Clean(path)

	// Convert back to forward slashes after Clean (Clean uses OS separator)
	path = filepath.ToSlash(path)

	// Strip leading "./"
	path = strings.TrimPrefix(path, "./")

	// Strip leading "/" (absolute paths)
	path = strings.TrimPrefix(path, "/")

	// Handle edge case where Clean might produce ".."
	// Keep stripping leading "../" segments
	for strings.HasPrefix(path, "../") {
		path = strings.TrimPrefix(path, "../")
	}

	// Handle edge case where path is just ".." or "."
	if path == ".." || path == "." {
		return ""
	}

	return path
}

// detectContentType returns MIME type based on file extension.
func detectContentType(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return "application/octet-stream"
	}

	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		return "application/octet-stream"
	}

	return mimeType
}

// parseServerError extracts error message from server response.
func parseServerError(statusCode int, body []byte) error {
	return &APIError{
		StatusCode: statusCode,
		Body:       string(body),
	}
}

// APIError represents an error response from the server.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return "server error: " + strconv.Itoa(e.StatusCode) + " - " + e.Body
}

// Is reports whether target matches this error.
// It matches if target is an *APIError with the same StatusCode.
func (e *APIError) Is(target error) bool {
	var t *APIError
	ok := errors.As(target, &t)
	if !ok {
		return false
	}
	return t.StatusCode == e.StatusCode
}

// IsNotFound returns true if the error is a 404.
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// Sentinel errors for common API error conditions.
// Use errors.Is() to check for these conditions.
var (
	// ErrNotFound is returned when the requested resource does not exist (404).
	ErrNotFound = &APIError{StatusCode: http.StatusNotFound}

	// ErrUnauthorized is returned when authentication fails (401).
	// This typically means invalid or missing credentials.
	ErrUnauthorized = &APIError{StatusCode: http.StatusUnauthorized}

	// ErrForbidden is returned when the request is not permitted (403).
	// This typically means the credentials are valid but lack permission.
	ErrForbidden = &APIError{StatusCode: http.StatusForbidden}
)
