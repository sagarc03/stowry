// Package service stores and serves objects along with their metadata.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
	"uuid"

	"github.com/sagarc03/stowry/types"
	"github.com/spf13/afero"
)

// MetaDataRepo persists object metadata. Implementations must be safe for
// concurrent use and must honour context cancellation.
type MetaDataRepo interface {
	// Get returns the metadata for path, or ErrNotFound if it does not exist.
	Get(ctx context.Context, path string) (types.MetaData, error)

	// Upsert creates or replaces the metadata for entry.Path. The bool reports
	// whether a new entry was created.
	Upsert(ctx context.Context, entry types.ObjectEntry) (types.MetaData, bool, error)

	// Delete soft-deletes the metadata for path, or returns ErrNotFound.
	Delete(ctx context.Context, path string) error

	// List returns a page of metadata entries matching q.
	List(ctx context.Context, q types.ListQuery) (types.ListResult, error)

	// ListPendingCleanup returns a page of soft-deleted entries whose files
	// have not been removed yet.
	ListPendingCleanup(ctx context.Context, q types.ListQuery) (types.ListResult, error)

	// MarkCleanedUp records that the file for a soft-deleted entry has been
	// removed. It returns ErrNotFound if the entry is not pending cleanup.
	MarkCleanedUp(ctx context.Context, id uuid.UUID) error
}

// Service serves objects from storage using repo as the metadata source of
// truth. Its mode decides how unresolved paths fall back, see resolveMetadata.
type Service struct {
	repo    MetaDataRepo
	storage afero.Fs
	mode    types.ServerMode
}

// ServiceConfig holds the options for New.
type ServiceConfig struct {
	Mode types.ServerMode
}

// New returns a Service, failing if cfg.Mode is not a valid server mode.
func New(repo MetaDataRepo, storage afero.Fs, cfg ServiceConfig) (*Service, error) {
	if !cfg.Mode.IsValid() {
		return nil, fmt.Errorf("new stowry service: invalid mode: %s", cfg.Mode)
	}
	return &Service{
		repo:    repo,
		storage: storage,
		mode:    cfg.Mode,
	}, nil
}

// Create writes content to obj.Path and records its metadata, using the
// SHA-256 of the content as the ETag. If the metadata write fails the stored
// file is removed on a best-effort basis, so no orphan is left behind.
//
// It returns ErrInvalidInput if the content type is empty or the path fails
// IsValidPath.
func (s *Service) Create(ctx context.Context, obj types.CreateObject, content io.Reader) (types.MetaData, error) {
	if err := ctx.Err(); err != nil {
		return types.MetaData{}, fmt.Errorf("context error: %w", err)
	}

	if obj.ContentType == "" {
		return types.MetaData{}, fmt.Errorf("content type empty: %w", ErrInvalidInput)
	}

	if !IsValidPath(obj.Path) {
		return types.MetaData{}, fmt.Errorf("invalid path %s: %w", obj.Path, ErrInvalidInput)
	}

	file, err := s.storage.Create(obj.Path)
	if err != nil {
		return types.MetaData{}, fmt.Errorf("failed to create file %s: %w", obj.Path, err)
	}
	defer func() { _ = file.Close }()

	h := sha256.New()
	w := io.MultiWriter(h, file)

	n, err := io.Copy(w, content)
	if err != nil {
		return types.MetaData{}, fmt.Errorf("write file %s: %w", obj.Path, err)
	}

	oe := types.ObjectEntry{
		Path:        obj.Path,
		Size:        n,
		ETag:        hex.EncodeToString(h.Sum(nil)),
		ContentType: obj.ContentType,
	}

	metaData, _, err := s.repo.Upsert(ctx, oe)
	if err != nil {
		_ = s.storage.Remove(obj.Path)
		return types.MetaData{}, fmt.Errorf("upsert failed %s: %w", obj.Path, err)
	}

	return metaData, nil
}

// resolveMetadata looks up path, falling back according to the server mode:
//   - store: no fallback, and an empty path is ErrNotFound
//   - static: "/foo/" tries {path}index.html; "/foo" tries the exact path,
//     then {path}.html, then {path}/index.html (S3 + CloudFront behaviour)
//   - spa: falls back to index.html
func (s *Service) resolveMetadata(ctx context.Context, path string) (types.MetaData, error) {
	if path == "" {
		switch s.mode {
		case types.ModeStore:
			return types.MetaData{}, ErrNotFound
		case types.ModeStatic, types.ModeSPA:
			path = "index.html"
		}
	}

	m, err := s.repo.Get(ctx, path)

	if errors.Is(err, ErrNotFound) {
		switch s.mode {
		case types.ModeStore:
		case types.ModeStatic:
			if strings.HasSuffix(path, "/") {
				m, err = s.repo.Get(ctx, path+"index.html")
			} else {
				m, err = s.repo.Get(ctx, path+".html")
				if errors.Is(err, ErrNotFound) {
					m, err = s.repo.Get(ctx, path+"/index.html")
				}
			}
		case types.ModeSPA:
			m, err = s.repo.Get(ctx, "index.html")
		}
	}

	return m, err
}

// Get returns the metadata for path together with a reader over its content.
// The caller owns the reader and must close it.
func (s *Service) Get(ctx context.Context, path string) (types.MetaData, io.ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return types.MetaData{}, nil, fmt.Errorf("context: %w", err)
	}

	m, err := s.resolveMetadata(ctx, path)
	if err != nil {
		return types.MetaData{}, nil, fmt.Errorf("meta data: %w", err)
	}

	f, err := s.storage.Open(m.Path)
	if err != nil {
		return types.MetaData{}, nil, fmt.Errorf("open file: %w", err)
	}

	return m, f, nil
}

// Info returns the metadata for path without opening its content.
func (s *Service) Info(ctx context.Context, path string) (types.MetaData, error) {
	if err := ctx.Err(); err != nil {
		return types.MetaData{}, fmt.Errorf("context: %w", err)
	}

	m, err := s.resolveMetadata(ctx, path)
	if err != nil {
		return types.MetaData{}, fmt.Errorf("meta data: %w", err)
	}

	return m, nil
}

// Delete removes the metadata for path and then its stored file. It returns
// ErrInvalidInput for an empty path. Unlike Get, it takes path literally and
// applies no mode fallback.
func (s *Service) Delete(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context: %w", err)
	}

	if path == "" {
		return fmt.Errorf("invalid path: %w", ErrInvalidInput)
	}

	err := s.repo.Delete(ctx, path)
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}

	err = s.storage.Remove(path)
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}

	return nil
}

// List returns a page of metadata entries matching q.
func (s *Service) List(ctx context.Context, q types.ListQuery) (types.ListResult, error) {
	if err := ctx.Err(); err != nil {
		return types.ListResult{}, fmt.Errorf("list object: %w", err)
	}

	result, err := s.repo.List(ctx, q)
	if err != nil {
		return types.ListResult{}, fmt.Errorf("list object: %w", err)
	}

	return result, nil
}

// IsValidPath reports whether p is usable as a storage path. A valid path is
// relative, valid UTF-8, and free of anything that could escape the store or
// confuse a URL: it must not
//   - be empty, "/", or "."
//   - start or end with "/"
//   - contain "..", "//", or a "." segment
//   - contain any of \ ? # ~
//   - contain NUL, control characters, DEL, or whitespace
func IsValidPath(p string) bool {
	if p == "" || p == "/" || p == "." {
		return false
	}

	if p[0] == '/' {
		return false
	}

	if strings.HasSuffix(p, "/") {
		return false
	}

	if strings.Contains(p, "..") {
		return false
	}

	if strings.Contains(p, "//") {
		return false
	}

	if strings.ContainsAny(p, `\?#~`) {
		return false
	}

	if !utf8.ValidString(p) {
		return false
	}

	if p == "/." || strings.Contains(p, "/./") || strings.HasSuffix(p, "/.") {
		return false
	}

	for _, r := range p {
		if r == 0 || r < 0x20 || r == 0x7f || unicode.IsSpace(r) {
			return false
		}
	}

	return true
}
