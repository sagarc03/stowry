package clientcli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sagarc03/stowry/clientcli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFile_GetProfile(t *testing.T) {
	cfg := &clientcli.ConfigFile{
		Profiles: []clientcli.Profile{
			{Name: "local", Endpoint: "http://localhost:5708"},
			{Name: "production", Endpoint: "https://prod.example.com", Default: true},
			{Name: "staging", Endpoint: "https://staging.example.com"},
		},
	}

	t.Run("get by name", func(t *testing.T) {
		p, err := cfg.GetProfile("staging")
		require.NoError(t, err)
		assert.Equal(t, "staging", p.Name)
		assert.Equal(t, "https://staging.example.com", p.Endpoint)
	})

	t.Run("get default when name is empty", func(t *testing.T) {
		p, err := cfg.GetProfile("")
		require.NoError(t, err)
		assert.Equal(t, "production", p.Name) // marked as default
	})

	t.Run("profile not found", func(t *testing.T) {
		_, err := cfg.GetProfile("nonexistent")
		assert.ErrorIs(t, err, clientcli.ErrProfileNotFound)
	})

	t.Run("no profiles configured", func(t *testing.T) {
		emptyCfg := &clientcli.ConfigFile{}
		_, err := emptyCfg.GetProfile("any")
		assert.ErrorIs(t, err, clientcli.ErrNoProfiles)
	})
}

func TestConfigFile_GetDefaultProfile(t *testing.T) {
	t.Run("returns profile marked as default", func(t *testing.T) {
		cfg := &clientcli.ConfigFile{
			Profiles: []clientcli.Profile{
				{Name: "local", Endpoint: "http://localhost:5708"},
				{Name: "production", Endpoint: "https://prod.example.com", Default: true},
			},
		}

		p, err := cfg.GetDefaultProfile()
		require.NoError(t, err)
		assert.Equal(t, "production", p.Name)
	})

	t.Run("returns first profile when none marked default", func(t *testing.T) {
		cfg := &clientcli.ConfigFile{
			Profiles: []clientcli.Profile{
				{Name: "local", Endpoint: "http://localhost:5708"},
				{Name: "production", Endpoint: "https://prod.example.com"},
			},
		}

		p, err := cfg.GetDefaultProfile()
		require.NoError(t, err)
		assert.Equal(t, "local", p.Name)
	})

	t.Run("error when no profiles", func(t *testing.T) {
		cfg := &clientcli.ConfigFile{}
		_, err := cfg.GetDefaultProfile()
		assert.ErrorIs(t, err, clientcli.ErrNoProfiles)
	})
}

func TestConfigFile_AddProfile(t *testing.T) {
	t.Run("add new profile", func(t *testing.T) {
		cfg := &clientcli.ConfigFile{
			Profiles: []clientcli.Profile{
				{Name: "local", Endpoint: "http://localhost:5708"},
			},
		}

		err := cfg.AddProfile(clientcli.Profile{
			Name:     "production",
			Endpoint: "https://prod.example.com",
		})
		require.NoError(t, err)
		assert.Len(t, cfg.Profiles, 2)

		p, err := cfg.GetProfile("production")
		require.NoError(t, err)
		assert.Equal(t, "https://prod.example.com", p.Endpoint)
	})

	t.Run("add existing profile fails", func(t *testing.T) {
		cfg := &clientcli.ConfigFile{
			Profiles: []clientcli.Profile{
				{Name: "local", Endpoint: "http://localhost:5708"},
			},
		}

		err := cfg.AddProfile(clientcli.Profile{
			Name:     "local",
			Endpoint: "http://localhost:9999",
		})
		assert.ErrorIs(t, err, clientcli.ErrProfileExists)
		assert.Len(t, cfg.Profiles, 1)
		// Original unchanged
		assert.Equal(t, "http://localhost:5708", cfg.Profiles[0].Endpoint)
	})

	t.Run("add to empty config", func(t *testing.T) {
		cfg := &clientcli.ConfigFile{}

		err := cfg.AddProfile(clientcli.Profile{
			Name:     "local",
			Endpoint: "http://localhost:5708",
		})
		require.NoError(t, err)
		assert.Len(t, cfg.Profiles, 1)
	})
}

func TestConfigFile_UpdateProfile(t *testing.T) {
	t.Run("update existing profile", func(t *testing.T) {
		cfg := &clientcli.ConfigFile{
			Profiles: []clientcli.Profile{
				{Name: "local", Endpoint: "http://localhost:5708"},
			},
		}

		err := cfg.UpdateProfile(clientcli.Profile{
			Name:     "local",
			Endpoint: "http://localhost:9999",
		})
		require.NoError(t, err)
		assert.Len(t, cfg.Profiles, 1)
		assert.Equal(t, "http://localhost:9999", cfg.Profiles[0].Endpoint)
	})

	t.Run("update nonexistent profile fails", func(t *testing.T) {
		cfg := &clientcli.ConfigFile{
			Profiles: []clientcli.Profile{
				{Name: "local", Endpoint: "http://localhost:5708"},
			},
		}

		err := cfg.UpdateProfile(clientcli.Profile{
			Name:     "production",
			Endpoint: "https://prod.example.com",
		})
		assert.ErrorIs(t, err, clientcli.ErrProfileNotFound)
		assert.Len(t, cfg.Profiles, 1)
	})

	t.Run("update preserves other fields", func(t *testing.T) {
		cfg := &clientcli.ConfigFile{
			Profiles: []clientcli.Profile{
				{Name: "local", Endpoint: "http://localhost:5708", AccessKey: "old-key"},
			},
		}

		err := cfg.UpdateProfile(clientcli.Profile{
			Name:      "local",
			Endpoint:  "http://localhost:9999",
			AccessKey: "new-key",
			SecretKey: "new-secret",
		})
		require.NoError(t, err)
		assert.Equal(t, "http://localhost:9999", cfg.Profiles[0].Endpoint)
		assert.Equal(t, "new-key", cfg.Profiles[0].AccessKey)
		assert.Equal(t, "new-secret", cfg.Profiles[0].SecretKey)
	})
}

func TestConfigFile_RemoveProfile(t *testing.T) {
	t.Run("remove existing profile", func(t *testing.T) {
		cfg := &clientcli.ConfigFile{
			Profiles: []clientcli.Profile{
				{Name: "local", Endpoint: "http://localhost:5708"},
				{Name: "production", Endpoint: "https://prod.example.com"},
			},
		}

		err := cfg.RemoveProfile("local")
		require.NoError(t, err)
		assert.Len(t, cfg.Profiles, 1)
		assert.Equal(t, "production", cfg.Profiles[0].Name)
	})

	t.Run("remove nonexistent profile", func(t *testing.T) {
		cfg := &clientcli.ConfigFile{
			Profiles: []clientcli.Profile{
				{Name: "local", Endpoint: "http://localhost:5708"},
			},
		}

		err := cfg.RemoveProfile("nonexistent")
		assert.ErrorIs(t, err, clientcli.ErrProfileNotFound)
	})
}

func TestConfigFile_SetDefault(t *testing.T) {
	t.Run("set default profile", func(t *testing.T) {
		cfg := &clientcli.ConfigFile{
			Profiles: []clientcli.Profile{
				{Name: "local", Endpoint: "http://localhost:5708", Default: true},
				{Name: "production", Endpoint: "https://prod.example.com"},
			},
		}

		err := cfg.SetDefault("production")
		require.NoError(t, err)

		// Check production is now default
		assert.True(t, cfg.Profiles[1].Default)
		// Check local is no longer default
		assert.False(t, cfg.Profiles[0].Default)
	})

	t.Run("set default nonexistent profile", func(t *testing.T) {
		cfg := &clientcli.ConfigFile{
			Profiles: []clientcli.Profile{
				{Name: "local", Endpoint: "http://localhost:5708"},
			},
		}

		err := cfg.SetDefault("nonexistent")
		assert.ErrorIs(t, err, clientcli.ErrProfileNotFound)
	})
}

func TestConfigFile_ProfileNames(t *testing.T) {
	cfg := &clientcli.ConfigFile{
		Profiles: []clientcli.Profile{
			{Name: "local"},
			{Name: "production"},
			{Name: "staging"},
		},
	}

	names := cfg.ProfileNames()
	assert.Equal(t, []string{"local", "production", "staging"}, names)
}

func TestConfigFile_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".stowry", "config.yaml")

	original := &clientcli.ConfigFile{
		Profiles: []clientcli.Profile{
			{
				Name:      "local",
				Endpoint:  "http://localhost:5708",
				AccessKey: "access123",
				SecretKey: "secret456",
				Default:   true,
			},
			{
				Name:      "production",
				Endpoint:  "https://prod.example.com",
				AccessKey: "prodaccess",
				SecretKey: "prodsecret",
			},
		},
	}

	// Save
	err := original.Save(configPath)
	require.NoError(t, err)

	// Verify file exists with correct permissions
	info, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// Load
	loaded, err := clientcli.LoadConfigFile(configPath)
	require.NoError(t, err)

	assert.Len(t, loaded.Profiles, 2)
	assert.Equal(t, "local", loaded.Profiles[0].Name)
	assert.Equal(t, "http://localhost:5708", loaded.Profiles[0].Endpoint)
	assert.Equal(t, "access123", loaded.Profiles[0].AccessKey)
	assert.Equal(t, "secret456", loaded.Profiles[0].SecretKey)
	assert.True(t, loaded.Profiles[0].Default)
}

func TestLoadConfigFile_Errors(t *testing.T) {
	t.Run("file not found", func(t *testing.T) {
		_, err := clientcli.LoadConfigFile("/nonexistent/path/config.yaml")
		assert.Error(t, err)
	})

	t.Run("invalid yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		content := `invalid: [yaml: content`
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err)

		_, err = clientcli.LoadConfigFile(configPath)
		assert.Error(t, err)
	})
}

func TestConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &clientcli.Config{Endpoint: "http://localhost:5708"}
		err := cfg.Validate()
		assert.NoError(t, err)
		assert.Equal(t, "http://localhost:5708", cfg.Endpoint)
	})

	t.Run("empty endpoint does not mutate", func(t *testing.T) {
		cfg := &clientcli.Config{}
		err := cfg.Validate()
		assert.NoError(t, err)
		assert.Equal(t, "", cfg.Endpoint) // Validate no longer mutates
	})
}

func TestConfig_WithDefaults(t *testing.T) {
	t.Run("applies default endpoint", func(t *testing.T) {
		cfg := &clientcli.Config{}
		result := cfg.WithDefaults()
		assert.Equal(t, clientcli.DefaultEndpoint, result.Endpoint)
		assert.Equal(t, "", cfg.Endpoint) // Original unchanged
	})

	t.Run("preserves existing endpoint", func(t *testing.T) {
		cfg := &clientcli.Config{Endpoint: "http://custom:8080"}
		result := cfg.WithDefaults()
		assert.Equal(t, "http://custom:8080", result.Endpoint)
	})

	t.Run("copies all fields", func(t *testing.T) {
		cfg := &clientcli.Config{
			Endpoint:  "http://localhost:5708",
			AccessKey: "key",
			SecretKey: "secret",
		}
		result := cfg.WithDefaults()
		assert.Equal(t, cfg.Endpoint, result.Endpoint)
		assert.Equal(t, cfg.AccessKey, result.AccessKey)
		assert.Equal(t, cfg.SecretKey, result.SecretKey)
	})
}

func TestConfig_ValidateWithAuth(t *testing.T) {
	t.Run("valid config with auth", func(t *testing.T) {
		cfg := &clientcli.Config{
			Endpoint:  "http://localhost:5708",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
		}
		err := cfg.ValidateWithAuth()
		assert.NoError(t, err)
	})

	t.Run("missing access key", func(t *testing.T) {
		cfg := &clientcli.Config{
			Endpoint:  "http://localhost:5708",
			SecretKey: "test-secret-key",
		}
		err := cfg.ValidateWithAuth()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access key")
	})

	t.Run("missing secret key", func(t *testing.T) {
		cfg := &clientcli.Config{
			Endpoint:  "http://localhost:5708",
			AccessKey: "test-access-key",
		}
		err := cfg.ValidateWithAuth()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "secret key")
	})
}

func TestConfigFromProfile(t *testing.T) {
	t.Run("converts profile to config", func(t *testing.T) {
		p := &clientcli.Profile{
			Name:      "test",
			Endpoint:  "https://example.com",
			AccessKey: "access",
			SecretKey: "secret",
		}

		cfg := clientcli.ConfigFromProfile(p)
		assert.Equal(t, "https://example.com", cfg.Endpoint)
		assert.Equal(t, "access", cfg.AccessKey)
		assert.Equal(t, "secret", cfg.SecretKey)
	})

	t.Run("nil profile returns empty config", func(t *testing.T) {
		cfg := clientcli.ConfigFromProfile(nil)
		assert.Equal(t, "", cfg.Endpoint)
		assert.Equal(t, "", cfg.AccessKey)
		assert.Equal(t, "", cfg.SecretKey)
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
				{Endpoint: "http://a.com", AccessKey: "key1", SecretKey: "secret1"},
			},
			expected: &clientcli.Config{Endpoint: "http://a.com", AccessKey: "key1", SecretKey: "secret1"},
		},
		{
			name: "later config overrides",
			configs: []*clientcli.Config{
				{Endpoint: "http://a.com", AccessKey: "key1", SecretKey: "secret1"},
				{Endpoint: "http://b.com", AccessKey: "key2"},
			},
			expected: &clientcli.Config{Endpoint: "http://b.com", AccessKey: "key2", SecretKey: "secret1"},
		},
		{
			name: "empty strings do not override",
			configs: []*clientcli.Config{
				{Endpoint: "http://a.com", AccessKey: "key1", SecretKey: "secret1"},
				{Endpoint: "", AccessKey: "", SecretKey: ""},
			},
			expected: &clientcli.Config{Endpoint: "http://a.com", AccessKey: "key1", SecretKey: "secret1"},
		},
		{
			name: "nil config is skipped",
			configs: []*clientcli.Config{
				{Endpoint: "http://a.com"},
				nil,
				{AccessKey: "key2"},
			},
			expected: &clientcli.Config{Endpoint: "http://a.com", AccessKey: "key2"},
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
	origEndpoint := os.Getenv("STOWRY_ENDPOINT")
	origAccessKey := os.Getenv("STOWRY_ACCESS_KEY")
	origSecretKey := os.Getenv("STOWRY_SECRET_KEY")

	// Restore after test
	t.Cleanup(func() {
		_ = os.Setenv("STOWRY_ENDPOINT", origEndpoint)
		_ = os.Setenv("STOWRY_ACCESS_KEY", origAccessKey)
		_ = os.Setenv("STOWRY_SECRET_KEY", origSecretKey)
	})

	// Set test values
	_ = os.Setenv("STOWRY_ENDPOINT", "http://test.example.com")
	_ = os.Setenv("STOWRY_ACCESS_KEY", "env-access-key")
	_ = os.Setenv("STOWRY_SECRET_KEY", "env-secret-key")

	cfg := clientcli.ConfigFromEnv()

	assert.Equal(t, "http://test.example.com", cfg.Endpoint)
	assert.Equal(t, "env-access-key", cfg.AccessKey)
	assert.Equal(t, "env-secret-key", cfg.SecretKey)
}

func TestProfileFromEnv(t *testing.T) {
	// Save original value
	origProfile := os.Getenv("STOWRY_PROFILE")

	// Restore after test
	t.Cleanup(func() {
		_ = os.Setenv("STOWRY_PROFILE", origProfile)
	})

	t.Run("returns profile from env", func(t *testing.T) {
		_ = os.Setenv("STOWRY_PROFILE", "production")
		assert.Equal(t, "production", clientcli.ProfileFromEnv())
	})

	t.Run("returns empty when not set", func(t *testing.T) {
		_ = os.Unsetenv("STOWRY_PROFILE")
		assert.Equal(t, "", clientcli.ProfileFromEnv())
	})
}
