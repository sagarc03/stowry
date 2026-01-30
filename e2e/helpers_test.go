package e2e_test

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
	Port        int
	Mode        string // store, static, spa
	DBType      string // sqlite, postgres
	DBDSN       string
	StoragePath string
	AuthRead    string    // public, private
	AuthWrite   string    // public, private
	AuthKeys    []AuthKey // Access keys for private auth
}

// buildBinary compiles the stowry binary once per test run.
// Returns the path to the compiled binary.
func buildBinary(t *testing.T) string {
	t.Helper()

	binaryOnce.Do(func() {
		binaryPath = filepath.Join(sharedTempDir, "stowry")

		cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/stowry")
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

// initDatabase runs the init command to migrate and set up the database.
func initDatabase(t *testing.T, cfg ServerConfig) {
	t.Helper()

	binary := buildBinary(t)

	// Create a minimal config file for init
	initConfig := fmt.Sprintf(`database:
  type: %s
  dsn: "%s"
storage:
  path: "%s"
log:
  level: error
`, cfg.DBType, cfg.DBDSN, cfg.StoragePath)

	configPath := filepath.Join(t.TempDir(), "init-config.yaml")
	err := os.WriteFile(configPath, []byte(initConfig), 0o600)
	require.NoError(t, err, "write init config file")

	cmd := exec.Command(binary, "init", "--config", configPath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "init database: %s", output)
}

// createConfigFile creates a temporary config file for the server.
// Returns the path to the config file.
func createConfigFile(t *testing.T, cfg ServerConfig) string {
	t.Helper()

	var sb strings.Builder
	fmt.Fprintf(&sb, `server:
  port: %d
  mode: %s

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
		cfg.DBType,
		cfg.DBDSN,
		cfg.StoragePath,
		cfg.AuthRead,
		cfg.AuthWrite,
	)

	// Add auth keys if provided
	if len(cfg.AuthKeys) > 0 {
		sb.WriteString("  keys:\n    inline:\n")
		for _, key := range cfg.AuthKeys {
			fmt.Fprintf(&sb, "      - access_key: %s\n        secret_key: %s\n", key.AccessKey, key.SecretKey)
		}
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

	// Initialize the database before starting the server
	initDatabase(t, cfg)

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
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			_ = cmd.Wait()
		}
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
