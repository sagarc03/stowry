package e2e_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/sagarc03/stowry"
	stowryclient "github.com/sagarc03/stowry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_BasicCRUD_SQLite tests the full CRUD lifecycle using SQLite.
func TestE2E_BasicCRUD_SQLite(t *testing.T) {
	storageDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	baseURL, cleanup := startServer(t, ServerConfig{
		Port:        getOpenPort(t),
		Mode:        "store",
		DBType:      "sqlite",
		DBDSN:       dbPath,
		StoragePath: storageDir,
		AuthRead:    "public",
		AuthWrite:   "public",
	})
	defer cleanup()

	runBasicCRUDTests(t, baseURL)
}

// TestE2E_BasicCRUD_Postgres tests the full CRUD lifecycle using PostgreSQL.
func TestE2E_BasicCRUD_Postgres(t *testing.T) {
	dsn := getSharedPostgresDatabase(t)
	storageDir := t.TempDir()

	baseURL, cleanup := startServer(t, ServerConfig{
		Port:        getOpenPort(t),
		Mode:        "store",
		DBType:      "postgres",
		DBDSN:       dsn,
		StoragePath: storageDir,
		AuthRead:    "public",
		AuthWrite:   "public",
	})
	defer cleanup()

	runBasicCRUDTests(t, baseURL)
}

// runBasicCRUDTests contains the shared CRUD test logic.
func runBasicCRUDTests(t *testing.T, baseURL string) {
	t.Helper()
	client := &http.Client{}

	t.Run("PUT creates text.txt", func(t *testing.T) {
		content := []byte("Hello, World!")
		req, err := http.NewRequest("PUT", baseURL+"/test.txt", bytes.NewReader(content))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var metadata stowry.MetaData
		err = json.NewDecoder(resp.Body).Decode(&metadata)
		require.NoError(t, err)
		assert.Equal(t, "test.txt", metadata.Path)
		assert.Equal(t, "text/plain", metadata.ContentType)
		assert.NotEmpty(t, metadata.Etag)
	})

	t.Run("GET returns file text.txt content", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/test.txt")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))
		assert.NotEmpty(t, resp.Header.Get("ETag"))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "Hello, World!", string(body))
	})

	t.Run("HEAD returns metadata for test.txt", func(t *testing.T) {
		req, err := http.NewRequest("HEAD", baseURL+"/test.txt", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))
		assert.NotEmpty(t, resp.Header.Get("ETag"))
		assert.Equal(t, "13", resp.Header.Get("Content-Length"))
		assert.Equal(t, "bytes", resp.Header.Get("Accept-Ranges"))
		assert.NotEmpty(t, resp.Header.Get("Last-Modified"))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Empty(t, body)
	})

	t.Run("DELETE removes test.txt", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", baseURL+"/test.txt", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	t.Run("GET returns 404 after delete", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/test.txt")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("HEAD returns 404 after delete", func(t *testing.T) {
		req, err := http.NewRequest("HEAD", baseURL+"/test.txt", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("HEAD returns 404 for nonexistent file", func(t *testing.T) {
		req, err := http.NewRequest("HEAD", baseURL+"/does-not-exist.txt", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

// TestE2E_List_SQLite tests listing files using SQLite.
func TestE2E_List_SQLite(t *testing.T) {
	storageDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	baseURL, cleanup := startServer(t, ServerConfig{
		Port:        getOpenPort(t),
		Mode:        "store",
		DBType:      "sqlite",
		DBDSN:       dbPath,
		StoragePath: storageDir,
		AuthRead:    "public",
		AuthWrite:   "public",
	})
	defer cleanup()

	runListTests(t, baseURL)
}

// runListTests contains the shared list test logic.
func runListTests(t *testing.T, baseURL string) {
	t.Helper()
	client := &http.Client{}

	files := []string{"file1.txt", "file2.txt", "docs/readme.md"}
	for _, file := range files {
		req, err := http.NewRequest("PUT", baseURL+"/"+file, bytes.NewReader([]byte("content")))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")

		resp, err := client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	t.Run("GET / lists all files", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result stowry.ListResult
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(result.Items), 3)
	})

	t.Run("GET /?prefix=docs/ filters by prefix", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/?prefix=docs/")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result stowry.ListResult
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, 1, len(result.Items))
		assert.Equal(t, "docs/readme.md", result.Items[0].Path)
	})

	t.Run("GET /?limit=1 limits results", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/?limit=1")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result stowry.ListResult
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, 1, len(result.Items))
		assert.NotEmpty(t, result.NextCursor)
	})
}

// TestE2E_ConditionalRequests_SQLite tests If-Match and If-None-Match headers.
func TestE2E_ConditionalRequests_SQLite(t *testing.T) {
	storageDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	baseURL, cleanup := startServer(t, ServerConfig{
		Port:        getOpenPort(t),
		Mode:        "store",
		DBType:      "sqlite",
		DBDSN:       dbPath,
		StoragePath: storageDir,
		AuthRead:    "public",
		AuthWrite:   "public",
	})
	defer cleanup()

	runConditionalRequestsTests(t, baseURL)
}

// runConditionalRequestsTests contains the shared conditional requests test logic.
func runConditionalRequestsTests(t *testing.T, baseURL string) {
	t.Helper()
	client := &http.Client{}

	var etag string
	t.Run("PUT creates file", func(t *testing.T) {
		req, err := http.NewRequest("PUT", baseURL+"/conditional.txt", bytes.NewReader([]byte("initial content")))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var metadata stowry.MetaData
		err = json.NewDecoder(resp.Body).Decode(&metadata)
		require.NoError(t, err)
		etag = metadata.Etag
		require.NotEmpty(t, etag)
	})

	t.Run("PUT with correct If-Match succeeds", func(t *testing.T) {
		req, err := http.NewRequest("PUT", baseURL+"/conditional.txt", bytes.NewReader([]byte("updated content")))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("If-Match", etag)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Update etag for next test
		var metadata stowry.MetaData
		err = json.NewDecoder(resp.Body).Decode(&metadata)
		require.NoError(t, err)
		etag = metadata.Etag
	})

	t.Run("PUT with wrong If-Match fails with 412", func(t *testing.T) {
		req, err := http.NewRequest("PUT", baseURL+"/conditional.txt", bytes.NewReader([]byte("conflict content")))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("If-Match", "wrong-etag")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
	})

	t.Run("GET with matching If-None-Match returns 304", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/conditional.txt", nil)
		require.NoError(t, err)
		req.Header.Set("If-None-Match", `"`+etag+`"`)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotModified, resp.StatusCode)
	})

	t.Run("GET with non-matching If-None-Match returns content", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/conditional.txt", nil)
		require.NoError(t, err)
		req.Header.Set("If-None-Match", `"different-etag"`)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("HEAD with matching If-None-Match returns 304", func(t *testing.T) {
		req, err := http.NewRequest("HEAD", baseURL+"/conditional.txt", nil)
		require.NoError(t, err)
		req.Header.Set("If-None-Match", `"`+etag+`"`)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotModified, resp.StatusCode)
		assert.Equal(t, `"`+etag+`"`, resp.Header.Get("ETag"))
	})

	t.Run("HEAD with non-matching If-None-Match returns 200", func(t *testing.T) {
		req, err := http.NewRequest("HEAD", baseURL+"/conditional.txt", nil)
		require.NoError(t, err)
		req.Header.Set("If-None-Match", `"different-etag"`)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("PUT with If-Match on nonexistent file returns 412", func(t *testing.T) {
		req, err := http.NewRequest("PUT", baseURL+"/no-such-file.txt", bytes.NewReader([]byte("content")))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("If-Match", `"any-etag"`)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
	})

	t.Run("PUT with If-Match wildcard on nonexistent file returns 412", func(t *testing.T) {
		req, err := http.NewRequest("PUT", baseURL+"/no-such-file.txt", bytes.NewReader([]byte("content")))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("If-Match", `*`)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
	})
}

// TestE2E_StaticMode_SQLite tests static file serving mode.
func TestE2E_StaticMode_SQLite(t *testing.T) {
	storageDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	cfg := ServerConfig{
		Port:        getOpenPort(t),
		Mode:        "static",
		DBType:      "sqlite",
		DBDSN:       dbPath,
		StoragePath: storageDir,
		AuthRead:    "public",
		AuthWrite:   "public",
	}

	// Seed files before starting server (PUT not available in static mode)
	initDatabase(t, cfg)
	indexContent := []byte("<html><body>Hello from index.html</body></html>")
	seedFile(t, cfg, "docs/index.html", indexContent)

	baseURL, cleanup := startServer(t, cfg)
	defer cleanup()

	runStaticModeTests(t, baseURL, indexContent)
}

// runStaticModeTests contains the shared static mode test logic.
func runStaticModeTests(t *testing.T, baseURL string, indexContent []byte) {
	t.Helper()
	client := &http.Client{}

	t.Run("GET /docs returns index.html content", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/docs")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, string(indexContent), string(body))
	})

	t.Run("GET /docs/index.html returns content directly", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/docs/index.html")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, string(indexContent), string(body))
	})

	t.Run("PUT returns 405", func(t *testing.T) {
		req, err := http.NewRequest("PUT", baseURL+"/new.txt", bytes.NewReader([]byte("content")))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})

	t.Run("DELETE returns 405", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", baseURL+"/docs/index.html", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})
}

// TestE2E_SPAMode_SQLite tests SPA (single page app) mode.
func TestE2E_SPAMode_SQLite(t *testing.T) {
	storageDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	cfg := ServerConfig{
		Port:        getOpenPort(t),
		Mode:        "spa",
		DBType:      "sqlite",
		DBDSN:       dbPath,
		StoragePath: storageDir,
		AuthRead:    "public",
		AuthWrite:   "public",
	}

	// Seed files before starting server (PUT not available in SPA mode)
	initDatabase(t, cfg)
	indexContent := []byte("<html><body>SPA Root</body></html>")
	realContent := []byte("real file content")
	seedFile(t, cfg, "index.html", indexContent)
	seedFile(t, cfg, "real.txt", realContent)

	baseURL, cleanup := startServer(t, cfg)
	defer cleanup()

	runSPAModeTests(t, baseURL, indexContent, realContent)
}

// runSPAModeTests contains the shared SPA mode test logic.
func runSPAModeTests(t *testing.T, baseURL string, indexContent, realContent []byte) {
	t.Helper()
	client := &http.Client{}

	t.Run("GET /nonexistent returns /index.html content", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/nonexistent/path")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, string(indexContent), string(body))
	})

	t.Run("GET / returns index.html content", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, string(indexContent), string(body))
	})

	t.Run("GET /real.txt returns actual file content", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/real.txt")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/plain")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, string(realContent), string(body))
	})

	t.Run("PUT returns 405", func(t *testing.T) {
		req, err := http.NewRequest("PUT", baseURL+"/new.txt", bytes.NewReader([]byte("content")))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})

	t.Run("DELETE returns 405", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", baseURL+"/real.txt", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})
}

// Test credentials for auth tests.
const (
	testAccessKey = "AKIAIOSFODNN7EXAMPLE"
	testSecretKey = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
)

// TestE2E_Auth_PrivateWrite tests authentication for write operations.
func TestE2E_Auth_PrivateWrite(t *testing.T) {
	storageDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	baseURL, cleanup := startServer(t, ServerConfig{
		Port:        getOpenPort(t),
		Mode:        "store",
		DBType:      "sqlite",
		DBDSN:       dbPath,
		StoragePath: storageDir,
		AuthRead:    "public",
		AuthWrite:   "private",
		AuthKeys: []AuthKey{
			{AccessKey: testAccessKey, SecretKey: testSecretKey},
		},
	})
	defer cleanup()

	httpClient := &http.Client{}

	t.Run("PUT without auth returns 401", func(t *testing.T) {
		req, err := http.NewRequest("PUT", baseURL+"/test.txt", bytes.NewReader([]byte("content")))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("DELETE without auth returns 401", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", baseURL+"/test.txt", nil)
		require.NoError(t, err)

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("PUT with presigned URL succeeds", func(t *testing.T) {
		client := stowryclient.NewClient(baseURL, testAccessKey, testSecretKey)
		presignedURL := client.PresignPut("/auth-test.txt", 900)

		req, err := http.NewRequest("PUT", presignedURL, bytes.NewReader([]byte("authenticated content")))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("GET (public read) works without auth", func(t *testing.T) {
		resp, err := httpClient.Get(baseURL + "/auth-test.txt")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "authenticated content", string(body))
	})

	t.Run("DELETE with presigned URL succeeds", func(t *testing.T) {
		client := stowryclient.NewClient(baseURL, testAccessKey, testSecretKey)
		presignedURL := client.PresignDelete("/auth-test.txt", 900)

		req, err := http.NewRequest("DELETE", presignedURL, nil)
		require.NoError(t, err)

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})
}

// TestE2E_Auth_PrivateRead tests authentication for read operations.
func TestE2E_Auth_PrivateRead(t *testing.T) {
	storageDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	baseURL, cleanup := startServer(t, ServerConfig{
		Port:        getOpenPort(t),
		Mode:        "store",
		DBType:      "sqlite",
		DBDSN:       dbPath,
		StoragePath: storageDir,
		AuthRead:    "private",
		AuthWrite:   "private",
		AuthKeys: []AuthKey{
			{AccessKey: testAccessKey, SecretKey: testSecretKey},
		},
	})
	defer cleanup()

	httpClient := &http.Client{}
	client := stowryclient.NewClient(baseURL, testAccessKey, testSecretKey)

	t.Run("PUT with presigned URL creates file", func(t *testing.T) {
		presignedURL := client.PresignPut("/private-file.txt", 900)

		req, err := http.NewRequest("PUT", presignedURL, bytes.NewReader([]byte("private content")))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("GET without auth returns 401", func(t *testing.T) {
		resp, err := httpClient.Get(baseURL + "/private-file.txt")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("HEAD without auth returns 401", func(t *testing.T) {
		req, err := http.NewRequest("HEAD", baseURL+"/private-file.txt", nil)
		require.NoError(t, err)

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("GET / (list) without auth returns 401", func(t *testing.T) {
		resp, err := httpClient.Get(baseURL + "/")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("GET with presigned URL succeeds", func(t *testing.T) {
		presignedURL := client.PresignGet("/private-file.txt", 900)

		resp, err := httpClient.Get(presignedURL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "private content", string(body))
	})
}

// TestE2E_Auth_InvalidSignature tests that invalid signatures are rejected.
func TestE2E_Auth_InvalidSignature(t *testing.T) {
	storageDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	baseURL, cleanup := startServer(t, ServerConfig{
		Port:        getOpenPort(t),
		Mode:        "store",
		DBType:      "sqlite",
		DBDSN:       dbPath,
		StoragePath: storageDir,
		AuthRead:    "public",
		AuthWrite:   "private",
		AuthKeys: []AuthKey{
			{AccessKey: testAccessKey, SecretKey: testSecretKey},
		},
	})
	defer cleanup()

	httpClient := &http.Client{}

	t.Run("PUT with wrong secret key returns 401", func(t *testing.T) {
		badClient := stowryclient.NewClient(baseURL, testAccessKey, "wrong-secret-key")
		presignedURL := badClient.PresignPut("/test.txt", 900)

		req, err := http.NewRequest("PUT", presignedURL, bytes.NewReader([]byte("content")))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("PUT with unknown access key returns 401", func(t *testing.T) {
		badClient := stowryclient.NewClient(baseURL, "UNKNOWN_ACCESS_KEY", "some-secret")
		presignedURL := badClient.PresignPut("/test.txt", 900)

		req, err := http.NewRequest("PUT", presignedURL, bytes.NewReader([]byte("content")))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")

		resp, err := httpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
