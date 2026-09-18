// Package service stores and serves objects along with their metadata.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
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
// truth. Every method takes its path literally; callers resolve request paths
// to object paths themselves.
type Service struct {
	repo    MetaDataRepo
	storage afero.Fs
}

// New returns a Service backed by repo and storage.
func New(repo MetaDataRepo, storage afero.Fs) *Service {
	return &Service{
		repo:    repo,
		storage: storage,
	}
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

	// Create does not make parent directories.
	if dir := path.Dir(obj.Path); dir != "." {
		if err := s.storage.MkdirAll(dir, 0o755); err != nil {
			return types.MetaData{}, fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	file, err := s.storage.Create(obj.Path)
	if err != nil {
		return types.MetaData{}, fmt.Errorf("failed to create file %s: %w", obj.Path, err)
	}
	defer func() { _ = file.Close() }()

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

// Get returns the metadata for path together with a reader over its content.
// The caller owns the reader and must close it. It returns ErrInvalidInput if
// path fails IsValidPath, or ErrNotFound if no object is stored at path.
func (s *Service) Get(ctx context.Context, path string) (types.MetaData, io.ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return types.MetaData{}, nil, fmt.Errorf("context: %w", err)
	}

	if !IsValidPath(path) {
		return types.MetaData{}, nil, fmt.Errorf("invalid path %s: %w", path, ErrInvalidInput)
	}

	m, err := s.repo.Get(ctx, path)
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

	if !IsValidPath(path) {
		return types.MetaData{}, fmt.Errorf("invalid path %s: %w", path, ErrInvalidInput)
	}

	m, err := s.repo.Get(ctx, path)
	if err != nil {
		return types.MetaData{}, fmt.Errorf("meta data: %w", err)
	}

	return m, nil
}

// Delete removes the metadata for path and then its stored file. It returns
// ErrInvalidInput for an empty path. It does not apply IsValidPath, so entries
// stored under paths that predate the current rules can still be removed.
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

// Populate records metadata for the files already in storage, so a directory
// of existing files can be served without uploading anything. The files are
// only read: their layout under the storage root becomes the object paths.
// An entry that already exists is updated in place.
//
// It stops at the first file it cannot record, returning the entries written
// before it.
func (s *Service) Populate(ctx context.Context) ([]types.MetaData, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context: %w", err)
	}

	// Walking from "." is what yields object paths: relative to the storage
	// root, slash-separated, no leading slash.
	var paths []string

	err := afero.Walk(s.storage, ".", func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			paths = append(paths, name)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list storage: %w", err)
	}

	entries := make([]types.MetaData, 0, len(paths))

	for _, p := range paths {
		m, err := s.populateOne(ctx, p)
		if err != nil {
			return entries, err
		}

		entries = append(entries, m)
	}

	return entries, nil
}

// populateOne hashes one stored file and records what it found.
func (s *Service) populateOne(ctx context.Context, p string) (types.MetaData, error) {
	if !IsValidPath(p) {
		return types.MetaData{}, fmt.Errorf("invalid path %s: %w", p, ErrInvalidInput)
	}

	file, err := s.storage.Open(p)
	if err != nil {
		return types.MetaData{}, fmt.Errorf("open file %s: %w", p, err)
	}
	defer func() { _ = file.Close() }()

	h := sha256.New()

	n, err := io.Copy(h, file)
	if err != nil {
		return types.MetaData{}, fmt.Errorf("read file %s: %w", p, err)
	}

	m, _, err := s.repo.Upsert(ctx, types.ObjectEntry{
		Path:        p,
		Size:        n,
		ETag:        hex.EncodeToString(h.Sum(nil)),
		ContentType: contentType(p),
	})
	if err != nil {
		return types.MetaData{}, fmt.Errorf("upsert failed %s: %w", p, err)
	}

	return m, nil
}

// contentType guesses from the extension, since a file on disk carries none of
// its own.
func contentType(name string) string {
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		return ct
	}

	return "application/octet-stream"
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
