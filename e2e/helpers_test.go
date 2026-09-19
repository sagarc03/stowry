package e2e_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

// seedFile puts content at destPath in the storage directory and records it,
// which is how objects get in without going through the server. Static and spa
// modes route no writes at all, so this is the only way there.
//
// The schema must already exist: call migrateDatabase first.
func seedFile(t *testing.T, cfg ServerConfig, destPath string, content []byte) {
	t.Helper()

	full := filepath.Join(cfg.StoragePath, filepath.FromSlash(destPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750), "create storage subdirectory")
	require.NoError(t, os.WriteFile(full, content, 0o600), "write seed file")

	populateStorage(t, cfg)
}

// populateStorage records every file already in the storage directory.
func populateStorage(t *testing.T, cfg ServerConfig) {
	t.Helper()

	binary := buildBinary(t)

	config := fmt.Sprintf("database:\n  type: %s\n  dsn: \"%s\"\nstorage:\n  path: \"%s\"\nlog:\n  level: error\n",
		cfg.DBType, yamlPath(cfg.DBDSN), yamlPath(cfg.StoragePath))

	configPath := filepath.Join(t.TempDir(), "populate-config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600), "write populate config")

	output, err := exec.Command(binary, "populate", "--config", configPath).CombinedOutput()
	require.NoError(t, err, "populate storage: %s", output)
}

var (
	binaryPath     string
	binaryBuildErr error
	binaryOnce     sync.Once
	sharedTempDir  string
)

// TestMain sets up and tears down shared test resources.
func TestMain(m *testing.M) {
	// Create shared temp directory for the binary
	var err error
	sharedTempDir, err = os.MkdirTemp("", "stowry-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup shared temp directory
	_ = os.RemoveAll(sharedTempDir)

	os.Exit(code)
}

// AuthKey represents an access key pair for authentication.
type AuthKey struct {
	AccessKey string
	SecretKey string
}

// ServerConfig holds configuration for starting the stowry server.
type ServerConfig struct {
	Port          int
	Mode          string // store, static, spa
	DBType        string // sqlite, postgres
	DBDSN         string
	StoragePath   string
	AuthRead      string    // public, private
	AuthWrite     string    // public, private
	AuthKeys      []AuthKey // Access keys for private auth
	ErrorDocument string    // Custom error page path (optional)
}

// buildBinary compiles the stowry binary once per test run.
// Returns the path to the compiled binary.
func buildBinary(t *testing.T) string {
	t.Helper()

	binaryOnce.Do(func() {
		// go build appends .exe on Windows, and exec will not find the binary
		// without it.
		name := "stowry"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}

		binaryPath = filepath.Join(sharedTempDir, name)

		cmd := exec.Command("go", "build", "-o", binaryPath, ".")
		cmd.Dir = getProjectRoot(t)
		output, err := cmd.CombinedOutput()
		if err != nil {
			binaryBuildErr = fmt.Errorf("build binary: %w\nOutput: %s", err, output)
			return
		}
	})

	if binaryBuildErr != nil {
		t.Fatalf("failed to build binary: %v", binaryBuildErr)
	}

	return binaryPath
}

// requireLinuxContainers skips unless this host can run the Linux image the
// PostgreSQL tests need.
//
// Only the Linux runners can. GitHub's macOS runners ship no Docker at all, and
// its Windows runners are already nested one level deep, so the hypervisor
// cannot give Docker the nested virtualization a Linux container would need -
// their daemon answers, but only for Windows containers.
func requireLinuxContainers(t *testing.T) {
	t.Helper()

	if runtime.GOOS != "linux" {
		t.Skipf("no Linux containers on %s", runtime.GOOS)
	}

	if _, err := testcontainers.NewDockerClientWithOpts(context.Background()); err != nil {
		t.Skipf("no container runtime: %v", err)
	}
}

// yamlPath makes a path safe to embed in a double-quoted YAML scalar, where a
// Windows backslash would be read as an escape. Both Go and SQLite accept
// forward slashes on Windows.
func yamlPath(p string) string {
	return filepath.ToSlash(p)
}

// getProjectRoot returns the root directory of the stowry project.
func getProjectRoot(t *testing.T) string {
	t.Helper()

	// Find the go.mod file to determine project root
	dir, err := os.Getwd()
	require.NoError(t, err, "get working directory")

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}

// migrateDatabase creates the schema the server expects.
func migrateDatabase(t *testing.T, cfg ServerConfig) {
	t.Helper()

	binary := buildBinary(t)

	migrateConfig := fmt.Sprintf(`database:
  type: %s
  dsn: "%s"
storage:
  path: "%s"
log:
  level: error
`, cfg.DBType, yamlPath(cfg.DBDSN), yamlPath(cfg.StoragePath))

	configPath := filepath.Join(t.TempDir(), "migrate-config.yaml")
	err := os.WriteFile(configPath, []byte(migrateConfig), 0o600)
	require.NoError(t, err, "write migrate config file")

	cmd := exec.Command(binary, "migrate", "--config", configPath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "migrate database: %s", output)
}

// createConfigFile creates a temporary config file for the server.
// Returns the path to the config file.
func createConfigFile(t *testing.T, cfg ServerConfig) string {
	t.Helper()

	var sb strings.Builder
	fmt.Fprintf(&sb, `server:
  port: %d
  mode: %s
  error_document: "%s"

database:
  type: %s
  dsn: "%s"

storage:
  path: "%s"

auth:
  read: %s
  write: %s
  aws:
    region: us-east-1
    service: s3
`,
		cfg.Port,
		cfg.Mode,
		cfg.ErrorDocument,
		cfg.DBType,
		yamlPath(cfg.DBDSN),
		yamlPath(cfg.StoragePath),
		cfg.AuthRead,
		cfg.AuthWrite,
	)

	// One pair is all the config carries; several go in a keys file.
	for _, key := range cfg.AuthKeys {
		fmt.Fprintf(&sb, "  access_key: %s\n  secret_key: %s\n", key.AccessKey, key.SecretKey)
	}

	sb.WriteString("\nlog:\n  level: error\n")

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(configPath, []byte(sb.String()), 0o600)
	require.NoError(t, err, "write config file")

	return configPath
}

// startServer starts the stowry binary with the given configuration.
// Returns the base URL and a cleanup function that must be called to stop the server.
func startServer(t *testing.T, cfg ServerConfig) (string, func()) {
	t.Helper()

	migrateDatabase(t, cfg)

	binary := buildBinary(t)

	// Create config file
	configPath := createConfigFile(t, cfg)

	args := []string{
		"serve",
		"--config", configPath,
	}

	cmd := exec.Command(binary, args...)

	// Capture output for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	require.NoError(t, err, "start server")

	baseURL := fmt.Sprintf("http://localhost:%d", cfg.Port)

	// Wait for server to be ready
	waitForServer(t, baseURL, 10*time.Second)

	cleanup := func() {
		if cmd.Process == nil {
			return
		}

		// Windows implements no signal but Kill, so SIGTERM would return an
		// error and leave Wait blocking on a server that never got asked to
		// stop.
		if runtime.GOOS == "windows" {
			_ = cmd.Process.Kill()
		} else {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}

		_ = cmd.Wait()
	}

	return baseURL, cleanup
}

// waitForServer polls the server until it responds or times out.
func waitForServer(t *testing.T, baseURL string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/")
		if err == nil {
			resp.Body.Close()
			return // Server is ready
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("server failed to start within %v", timeout)
}

// getOpenPort finds an available TCP port.
func getOpenPort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err, "find open port")

	addr := l.Addr().(*net.TCPAddr)
	port := addr.Port

	err = l.Close()
	require.NoError(t, err, "close port")

	return port
}
