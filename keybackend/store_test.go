package keybackend_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sagarc03/stowry/keybackend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSecretStore_InlineKeysOnly(t *testing.T) {
	t.Parallel()

	cfg := keybackend.KeysConfig{
		Inline: []keybackend.KeyPair{
			{AccessKey: "KEY1", SecretKey: "secret1"},
			{AccessKey: "KEY2", SecretKey: "secret2"},
		},
	}

	store, err := keybackend.NewSecretStore(cfg)
	require.NoError(t, err)

	secret1, err := store.Lookup("KEY1")
	require.NoError(t, err)
	assert.Equal(t, "secret1", secret1)

	secret2, err := store.Lookup("KEY2")
	require.NoError(t, err)
	assert.Equal(t, "secret2", secret2)
}

func TestNewSecretStore_FileKeysOnly(t *testing.T) {
	t.Parallel()

	content := `[
		{"access_key": "FILE_KEY1", "secret_key": "file_secret1"},
		{"access_key": "FILE_KEY2", "secret_key": "file_secret2"}
	]`
	path := writeKeysFile(t, content)

	cfg := keybackend.KeysConfig{
		File: path,
	}

	store, err := keybackend.NewSecretStore(cfg)
	require.NoError(t, err)

	secret1, err := store.Lookup("FILE_KEY1")
	require.NoError(t, err)
	assert.Equal(t, "file_secret1", secret1)

	secret2, err := store.Lookup("FILE_KEY2")
	require.NoError(t, err)
	assert.Equal(t, "file_secret2", secret2)
}

func TestNewSecretStore_BothInlineAndFile(t *testing.T) {
	t.Parallel()

	content := `[{"access_key": "FILE_KEY", "secret_key": "file_secret"}]`
	path := writeKeysFile(t, content)

	cfg := keybackend.KeysConfig{
		Inline: []keybackend.KeyPair{
			{AccessKey: "INLINE_KEY", SecretKey: "inline_secret"},
		},
		File: path,
	}

	store, err := keybackend.NewSecretStore(cfg)
	require.NoError(t, err)

	// Both keys should be accessible
	inlineSecret, err := store.Lookup("INLINE_KEY")
	require.NoError(t, err)
	assert.Equal(t, "inline_secret", inlineSecret)

	fileSecret, err := store.Lookup("FILE_KEY")
	require.NoError(t, err)
	assert.Equal(t, "file_secret", fileSecret)
}

func TestNewSecretStore_FileOverridesInline(t *testing.T) {
	t.Parallel()

	content := `[{"access_key": "DUPLICATE_KEY", "secret_key": "file_wins"}]`
	path := writeKeysFile(t, content)

	cfg := keybackend.KeysConfig{
		Inline: []keybackend.KeyPair{
			{AccessKey: "DUPLICATE_KEY", SecretKey: "inline_loses"},
		},
		File: path,
	}

	store, err := keybackend.NewSecretStore(cfg)
	require.NoError(t, err)

	secret, err := store.Lookup("DUPLICATE_KEY")
	require.NoError(t, err)
	assert.Equal(t, "file_wins", secret, "file keys should override inline keys")
}

func TestNewSecretStore_EmptyConfig(t *testing.T) {
	t.Parallel()

	cfg := keybackend.KeysConfig{}

	store, err := keybackend.NewSecretStore(cfg)
	require.NoError(t, err)

	_, err = store.Lookup("ANY_KEY")
	assert.Error(t, err)
}

func TestNewSecretStore_InlineSkipsEmptyKeys(t *testing.T) {
	t.Parallel()

	cfg := keybackend.KeysConfig{
		Inline: []keybackend.KeyPair{
			{AccessKey: "", SecretKey: "secret1"},
			{AccessKey: "KEY2", SecretKey: ""},
			{AccessKey: "", SecretKey: ""},
			{AccessKey: "VALID_KEY", SecretKey: "valid_secret"},
		},
	}

	store, err := keybackend.NewSecretStore(cfg)
	require.NoError(t, err)

	// Only VALID_KEY should be present
	secret, err := store.Lookup("VALID_KEY")
	require.NoError(t, err)
	assert.Equal(t, "valid_secret", secret)

	// Empty access keys should not be stored
	_, err = store.Lookup("")
	assert.Error(t, err)
}

func TestNewSecretStore_FileNotFound(t *testing.T) {
	t.Parallel()

	cfg := keybackend.KeysConfig{
		File: "/nonexistent/path/keys.json",
	}

	_, err := keybackend.NewSecretStore(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read keys file")
}

func TestNewSecretStore_InvalidFileJSON(t *testing.T) {
	t.Parallel()

	content := "not valid json"
	path := writeKeysFile(t, content)

	cfg := keybackend.KeysConfig{
		File: path,
	}

	_, err := keybackend.NewSecretStore(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse keys file")
}

func TestNewSecretStore_KeyNotFound(t *testing.T) {
	t.Parallel()

	cfg := keybackend.KeysConfig{
		Inline: []keybackend.KeyPair{
			{AccessKey: "EXISTING_KEY", SecretKey: "secret"},
		},
	}

	store, err := keybackend.NewSecretStore(cfg)
	require.NoError(t, err)

	_, err = store.Lookup("NONEXISTENT_KEY")
	assert.Error(t, err)
}

// writeKeysFile is a test helper that creates a temporary file with the given content
func writeKeysFile(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "keys.json")

	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)

	return path
}
