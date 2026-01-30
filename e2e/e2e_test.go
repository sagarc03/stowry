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
}

// TestE2E_StaticMode_SQLite tests static file serving mode.
func TestE2E_StaticMode_SQLite(t *testing.T) {
	storageDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	baseURL, cleanup := startServer(t, ServerConfig{
		Port:        getOpenPort(t),
		Mode:        "static",
		DBType:      "sqlite",
		DBDSN:       dbPath,
		StoragePath: storageDir,
		AuthRead:    "public",
		AuthWrite:   "public",
	})
	defer cleanup()

	runStaticModeTests(t, baseURL)
}

// runStaticModeTests contains the shared static mode test logic.
func runStaticModeTests(t *testing.T, baseURL string) {
	t.Helper()
	client := &http.Client{}

	indexContent := []byte("<html><body>Hello from index.html</body></html>")
	t.Run("PUT dir/index.html creates file", func(t *testing.T) {
		req, err := http.NewRequest("PUT", baseURL+"/docs/index.html", bytes.NewReader(indexContent))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/html")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("GET /docs returns index.html content", func(t *testing.T) {
		// Note: trailing slash is not valid - use /docs not /docs/
		resp, err := client.Get(baseURL + "/docs")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/html", resp.Header.Get("Content-Type"))

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
}

// TestE2E_SPAMode_SQLite tests SPA (single page app) mode.
func TestE2E_SPAMode_SQLite(t *testing.T) {
	storageDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	baseURL, cleanup := startServer(t, ServerConfig{
		Port:        getOpenPort(t),
		Mode:        "spa",
		DBType:      "sqlite",
		DBDSN:       dbPath,
		StoragePath: storageDir,
		AuthRead:    "public",
		AuthWrite:   "public",
	})
	defer cleanup()

	runSPAModeTests(t, baseURL)
}

// runSPAModeTests contains the shared SPA mode test logic.
func runSPAModeTests(t *testing.T, baseURL string) {
	t.Helper()
	client := &http.Client{}

	indexContent := []byte("<html><body>SPA Root</body></html>")
	t.Run("PUT /index.html creates file", func(t *testing.T) {
		req, err := http.NewRequest("PUT", baseURL+"/index.html", bytes.NewReader(indexContent))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/html")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("GET /nonexistent returns /index.html content", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/nonexistent/path")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/html", resp.Header.Get("Content-Type"))

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

	realContent := []byte("real file content")
	t.Run("PUT /real.txt creates file", func(t *testing.T) {
		req, err := http.NewRequest("PUT", baseURL+"/real.txt", bytes.NewReader(realContent))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "text/plain")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("GET /real.txt returns actual file content", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/real.txt")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, string(realContent), string(body))
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
