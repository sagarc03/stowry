package keybackend_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sagarc03/stowry/keybackend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadKeysFromFile_ValidJSON(t *testing.T) {
	t.Parallel()

	content := `[
		{"access_key": "AKIAIOSFODNN7EXAMPLE", "secret_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"},
		{"access_key": "ANOTHER_KEY", "secret_key": "another_secret"}
	]`

	path := writeTestFile(t, content)

	keys, err := keybackend.LoadKeysFromFile(path)
	require.NoError(t, err)

	assert.Len(t, keys, 2)
	assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", keys["AKIAIOSFODNN7EXAMPLE"])
	assert.Equal(t, "another_secret", keys["ANOTHER_KEY"])
}

func TestLoadKeysFromFile_EmptyArray(t *testing.T) {
	t.Parallel()

	content := `[]`
	path := writeTestFile(t, content)

	keys, err := keybackend.LoadKeysFromFile(path)
	require.NoError(t, err)

	assert.Empty(t, keys)
}

func TestLoadKeysFromFile_SkipsEmptyKeys(t *testing.T) {
	t.Parallel()

	content := `[
		{"access_key": "", "secret_key": "secret1"},
		{"access_key": "key2", "secret_key": ""},
		{"access_key": "", "secret_key": ""},
		{"access_key": "valid_key", "secret_key": "valid_secret"}
	]`

	path := writeTestFile(t, content)

	keys, err := keybackend.LoadKeysFromFile(path)
	require.NoError(t, err)

	assert.Len(t, keys, 1)
	assert.Equal(t, "valid_secret", keys["valid_key"])
}

func TestLoadKeysFromFile_FileNotFound(t *testing.T) {
	t.Parallel()

	_, err := keybackend.LoadKeysFromFile("/nonexistent/path/keys.json")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read keys file")
}

func TestLoadKeysFromFile_InvalidJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "not json",
			content: "this is not json",
		},
		{
			name:    "json object instead of array",
			content: `{"access_key": "key", "secret_key": "secret"}`,
		},
		{
			name:    "malformed json",
			content: `[{"access_key": "key", "secret_key": "secret"`,
		},
		{
			name:    "array of strings",
			content: `["key1", "key2"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeTestFile(t, tt.content)

			_, err := keybackend.LoadKeysFromFile(path)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "parse keys file")
		})
	}
}

func TestLoadKeysFromFile_SingleKey(t *testing.T) {
	t.Parallel()

	content := `[{"access_key": "SINGLE_KEY", "secret_key": "single_secret"}]`
	path := writeTestFile(t, content)

	keys, err := keybackend.LoadKeysFromFile(path)
	require.NoError(t, err)

	assert.Len(t, keys, 1)
	assert.Equal(t, "single_secret", keys["SINGLE_KEY"])
}

func TestLoadKeysFromFile_DuplicateKeys(t *testing.T) {
	t.Parallel()

	content := `[
		{"access_key": "DUPLICATE", "secret_key": "first_secret"},
		{"access_key": "DUPLICATE", "secret_key": "second_secret"}
	]`

	path := writeTestFile(t, content)

	keys, err := keybackend.LoadKeysFromFile(path)
	require.NoError(t, err)

	assert.Len(t, keys, 1)
	// Last one wins
	assert.Equal(t, "second_secret", keys["DUPLICATE"])
}

func TestLoadKeysFromFile_SpecialCharactersInSecret(t *testing.T) {
	t.Parallel()

	content := `[
		{"access_key": "KEY1", "secret_key": "secret/with+special=chars"},
		{"access_key": "KEY2", "secret_key": "secret with spaces"},
		{"access_key": "KEY3", "secret_key": "secret\"with\"quotes"}
	]`

	path := writeTestFile(t, content)

	keys, err := keybackend.LoadKeysFromFile(path)
	require.NoError(t, err)

	assert.Len(t, keys, 3)
	assert.Equal(t, "secret/with+special=chars", keys["KEY1"])
	assert.Equal(t, "secret with spaces", keys["KEY2"])
	assert.Equal(t, "secret\"with\"quotes", keys["KEY3"])
}

func TestLoadKeysFromFile_ExtraFieldsIgnored(t *testing.T) {
	t.Parallel()

	content := `[
		{
			"access_key": "KEY1",
			"secret_key": "secret1",
			"extra_field": "ignored",
			"another": 123
		}
	]`

	path := writeTestFile(t, content)

	keys, err := keybackend.LoadKeysFromFile(path)
	require.NoError(t, err)

	assert.Len(t, keys, 1)
	assert.Equal(t, "secret1", keys["KEY1"])
}

// writeTestFile is a test helper that creates a temporary file with the given content
func writeTestFile(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "keys.json")

	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)

	return path
}
