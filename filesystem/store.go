// Package filesystem provides a file system storage backend for stowry.
// It supports atomic writes using temp files, SHA256-based etags, and
// content type detection based on file extensions.
package filesystem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/sagarc03/stowry"
)

// Store provides file system storage operations.
type Store struct {
	root *os.Root
}

// NewFileStorage creates a new Store with the given root directory.
// The root provides sandboxed file operations preventing path traversal.
func NewFileStorage(root *os.Root) *Store {
	return &Store{root: root}
}

// Get opens a file for reading. Returns stowry.ErrNotFound if the file does not exist.
func (s *Store) Get(ctx context.Context, path string) (io.ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f, err := s.root.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, stowry.ErrNotFound
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return f, nil
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *ctxReader) Read(p []byte) (n int, err error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// Write atomically writes content to the given path using a temp file and rename.
// It creates intermediate directories as needed and returns a SaveResult containing
// the number of bytes written and SHA256-based etag. The operation respects context cancellation.
func (s *Store) Write(ctx context.Context, path string, content io.Reader) (stowry.SaveResult, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return stowry.SaveResult{}, ctxErr
	}

	tmpFile := tmpFileName()
	t, createErr := s.root.Create(tmpFile)
	if createErr != nil {
		return stowry.SaveResult{}, fmt.Errorf("could not open temp file: %w", createErr)
	}

	success := false
	defer func() {
		if closeErr := t.Close(); closeErr != nil {
			slog.Warn("failed to close tmp file", "err", closeErr)
		}
		if !success {
			if rmErr := s.root.Remove(t.Name()); rmErr != nil {
				slog.Warn("failed to remove tmp file", "err", rmErr)
			}
		}
	}()

	h := sha256.New()
	w := io.MultiWriter(h, t)

	fileSizeBytes, err := io.Copy(w, &ctxReader{ctx: ctx, r: content})
	if err != nil {
		return stowry.SaveResult{}, fmt.Errorf("could not copy file contents: %w", err)
	}

	err = t.Sync()
	if err != nil {
		return stowry.SaveResult{}, fmt.Errorf("could not sync written file: %w", err)
	}

	destDir := filepath.Dir(path)
	if destDir != "." {
		if err := s.root.MkdirAll(destDir, 0o755); err != nil {
			return stowry.SaveResult{}, fmt.Errorf("could not create intermediate directories: %w", err)
		}
	}

	if renameErr := s.root.Rename(tmpFile, path); renameErr != nil {
		return stowry.SaveResult{}, fmt.Errorf("failed to rename file: %w", renameErr)
	}

	etag := hex.EncodeToString(h.Sum(nil))
	success = true

	return stowry.SaveResult{BytesWritten: fileSizeBytes, Etag: etag}, nil
}

// Delete removes a file. Returns stowry.ErrNotFound if the file does not exist.
func (s *Store) Delete(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := s.root.Remove(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return stowry.ErrNotFound
		}
		return fmt.Errorf("could not delete file: %w", err)
	}
	return nil
}

// List recursively walks the root directory and returns all files with their
// metadata including path, size, SHA256-based etag, and detected content type.
// This is intended for one-time initial sync operations.
func (s *Store) List(ctx context.Context) ([]stowry.ObjectEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var entries []stowry.ObjectEntry

	err := s.walkDir(ctx, ".", &entries)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return entries, nil
}

func (s *Store) walkDir(ctx context.Context, path string, entries *[]stowry.ObjectEntry) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	dirEntries, err := fs.ReadDir(s.root.FS(), path)
	if err != nil {
		return err
	}

	for _, entry := range dirEntries {
		if err := ctx.Err(); err != nil {
			return err
		}

		entryPath := filepath.Join(path, entry.Name())

		if entry.IsDir() {
			if err := s.walkDir(ctx, entryPath, entries); err != nil {
				return err
			}
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("walk dir: %w", err)
		}

		f, err := s.root.Open(entryPath)
		if err != nil {
			return fmt.Errorf("walk dir: %w", err)
		}

		h := sha256.New()
		_, copyErr := io.Copy(h, f)

		if closeErr := f.Close(); closeErr != nil {
			slog.Warn("failed to close file", "path", entryPath, "err", closeErr)
		}

		if copyErr != nil {
			return fmt.Errorf("walk dir: %w", copyErr)
		}

		etag := hex.EncodeToString(h.Sum(nil))
		contentType := detectContentType(entryPath)

		*entries = append(*entries, stowry.ObjectEntry{
			Path:        entryPath,
			Size:        info.Size(),
			ETag:        etag,
			ContentType: contentType,
		})
	}

	return nil
}

func detectContentType(path string) string {
	ext := filepath.Ext(path)
	contentType := mime.TypeByExtension(ext)

	if contentType == "" {
		return "application/octet-stream"
	}

	return contentType
}

func tmpFileName() string {
	return fmt.Sprintf(".t%s", uuid.New().String())
}
