package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sagarc03/stowry/config"
)

func TestLoad_Defaults(t *testing.T) {
	// Load with no config files should use defaults
	cfg, err := config.Load(nil, nil)
	require.NoError(t, err)

	assert.Equal(t, 5708, cfg.Server.Port)
	assert.Equal(t, "store", cfg.Server.Mode)
	assert.Equal(t, "sqlite", cfg.Database.Type)
	assert.Equal(t, "stowry.db", cfg.Database.DSN)
	assert.Equal(t, "stowry_metadata", cfg.Database.Tables.MetaData)
	assert.Equal(t, "./data", cfg.Storage.Path)
	assert.Equal(t, "public", cfg.Auth.Read)
	assert.Equal(t, "public", cfg.Auth.Write)
	assert.Equal(t, "us-east-1", cfg.Auth.AWS.Region)
	assert.Equal(t, "s3", cfg.Auth.AWS.Service)
	assert.Equal(t, "info", cfg.Log.Level)
}

func TestLoad_ConfigFile(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  port: 8080
  mode: static
database:
  type: postgres
  dsn: postgres://localhost/test
  tables:
    meta_data: custom_table
storage:
  path: /tmp/storage
auth:
  read: private
  write: private
  aws:
    region: eu-west-1
    service: custom
log:
  level: debug
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	cfg, err := config.Load([]string{configPath}, nil)
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "static", cfg.Server.Mode)
	assert.Equal(t, "postgres", cfg.Database.Type)
	assert.Equal(t, "postgres://localhost/test", cfg.Database.DSN)
	assert.Equal(t, "custom_table", cfg.Database.Tables.MetaData)
	assert.Equal(t, "/tmp/storage", cfg.Storage.Path)
	assert.Equal(t, "private", cfg.Auth.Read)
	assert.Equal(t, "private", cfg.Auth.Write)
	assert.Equal(t, "eu-west-1", cfg.Auth.AWS.Region)
	assert.Equal(t, "custom", cfg.Auth.AWS.Service)
	assert.Equal(t, "debug", cfg.Log.Level)
}

func TestLoad_ConfigFileMerge(t *testing.T) {
	tmpDir := t.TempDir()

	// Base config
	basePath := filepath.Join(tmpDir, "base.yaml")
	baseContent := `
server:
  port: 5708
  mode: store
database:
  type: sqlite
  dsn: stowry.db
  tables:
    meta_data: stowry_metadata
storage:
  path: ./data
auth:
  read: public
  write: public
  aws:
    region: us-east-1
    service: s3
log:
  level: info
`
	err := os.WriteFile(basePath, []byte(baseContent), 0o644)
	require.NoError(t, err)

	// Override config
	overridePath := filepath.Join(tmpDir, "override.yaml")
	overrideContent := `
server:
  port: 9000
auth:
  read: private
`
	err = os.WriteFile(overridePath, []byte(overrideContent), 0o644)
	require.NoError(t, err)

	// Load with merge (later files override earlier)
	cfg, err := config.Load([]string{basePath, overridePath}, nil)
	require.NoError(t, err)

	// Overridden values
	assert.Equal(t, 9000, cfg.Server.Port)
	assert.Equal(t, "private", cfg.Auth.Read)

	// Preserved values from base
	assert.Equal(t, "store", cfg.Server.Mode)
	assert.Equal(t, "public", cfg.Auth.Write)
	assert.Equal(t, "sqlite", cfg.Database.Type)
}

func TestLoad_ValidationError_InvalidPort(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  port: 99999
  mode: store
database:
  type: sqlite
  dsn: stowry.db
  tables:
    meta_data: test
storage:
  path: ./data
auth:
  read: public
  write: public
log:
  level: info
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	_, err = config.Load([]string{configPath}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validate config")
}

func TestLoad_ValidationError_InvalidMode(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  port: 5708
  mode: invalid
database:
  type: sqlite
  dsn: stowry.db
  tables:
    meta_data: test
storage:
  path: ./data
auth:
  read: public
  write: public
log:
  level: info
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	_, err = config.Load([]string{configPath}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validate config")
}

func TestLoad_ValidationError_InvalidAuthMode(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  port: 5708
  mode: store
database:
  type: sqlite
  dsn: stowry.db
  tables:
    meta_data: test
storage:
  path: ./data
auth:
  read: invalid
  write: public
log:
  level: info
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	_, err = config.Load([]string{configPath}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validate config")
}

func TestLoad_WithInlineKeys(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  port: 5708
  mode: store
database:
  type: sqlite
  dsn: stowry.db
  tables:
    meta_data: test
storage:
  path: ./data
auth:
  read: private
  write: private
  aws:
    region: us-east-1
    service: s3
  keys:
    inline:
      - access_key: AKIATEST123
        secret_key: secretkey123
      - access_key: AKIATEST456
        secret_key: secretkey456
log:
  level: info
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	cfg, err := config.Load([]string{configPath}, nil)
	require.NoError(t, err)

	require.Len(t, cfg.Auth.Keys.Inline, 2)
	assert.Equal(t, "AKIATEST123", cfg.Auth.Keys.Inline[0].AccessKey)
	assert.Equal(t, "secretkey123", cfg.Auth.Keys.Inline[0].SecretKey)
	assert.Equal(t, "AKIATEST456", cfg.Auth.Keys.Inline[1].AccessKey)
	assert.Equal(t, "secretkey456", cfg.Auth.Keys.Inline[1].SecretKey)
}

func TestLoad_WithCORS(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  port: 5708
  mode: store
database:
  type: sqlite
  dsn: stowry.db
  tables:
    meta_data: test
storage:
  path: ./data
auth:
  read: public
  write: public
log:
  level: info
cors:
  enabled: true
  allowed_origins:
    - https://example.com
    - https://app.example.com
  allowed_methods:
    - GET
    - PUT
  allowed_headers:
    - Content-Type
  max_age: 600
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	cfg, err := config.Load([]string{configPath}, nil)
	require.NoError(t, err)

	assert.True(t, cfg.CORS.Enabled)
	assert.Equal(t, []string{"https://example.com", "https://app.example.com"}, cfg.CORS.AllowedOrigins)
	assert.Equal(t, []string{"GET", "PUT"}, cfg.CORS.AllowedMethods)
	assert.Equal(t, []string{"Content-Type"}, cfg.CORS.AllowedHeaders)
	assert.Equal(t, 600, cfg.CORS.MaxAge)
}

func TestLoad_EnvironmentVariables(t *testing.T) {
	// Set environment variables
	t.Setenv("STOWRY_SERVER_PORT", "9090")
	t.Setenv("STOWRY_DATABASE_TYPE", "postgres")
	t.Setenv("STOWRY_AUTH_READ", "private")

	cfg, err := config.Load(nil, nil)
	require.NoError(t, err)

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "postgres", cfg.Database.Type)
	assert.Equal(t, "private", cfg.Auth.Read)
}

func TestWithContext_FromContext(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Port: 8080, Mode: "store"},
	}

	ctx := context.Background()
	ctx = config.WithContext(ctx, cfg)

	retrieved, err := config.FromContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, cfg, retrieved)
	assert.Equal(t, 8080, retrieved.Server.Port)
}

func TestFromContext_NotFound(t *testing.T) {
	ctx := context.Background()

	_, err := config.FromContext(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config not found")
}

func TestLoad_WithFlags(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.Int("port", 5708, "port")
	flags.String("mode", "store", "mode")
	flags.String("db-type", "sqlite", "db type")

	// Simulate flag being set
	err := flags.Set("port", "9999")
	require.NoError(t, err)
	err = flags.Set("db-type", "postgres")
	require.NoError(t, err)

	cfg, err := config.Load(nil, flags)
	require.NoError(t, err)

	assert.Equal(t, 9999, cfg.Server.Port)
	assert.Equal(t, "postgres", cfg.Database.Type)
}

func TestLoad_MissingConfigFile(t *testing.T) {
	// Missing config file should use defaults, not error
	cfg, err := config.Load([]string{"/nonexistent/config.yaml"}, nil)
	require.NoError(t, err)

	assert.Equal(t, 5708, cfg.Server.Port) // Default value
}

func TestLoad_ValidationError_InvalidLogLevel(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  port: 5708
  mode: store
database:
  type: sqlite
  dsn: stowry.db
  tables:
    meta_data: test
storage:
  path: ./data
auth:
  read: public
  write: public
log:
  level: invalid
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	_, err = config.Load([]string{configPath}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validate config")
}
