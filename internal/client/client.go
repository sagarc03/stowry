// Package client talks to a stowry server over HTTP, signing every request.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sagarc03/stowry/sign"
	"github.com/sagarc03/stowry/types"
)

const (
	// DefaultTimeout is the default HTTP client timeout.
	DefaultTimeout = 30 * time.Second

	// DefaultExpires is the default presigned URL expiry in seconds (15 minutes).
	DefaultExpires = 900
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

// Config addresses one server. Defaults and validation live in package config.
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
}

// Client performs operations against a Stowry server.
type Client struct {
	endpoint   string
	httpClient *http.Client
	signer     *sign.Client
	// timeout is applied after every option, so it survives a WithHTTPClient
	// that follows it.
	timeout time.Duration
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithTimeout sets the request timeout. It applies whether or not
// WithHTTPClient is also given, and in either order.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// New creates a new Client with the given config and options.
func New(cfg *Config, opts ...Option) (*Client, error) {
	if cfg == nil {
		return nil, ErrConfigRequired
	}

	endpoint := strings.TrimSuffix(cfg.Endpoint, "/")

	c := &Client{
		// The signer holds the credentials; keeping a second copy of the secret
		// here only widens where it can leak from.
		endpoint:   endpoint,
		httpClient: &http.Client{Timeout: DefaultTimeout},
		signer:     sign.NewClient(endpoint, cfg.AccessKey, cfg.SecretKey),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.timeout > 0 {
		// A copy: a client the caller shares keeps its own timeout.
		httpClient := *c.httpClient
		httpClient.Timeout = c.timeout
		c.httpClient = &httpClient
	}

	return c, nil
}

// Upload uploads localPath. A directory is walked and uploaded whole, with the
// tree's relative paths preserved under opts.RemotePath.
func (c *Client) Upload(ctx context.Context, opts types.UploadOptions) ([]types.UploadResult, error) {
	if opts.LocalPath == "" {
		return nil, fmt.Errorf("upload: %w", ErrEmptyPath)
	}

	info, err := os.Stat(opts.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("stat local path: %w", err)
	}

	if info.IsDir() {
		return c.uploadRecursive(ctx, opts)
	}

	result, err := c.uploadSingle(ctx, opts.LocalPath, opts.RemotePath, opts.ContentType)
	if err != nil {
		return nil, err
	}

	return []types.UploadResult{result}, nil
}

func (c *Client) uploadRecursive(ctx context.Context, opts types.UploadOptions) ([]types.UploadResult, error) {
	var results []types.UploadResult
	baseDir := opts.LocalPath
	remotePrefix := strings.TrimSuffix(opts.RemotePath, "/")

	walkErr := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, fileErr error) error {
		if fileErr != nil {
			return fileErr
		}

		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		if d.IsDir() {
			return nil
		}

		relPath, relErr := filepath.Rel(baseDir, path)
		if relErr != nil {
			results = append(results, types.UploadResult{
				LocalPath: path,
				Err:       fmt.Errorf("calculate relative path: %w", relErr),
			})
			return nil
		}

		relPath = filepath.ToSlash(relPath)
		remotePath := remotePrefix + "/" + relPath

		// Empty means detect each file from its own extension.
		result, uploadErr := c.uploadSingle(ctx, path, remotePath, opts.ContentType)
		if uploadErr != nil {
			result = types.UploadResult{
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
func (c *Client) uploadSingle(ctx context.Context, localPath, remotePath, contentType string) (types.UploadResult, error) {
	file, err := os.Open(localPath) //#nosec G304 -- localPath is user-provided input
	if err != nil {
		return types.UploadResult{}, fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return types.UploadResult{}, fmt.Errorf("stat file: %w", err)
	}

	if contentType == "" {
		contentType = detectContentType(localPath)
	}

	remotePath = normalizePath(remotePath)

	presignURL := c.signer.PresignPut(remotePath, DefaultExpires)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presignURL, file)
	if err != nil {
		return types.UploadResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = info.Size()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.UploadResult{}, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.UploadResult{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return types.UploadResult{}, parseServerError(resp.StatusCode, body)
	}

	var meta types.MetaData
	if err := json.Unmarshal(body, &meta); err != nil {
		return types.UploadResult{}, fmt.Errorf("parse response: %w", err)
	}

	return types.UploadResult{
		LocalPath:   localPath,
		RemotePath:  meta.Path,
		ID:          meta.ID,
		ContentType: meta.ContentType,
		ETag:        meta.Etag,
		Size:        meta.FileSizeBytes,
		CreatedAt:   meta.CreatedAt,
		UpdatedAt:   meta.UpdatedAt,
	}, nil
}

// Download fetches a file. With opts.LocalPath "-" the body is returned for the
// caller to read and close; otherwise it is written to the file and the reader
// is nil.
func (c *Client) Download(ctx context.Context, opts types.DownloadOptions) (*types.DownloadResult, io.ReadCloser, error) {
	if opts.RemotePath == "" {
		return nil, nil, fmt.Errorf("download: %w", ErrEmptyPath)
	}
	remotePath := normalizePath(opts.RemotePath)

	presignURL := c.signer.PresignGet(remotePath, DefaultExpires)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, presignURL, http.NoBody)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, nil, parseServerError(resp.StatusCode, body)
	}

	etag := strings.Trim(resp.Header.Get("ETag"), `"`)
	contentType := resp.Header.Get("Content-Type")

	result := &types.DownloadResult{
		RemotePath:  strings.TrimPrefix(remotePath, "/"),
		ETag:        etag,
		ContentType: contentType,
		Size:        resp.ContentLength,
	}

	if opts.LocalPath == "-" {
		result.LocalPath = "-"
		return result, resp.Body, nil
	}

	localPath := opts.LocalPath
	if localPath == "" {
		localPath = filepath.Base(remotePath)
	}
	result.LocalPath = localPath

	dir := filepath.Dir(localPath)
	if dir != "" && dir != "." {
		if mkdirErr := os.MkdirAll(dir, 0o750); mkdirErr != nil {
			_ = resp.Body.Close()
			return nil, nil, fmt.Errorf("create directory: %w", mkdirErr)
		}
	}

	file, createErr := os.Create(localPath) //#nosec G304 -- localPath is user-provided input
	if createErr != nil {
		_ = resp.Body.Close()
		return nil, nil, fmt.Errorf("create file: %w", createErr)
	}

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

// Delete removes one or more files. Every path is attempted; the error is
// non-nil if any failed, and errors.Is reaches each cause.
func (c *Client) Delete(ctx context.Context, opts types.DeleteOptions) ([]types.DeleteResult, error) {
	if len(opts.Paths) == 0 {
		return nil, ErrNoPaths
	}

	results := make([]types.DeleteResult, 0, len(opts.Paths))

	for _, path := range opts.Paths {
		if err := ctx.Err(); err != nil {
			return results, err
		}

		result := c.deleteSingle(ctx, path)
		results = append(results, result)
	}

	return results, deleteError(results)
}

// deleteError joins the failures in results, or returns nil if there are none.
func deleteError(results []types.DeleteResult) error {
	var failed []error

	for _, r := range results {
		if r.Err != nil {
			failed = append(failed, fmt.Errorf("%s: %w", r.Path, r.Err))
		}
	}

	if len(failed) == 0 {
		return nil
	}

	return fmt.Errorf("delete %d of %d: %w", len(failed), len(results), errors.Join(failed...))
}

// deleteSingle deletes a single file from the server.
func (c *Client) deleteSingle(ctx context.Context, path string) types.DeleteResult {
	remotePath := normalizePath(path)

	presignURL := c.signer.PresignDelete(remotePath, DefaultExpires)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, presignURL, http.NoBody)
	if err != nil {
		return types.DeleteResult{
			Path:    path,
			Deleted: false,
			Err:     fmt.Errorf("create request: %w", err),
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return types.DeleteResult{
			Path:    path,
			Deleted: false,
			Err:     fmt.Errorf("do request: %w", err),
		}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return types.DeleteResult{
			Path:    path,
			Deleted: true,
		}
	}

	body, _ := io.ReadAll(resp.Body)
	return types.DeleteResult{
		Path:    path,
		Deleted: false,
		Err:     parseServerError(resp.StatusCode, body),
	}
}

// HasDeleteErrors reports whether any delete failed. Delete returns the same in
// its error; this is for a caller holding results it did not fetch.
func HasDeleteErrors(results []types.DeleteResult) bool {
	for _, r := range results {
		if r.Err != nil {
			return true
		}
	}
	return false
}

// List returns a page of objects, or every page when opts.All is set. Store
// mode only.
func (c *Client) List(ctx context.Context, opts types.ListOptions) (*types.ListResult, error) {
	if opts.All {
		return c.listAll(ctx, opts)
	}
	return c.listPage(ctx, opts)
}

// listPage fetches a single page of results.
func (c *Client) listPage(ctx context.Context, opts types.ListOptions) (*types.ListResult, error) {
	// The server clamps the limit and supplies its own default.
	presignURL := c.signer.PresignList(opts.Prefix, opts.Limit, opts.Cursor, DefaultExpires)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, presignURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

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

	var result types.ListResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &result, nil
}

// listAll fetches all pages of results.
func (c *Client) listAll(ctx context.Context, opts types.ListOptions) (*types.ListResult, error) {
	var allItems []types.MetaData
	cursor := opts.Cursor

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		pageOpts := types.ListOptions{
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

	return &types.ListResult{
		Items:      allItems,
		NextCursor: "", // All pages fetched
	}, nil
}

// normalizePath ensures path has leading slash and no trailing slash.
func normalizePath(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimSuffix(path, "/")
}

// NormalizeLocalToRemotePath converts a local path to a remote one: slashes are
// forward, ".", ".." and leading separators are resolved away. It returns "" if
// nothing is left.
func NormalizeLocalToRemotePath(localPath string) string {
	path := filepath.ToSlash(localPath)

	path = filepath.Clean(path)

	path = filepath.ToSlash(path)

	path = strings.TrimPrefix(path, "./")

	path = strings.TrimPrefix(path, "/")

	for strings.HasPrefix(path, "../") {
		path = strings.TrimPrefix(path, "../")
	}

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

// parseServerError decodes the error the server sent, keeping an unparseable
// body verbatim.
func parseServerError(statusCode int, body []byte) error {
	apiErr := &APIError{StatusCode: statusCode, Body: string(body)}

	var wire types.ErrorBody
	if err := json.Unmarshal(body, &wire); err == nil {
		apiErr.Code = wire.Code
		apiErr.Message = wire.Message
	}

	return apiErr
}

// APIError is an error response from the server. Code and Message are set when
// the body parsed; Body is always what arrived.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	detail := e.Message
	if detail == "" {
		detail = e.Body
	}

	return "server error: " + strconv.Itoa(e.StatusCode) + " - " + detail
}

// Is reports whether target is an *APIError with the same StatusCode.
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

// Sentinel errors to match with errors.Is.
var (
	ErrNotFound     = &APIError{StatusCode: http.StatusNotFound}
	ErrUnauthorized = &APIError{StatusCode: http.StatusUnauthorized}
	ErrForbidden    = &APIError{StatusCode: http.StatusForbidden}
)
