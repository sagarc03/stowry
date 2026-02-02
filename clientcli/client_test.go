package clientcli_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sagarc03/stowry/clientcli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &clientcli.Config{
			Server:    "http://localhost:5708",
			AccessKey: "test-key",
			SecretKey: "test-secret",
		}

		client, err := clientcli.New(cfg)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("empty server uses default", func(t *testing.T) {
		cfg := &clientcli.Config{}

		client, err := clientcli.New(cfg)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("trailing slash removed", func(t *testing.T) {
		cfg := &clientcli.Config{
			Server: "http://localhost:5708/",
		}

		client, err := clientcli.New(cfg)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})
}

func TestClient_Upload(t *testing.T) {
	t.Run("successful upload", func(t *testing.T) {
		// Create mock server
		expectedID := uuid.New()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method)
			assert.Contains(t, r.URL.Path, "/test/file.txt")
			assert.Equal(t, "text/plain; charset=utf-8", r.Header.Get("Content-Type"))

			// Read body to verify content
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			assert.Equal(t, "test content", string(body))

			// Return metadata response
			resp := map[string]any{
				"id":              expectedID.String(),
				"path":            "test/file.txt",
				"content_type":    "text/plain; charset=utf-8",
				"etag":            "abc123",
				"file_size_bytes": 12,
				"created_at":      time.Now().Format(time.RFC3339),
				"updated_at":      time.Now().Format(time.RFC3339),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		// Create temp file
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "file.txt")
		err := os.WriteFile(localPath, []byte("test content"), 0o600)
		require.NoError(t, err)

		// Create client
		cfg := &clientcli.Config{
			Server:    server.URL,
			AccessKey: "test-key",
			SecretKey: "test-secret",
		}
		client, err := clientcli.New(cfg)
		require.NoError(t, err)

		// Upload
		results, err := client.Upload(context.Background(), clientcli.UploadOptions{
			LocalPath:  localPath,
			RemotePath: "test/file.txt",
		})
		require.NoError(t, err)
		require.Len(t, results, 1)

		result := results[0]
		assert.Equal(t, localPath, result.LocalPath)
		assert.Equal(t, "test/file.txt", result.RemotePath)
		assert.Equal(t, expectedID, result.ID)
		assert.Equal(t, "abc123", result.ETag)
		assert.Nil(t, result.Err)
	})

	t.Run("upload error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "internal_error", "message": "Something went wrong"}`))
		}))
		defer server.Close()

		// Create temp file
		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "file.txt")
		err := os.WriteFile(localPath, []byte("test content"), 0o600)
		require.NoError(t, err)

		cfg := &clientcli.Config{
			Server:    server.URL,
			AccessKey: "test-key",
			SecretKey: "test-secret",
		}
		client, err := clientcli.New(cfg)
		require.NoError(t, err)

		_, err = client.Upload(context.Background(), clientcli.UploadOptions{
			LocalPath:  localPath,
			RemotePath: "test/file.txt",
		})
		assert.Error(t, err)
	})
}

func TestClient_Download(t *testing.T) {
	t.Run("successful download to file", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)

			w.Header().Set("ETag", `"etag123"`)
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("downloaded content"))
		}))
		defer server.Close()

		cfg := &clientcli.Config{
			Server:    server.URL,
			AccessKey: "test-key",
			SecretKey: "test-secret",
		}
		client, err := clientcli.New(cfg)
		require.NoError(t, err)

		tmpDir := t.TempDir()
		localPath := filepath.Join(tmpDir, "downloaded.txt")

		result, reader, err := client.Download(context.Background(), clientcli.DownloadOptions{
			RemotePath: "test/file.txt",
			LocalPath:  localPath,
		})
		require.NoError(t, err)
		assert.Nil(t, reader)
		assert.Equal(t, "etag123", result.ETag)
		assert.Equal(t, "text/plain", result.ContentType)

		// Verify file content
		content, err := os.ReadFile(localPath)
		require.NoError(t, err)
		assert.Equal(t, "downloaded content", string(content))
	})

	t.Run("download to stdout returns reader", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("ETag", `"etag123"`)
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("stdout content"))
		}))
		defer server.Close()

		cfg := &clientcli.Config{
			Server:    server.URL,
			AccessKey: "test-key",
			SecretKey: "test-secret",
		}
		client, err := clientcli.New(cfg)
		require.NoError(t, err)

		result, reader, err := client.Download(context.Background(), clientcli.DownloadOptions{
			RemotePath: "test/file.txt",
			LocalPath:  "-",
		})
		require.NoError(t, err)
		require.NotNil(t, reader)
		defer func() { _ = reader.Close() }()

		assert.Equal(t, "-", result.LocalPath)

		content, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, "stdout content", string(content))
	})

	t.Run("download 404 error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "not_found", "message": "Object not found"}`))
		}))
		defer server.Close()

		cfg := &clientcli.Config{
			Server:    server.URL,
			AccessKey: "test-key",
			SecretKey: "test-secret",
		}
		client, err := clientcli.New(cfg)
		require.NoError(t, err)

		_, _, err = client.Download(context.Background(), clientcli.DownloadOptions{
			RemotePath: "nonexistent/file.txt",
			LocalPath:  "-",
		})
		assert.Error(t, err)
	})
}

func TestClient_Delete(t *testing.T) {
	t.Run("successful delete", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		cfg := &clientcli.Config{
			Server:    server.URL,
			AccessKey: "test-key",
			SecretKey: "test-secret",
		}
		client, err := clientcli.New(cfg)
		require.NoError(t, err)

		results, err := client.Delete(context.Background(), clientcli.DeleteOptions{
			Paths: []string{"test/file.txt"},
		})
		require.NoError(t, err)
		require.Len(t, results, 1)

		assert.Equal(t, "test/file.txt", results[0].Path)
		assert.True(t, results[0].Deleted)
		assert.Nil(t, results[0].Err)
	})

	t.Run("delete not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "not_found"}`))
		}))
		defer server.Close()

		cfg := &clientcli.Config{
			Server:    server.URL,
			AccessKey: "test-key",
			SecretKey: "test-secret",
		}
		client, err := clientcli.New(cfg)
		require.NoError(t, err)

		results, err := client.Delete(context.Background(), clientcli.DeleteOptions{
			Paths: []string{"nonexistent.txt"},
		})
		require.NoError(t, err)
		require.Len(t, results, 1)

		assert.False(t, results[0].Deleted)
		assert.NotNil(t, results[0].Err)
	})

	t.Run("empty paths error", func(t *testing.T) {
		cfg := &clientcli.Config{
			Server:    "http://localhost",
			AccessKey: "test-key",
			SecretKey: "test-secret",
		}
		client, err := clientcli.New(cfg)
		require.NoError(t, err)

		_, err = client.Delete(context.Background(), clientcli.DeleteOptions{
			Paths: []string{},
		})
		assert.Error(t, err)
	})
}

func TestClient_List(t *testing.T) {
	t.Run("successful list", func(t *testing.T) {
		id1 := uuid.New()
		id2 := uuid.New()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/", r.URL.Path)

			resp := map[string]any{
				"items": []map[string]any{
					{
						"id":              id1.String(),
						"path":            "file1.txt",
						"content_type":    "text/plain",
						"etag":            "etag1",
						"file_size_bytes": 100,
						"created_at":      time.Now().Format(time.RFC3339),
						"updated_at":      time.Now().Format(time.RFC3339),
					},
					{
						"id":              id2.String(),
						"path":            "file2.txt",
						"content_type":    "text/plain",
						"etag":            "etag2",
						"file_size_bytes": 200,
						"created_at":      time.Now().Format(time.RFC3339),
						"updated_at":      time.Now().Format(time.RFC3339),
					},
				},
				"next_cursor": "cursor123",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		cfg := &clientcli.Config{
			Server:    server.URL,
			AccessKey: "test-key",
			SecretKey: "test-secret",
		}
		client, err := clientcli.New(cfg)
		require.NoError(t, err)

		result, err := client.List(context.Background(), clientcli.ListOptions{
			Limit: 100,
		})
		require.NoError(t, err)

		assert.Len(t, result.Items, 2)
		assert.Equal(t, "file1.txt", result.Items[0].Path)
		assert.Equal(t, "file2.txt", result.Items[1].Path)
		assert.Equal(t, "cursor123", result.NextCursor)
		assert.Equal(t, int64(300), result.TotalSize())
	})

	t.Run("list with prefix", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "images/", r.URL.Query().Get("prefix"))

			resp := map[string]any{
				"items": []map[string]any{},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		cfg := &clientcli.Config{
			Server:    server.URL,
			AccessKey: "test-key",
			SecretKey: "test-secret",
		}
		client, err := clientcli.New(cfg)
		require.NoError(t, err)

		_, err = client.List(context.Background(), clientcli.ListOptions{
			Prefix: "images/",
		})
		require.NoError(t, err)
	})
}

func TestHasDeleteErrors(t *testing.T) {
	t.Run("no errors", func(t *testing.T) {
		results := []clientcli.DeleteResult{
			{Path: "a.txt", Deleted: true},
			{Path: "b.txt", Deleted: true},
		}
		assert.False(t, clientcli.HasDeleteErrors(results))
	})

	t.Run("has errors", func(t *testing.T) {
		results := []clientcli.DeleteResult{
			{Path: "a.txt", Deleted: true},
			{Path: "b.txt", Deleted: false, Err: assert.AnError},
		}
		assert.True(t, clientcli.HasDeleteErrors(results))
	})

	t.Run("empty results", func(t *testing.T) {
		results := []clientcli.DeleteResult{}
		assert.False(t, clientcli.HasDeleteErrors(results))
	})
}

func TestNormalizeLocalToRemotePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple file", "file.txt", "file.txt"},
		{"with leading dot slash", "./file.txt", "file.txt"},
		{"nested with dot slash", "./images/photo.jpg", "images/photo.jpg"},
		{"absolute path", "/abs/path/file.txt", "abs/path/file.txt"},
		{"parent traversal", "../sibling/file.txt", "sibling/file.txt"},
		{"multiple parent traversal", "../../other/file.txt", "other/file.txt"},
		{"mixed traversal", "./foo/../bar/file.txt", "bar/file.txt"},
		{"deep nested", "./a/b/c/d/file.txt", "a/b/c/d/file.txt"},
		{"just dot", ".", ""},
		{"just double dot", "..", ""},
		{"trailing slash directory", "./images/", "images"},
		{"nested directory no slash", "./path/to/dir", "path/to/dir"},
		{"absolute with trailing slash", "/abs/path/", "abs/path"},
		{"parent then nested", "../foo/bar/baz.txt", "foo/bar/baz.txt"},
		{"current dir reference", "./foo/./bar/file.txt", "foo/bar/file.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clientcli.NormalizeLocalToRemotePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
