package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sagarc03/stowry/internal/config"
	"github.com/sagarc03/stowry/types"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	return path
}

func load(t *testing.T, files []string, flags *pflag.FlagSet) *config.Config {
	t.Helper()

	cfg, err := config.Load(files, flags)
	require.NoError(t, err)

	return cfg
}

func TestLoadDefaults(t *testing.T) {
	cfg := load(t, nil, nil)

	assert.Equal(t, config.Defaults(), *cfg)
	assert.Equal(t, 5708, cfg.Server.Port)
	assert.Equal(t, types.ModeStore, cfg.Server.Mode)
	assert.Equal(t, "stowry_metadata", cfg.Database.Tables.MetaData)
	assert.Equal(t, 30*time.Second, cfg.Service.CleanupTimeout)
	assert.Equal(t, config.MemoryPath, cfg.Database.DSN, "the binary runs with no config and keeps nothing")
	assert.Equal(t, config.MemoryPath, cfg.Storage.Path)
}

func TestLoadYAML(t *testing.T) {
	path := writeConfig(t, `
server:
  port: 9090
  mode: static
  error_document: 404.html
database:
  type: postgres
  dsn: postgres://localhost/stowry
  tables:
    meta_data: custom_metadata
service:
  cleanup_timeout: 45s
cors:
  allowed_origins:
    - https://app.example.com
  max_age: 600
`)

	cfg := load(t, []string{path}, nil)

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, types.ModeStatic, cfg.Server.Mode)
	assert.Equal(t, "404.html", cfg.Server.ErrorDocument)
	assert.Equal(t, "postgres", cfg.Database.Type)
	assert.Equal(t, "custom_metadata", cfg.Database.Tables.MetaData)
	assert.Equal(t, 45*time.Second, cfg.Service.CleanupTimeout)
	assert.Equal(t, []string{"https://app.example.com"}, cfg.CORS.AllowedOrigins)

	assert.Equal(t, config.MemoryPath, cfg.Storage.Path, "an unset key keeps its default")
}

// Every field must be reachable from the environment, including ones whose
// default is the zero value. Viper resolves an environment variable only for a
// key it already knows, so a field missing from Defaults would be silently
// ignored here.
func TestLoadEnvReachesEverySetting(t *testing.T) {
	env := map[string]string{
		"STOWRY_SERVER_PORT":               "9090",
		"STOWRY_SERVER_MODE":               "spa",
		"STOWRY_SERVER_MAX_UPLOAD_SIZE":    "1048576",
		"STOWRY_SERVER_ERROR_DOCUMENT":     "404.html",
		"STOWRY_SERVICE_CLEANUP_TIMEOUT":   "45s",
		"STOWRY_DATABASE_TYPE":             "postgres",
		"STOWRY_DATABASE_DSN":              "postgres://localhost/stowry",
		"STOWRY_DATABASE_TABLES_META_DATA": "custom_metadata",
		"STOWRY_STORAGE_PATH":              "/data",
		"STOWRY_AUTH_READ":                 "private",
		"STOWRY_AUTH_WRITE":                "private",
		"STOWRY_AUTH_AWS_REGION":           "eu-west-1",
		"STOWRY_AUTH_AWS_SERVICE":          "s4",
		"STOWRY_AUTH_KEYS_FILE":            "/keys.json",
		"STOWRY_CORS_ALLOWED_ORIGINS":      "https://a.example.com",
		"STOWRY_CORS_ALLOW_CREDENTIALS":    "true",
		"STOWRY_CORS_MAX_AGE":              "600",
		"STOWRY_LOG_LEVEL":                 "debug",
	}
	for k, v := range env {
		t.Setenv(k, v)
	}

	cfg := load(t, nil, nil)

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, types.ModeSPA, cfg.Server.Mode)
	assert.Equal(t, int64(1048576), cfg.Server.MaxUploadSize)
	assert.Equal(t, "404.html", cfg.Server.ErrorDocument, "a zero-default field must still be env-reachable")
	assert.Equal(t, 45*time.Second, cfg.Service.CleanupTimeout)
	assert.Equal(t, "postgres", cfg.Database.Type)
	assert.Equal(t, "postgres://localhost/stowry", cfg.Database.DSN)
	assert.Equal(t, "custom_metadata", cfg.Database.Tables.MetaData)
	assert.Equal(t, "/data", cfg.Storage.Path)
	assert.Equal(t, config.AccessPrivate, cfg.Auth.Read)
	assert.Equal(t, config.AccessPrivate, cfg.Auth.Write)
	assert.Equal(t, "eu-west-1", cfg.Auth.AWS.Region)
	assert.Equal(t, "s4", cfg.Auth.AWS.Service)
	assert.Equal(t, "/keys.json", cfg.Auth.Keys.File, "a zero-default field must still be env-reachable")
	assert.Equal(t, []string{"https://a.example.com"}, cfg.CORS.AllowedOrigins)
	assert.True(t, cfg.CORS.AllowCredentials)
	assert.Equal(t, 600, cfg.CORS.MaxAge)
	assert.Equal(t, "debug", cfg.Log.Level)
}

func TestLoadEnvList(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "single", value: "https://a.example.com", want: []string{"https://a.example.com"}},
		{name: "comma", value: "https://a.example.com,https://b.example.com", want: []string{"https://a.example.com", "https://b.example.com"}},
		{name: "comma and space", value: "https://a.example.com, https://b.example.com", want: []string{"https://a.example.com", "https://b.example.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("STOWRY_CORS_ALLOWED_ORIGINS", tt.value)

			assert.Equal(t, tt.want, load(t, nil, nil).CORS.AllowedOrigins)
		})
	}
}

func TestLoadPrecedence(t *testing.T) {
	path := writeConfig(t, "server:\n  port: 1111\nstorage:\n  path: /from-yaml\n")

	t.Run("env beats the file", func(t *testing.T) {
		t.Setenv("STOWRY_SERVER_PORT", "2222")

		assert.Equal(t, 2222, load(t, []string{path}, nil).Server.Port)
	})

	t.Run("a flag beats env and the file", func(t *testing.T) {
		t.Setenv("STOWRY_SERVER_PORT", "2222")

		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flags.Int("port", 0, "")
		require.NoError(t, flags.Set("port", "3333"))

		assert.Equal(t, 3333, load(t, []string{path}, flags).Server.Port)
	})

	t.Run("an unset flag changes nothing", func(t *testing.T) {
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flags.String("storage-path", "", "")

		assert.Equal(t, "/from-yaml", load(t, []string{path}, flags).Storage.Path)
	})

	t.Run("a later file beats an earlier one", func(t *testing.T) {
		second := writeConfig(t, "server:\n  port: 4444\n")

		cfg := load(t, []string{path, second}, nil)
		assert.Equal(t, 4444, cfg.Server.Port)
		assert.Equal(t, "/from-yaml", cfg.Storage.Path, "keys only in the first file survive")
	})
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := config.Load([]string{"/nonexistent/config.yaml"}, nil)

	require.NoError(t, err, "a missing file falls back to defaults")
	assert.Equal(t, 5708, cfg.Server.Port)
}

func TestLoadRejectsInvalid(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		errContains string
	}{
		{name: "port out of range", yaml: "server:\n  port: 99999\n", errContains: "Port"},
		{name: "unknown mode", yaml: "server:\n  mode: gopher\n", errContains: "Mode"},
		{name: "unknown database", yaml: "database:\n  type: mysql\n", errContains: "Type"},
		{name: "empty dsn", yaml: "database:\n  dsn: \"\"\n", errContains: "DSN"},
		{name: "table name is not an identifier", yaml: "database:\n  tables:\n    meta_data: Meta-Data\n", errContains: "table_name"},
		{name: "unknown access", yaml: "auth:\n  read: maybe\n", errContains: "Read"},
		{name: "unknown log level", yaml: "log:\n  level: loud\n", errContains: "Level"},
		{name: "half-written key pair", yaml: "auth:\n  keys:\n    inline:\n      - access_key: AKIA\n", errContains: "SecretKey"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := config.Load([]string{writeConfig(t, tt.yaml)}, nil)

			require.Error(t, err)
			assert.ErrorContains(t, err, tt.errContains)
		})
	}
}

func TestValidateForServe(t *testing.T) {
	tests := []struct {
		name    string
		mode    types.ServerMode
		read    config.Access
		wantErr bool
	}{
		{name: "store mode allows private reads", mode: types.ModeStore, read: config.AccessPrivate},
		{name: "store mode allows public reads", mode: types.ModeStore, read: config.AccessPublic},
		{name: "static mode allows public reads", mode: types.ModeStatic, read: config.AccessPublic},
		{name: "static mode rejects private reads", mode: types.ModeStatic, read: config.AccessPrivate, wantErr: true},
		{name: "spa mode rejects private reads", mode: types.ModeSPA, read: config.AccessPrivate, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Defaults()
			cfg.Server.Mode = tt.mode
			cfg.Auth.Read = tt.read

			err := cfg.ValidateForServe()
			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}

			assert.ErrorContains(t, err, "cannot verify signed requests")
		})
	}
}

func TestConversions(t *testing.T) {
	cfg := config.Defaults()
	cfg.Auth.Keys.Inline = []config.KeyPair{{AccessKey: "AKIA", SecretKey: "secret"}}
	cfg.Auth.Keys.File = "/keys.json"

	t.Run("database", func(t *testing.T) {
		got := cfg.DatabaseConfig()

		assert.Equal(t, "sqlite", got.Type)
		assert.Equal(t, config.MemoryPath, got.DSN)
		assert.Equal(t, "stowry_metadata", got.Tables.MetaData)
		assert.NoError(t, got.Tables.Validate())
	})

	t.Run("keys", func(t *testing.T) {
		got := cfg.KeysConfig()

		require.Len(t, got.Inline, 1)
		assert.Equal(t, "AKIA", got.Inline[0].AccessKey)
		assert.Equal(t, "secret", got.Inline[0].SecretKey)
		assert.Equal(t, "/keys.json", got.File)
	})

	t.Run("aws", func(t *testing.T) {
		got := cfg.AWSConfig()

		assert.Equal(t, "us-east-1", got.Region)
		assert.Equal(t, "s3", got.Service)
	})

	t.Run("cors is off when no origins are set", func(t *testing.T) {
		_, enabled := cfg.CORSConfig()

		assert.False(t, enabled)
	})

	t.Run("cors is on once origins are set", func(t *testing.T) {
		withCORS := cfg
		withCORS.CORS.AllowedOrigins = []string{"*"}
		withCORS.CORS.MaxAge = 600

		got, enabled := withCORS.CORSConfig()

		assert.True(t, enabled)
		assert.Equal(t, []string{"*"}, got.AllowedOrigins)
		assert.Equal(t, 600, got.MaxAge)
	})
}
