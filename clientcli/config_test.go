package clientcli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sagarc03/stowry/clientcli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &clientcli.Config{Server: "http://localhost:5708"}
		err := cfg.Validate()
		assert.NoError(t, err)
		assert.Equal(t, "http://localhost:5708", cfg.Server)
	})

	t.Run("empty server gets default", func(t *testing.T) {
		cfg := &clientcli.Config{}
		err := cfg.Validate()
		assert.NoError(t, err)
		assert.Equal(t, clientcli.DefaultServer, cfg.Server)
	})
}

func TestConfig_ValidateWithAuth(t *testing.T) {
	t.Run("valid config with auth", func(t *testing.T) {
		cfg := &clientcli.Config{
			Server:    "http://localhost:5708",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
		}
		err := cfg.ValidateWithAuth()
		assert.NoError(t, err)
	})

	t.Run("empty server gets default with auth", func(t *testing.T) {
		cfg := &clientcli.Config{
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
		}
		err := cfg.ValidateWithAuth()
		assert.NoError(t, err)
		assert.Equal(t, clientcli.DefaultServer, cfg.Server)
	})

	t.Run("missing access key", func(t *testing.T) {
		cfg := &clientcli.Config{
			Server:    "http://localhost:5708",
			SecretKey: "test-secret-key",
		}
		err := cfg.ValidateWithAuth()
		assert.Error(t, err)
	})

	t.Run("missing secret key", func(t *testing.T) {
		cfg := &clientcli.Config{
			Server:    "http://localhost:5708",
			AccessKey: "test-access-key",
		}
		err := cfg.ValidateWithAuth()
		assert.Error(t, err)
	})
}

func TestLoadConfigFromFile(t *testing.T) {
	t.Run("valid config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		content := `server: http://localhost:5708
access_key: test-access
secret_key: test-secret
`
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		cfg, err := clientcli.LoadConfigFromFile(configPath)
		require.NoError(t, err)

		assert.Equal(t, "http://localhost:5708", cfg.Server)
		assert.Equal(t, "test-access", cfg.AccessKey)
		assert.Equal(t, "test-secret", cfg.SecretKey)
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := clientcli.LoadConfigFromFile("/nonexistent/path/config.yaml")
		assert.Error(t, err)
	})

	t.Run("invalid yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		content := `invalid: [yaml: content`
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		_, err = clientcli.LoadConfigFromFile(configPath)
		assert.Error(t, err)
	})
}

func TestMergeConfig(t *testing.T) {
	tests := []struct {
		name     string
		configs  []*clientcli.Config
		expected *clientcli.Config
	}{
		{
			name:     "empty configs",
			configs:  []*clientcli.Config{},
			expected: &clientcli.Config{},
		},
		{
			name: "single config",
			configs: []*clientcli.Config{
				{Server: "http://a.com", AccessKey: "key1", SecretKey: "secret1"},
			},
			expected: &clientcli.Config{Server: "http://a.com", AccessKey: "key1", SecretKey: "secret1"},
		},
		{
			name: "later config overrides",
			configs: []*clientcli.Config{
				{Server: "http://a.com", AccessKey: "key1", SecretKey: "secret1"},
				{Server: "http://b.com", AccessKey: "key2"},
			},
			expected: &clientcli.Config{Server: "http://b.com", AccessKey: "key2", SecretKey: "secret1"},
		},
		{
			name: "empty strings do not override",
			configs: []*clientcli.Config{
				{Server: "http://a.com", AccessKey: "key1", SecretKey: "secret1"},
				{Server: "", AccessKey: "", SecretKey: ""},
			},
			expected: &clientcli.Config{Server: "http://a.com", AccessKey: "key1", SecretKey: "secret1"},
		},
		{
			name: "nil config is skipped",
			configs: []*clientcli.Config{
				{Server: "http://a.com"},
				nil,
				{AccessKey: "key2"},
			},
			expected: &clientcli.Config{Server: "http://a.com", AccessKey: "key2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clientcli.MergeConfig(tt.configs...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfigFromEnv(t *testing.T) {
	// Save original values
	origServer := os.Getenv("STOWRY_SERVER")
	origAccessKey := os.Getenv("STOWRY_ACCESS_KEY")
	origSecretKey := os.Getenv("STOWRY_SECRET_KEY")

	// Restore after test
	t.Cleanup(func() {
		_ = os.Setenv("STOWRY_SERVER", origServer)
		_ = os.Setenv("STOWRY_ACCESS_KEY", origAccessKey)
		_ = os.Setenv("STOWRY_SECRET_KEY", origSecretKey)
	})

	// Set test values
	_ = os.Setenv("STOWRY_SERVER", "http://test.example.com")
	_ = os.Setenv("STOWRY_ACCESS_KEY", "env-access-key")
	_ = os.Setenv("STOWRY_SECRET_KEY", "env-secret-key")

	cfg := clientcli.ConfigFromEnv()

	assert.Equal(t, "http://test.example.com", cfg.Server)
	assert.Equal(t, "env-access-key", cfg.AccessKey)
	assert.Equal(t, "env-secret-key", cfg.SecretKey)
}
