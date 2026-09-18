package keybackend_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sagarc03/stowry/internal/keybackend"
	"github.com/sagarc03/stowry/sign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapSecretStoreLookup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		keys      map[string]string
		accessKey string
		want      string
		wantErr   error
	}{
		{
			name:      "returns the secret for a known access key",
			keys:      map[string]string{"access1": "secret1", "access2": "secret2"},
			accessKey: "access1",
			want:      "secret1",
		},
		{
			name:      "rejects an unknown access key",
			keys:      map[string]string{"access1": "secret1"},
			accessKey: "nonexistent",
			wantErr:   keybackend.ErrKeyNotFound,
		},
		{
			name:      "rejects any key when the store is empty",
			keys:      map[string]string{},
			accessKey: "anykey",
			wantErr:   keybackend.ErrKeyNotFound,
		},
		{
			name:      "rejects any key when the map is nil",
			keys:      nil,
			accessKey: "anykey",
			wantErr:   keybackend.ErrKeyNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := keybackend.NewMapSecretStore(tt.keys).Lookup(tt.accessKey)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewSecretStoreInlineKeys(t *testing.T) {
	t.Parallel()

	store := mustStore(t, keybackend.KeysConfig{
		Inline: []keybackend.KeyPair{
			{AccessKey: "KEY1", SecretKey: "secret1"},
			{AccessKey: "KEY2", SecretKey: "secret2"},
		},
	})

	assert.Equal(t, "secret1", lookup(t, store, "KEY1"))
	assert.Equal(t, "secret2", lookup(t, store, "KEY2"))

	_, err := store.Lookup("NONEXISTENT_KEY")
	assert.ErrorIs(t, err, keybackend.ErrKeyNotFound)
}

func TestNewSecretStoreEmptyConfig(t *testing.T) {
	t.Parallel()

	store := mustStore(t, keybackend.KeysConfig{})

	_, err := store.Lookup("ANY_KEY")
	assert.ErrorIs(t, err, keybackend.ErrKeyNotFound)
}

func TestNewSecretStoreMergesInlineAndFile(t *testing.T) {
	t.Parallel()

	t.Run("keeps both sources", func(t *testing.T) {
		t.Parallel()

		store := mustStore(t, keybackend.KeysConfig{
			Inline: []keybackend.KeyPair{{AccessKey: "INLINE_KEY", SecretKey: "inline_secret"}},
			File:   writeKeysFile(t, `[{"access_key": "FILE_KEY", "secret_key": "file_secret"}]`),
		})

		assert.Equal(t, "inline_secret", lookup(t, store, "INLINE_KEY"))
		assert.Equal(t, "file_secret", lookup(t, store, "FILE_KEY"))
	})

	t.Run("file wins on a duplicate access key", func(t *testing.T) {
		t.Parallel()

		store := mustStore(t, keybackend.KeysConfig{
			Inline: []keybackend.KeyPair{{AccessKey: "DUPLICATE", SecretKey: "inline_loses"}},
			File:   writeKeysFile(t, `[{"access_key": "DUPLICATE", "secret_key": "file_wins"}]`),
		})

		assert.Equal(t, "file_wins", lookup(t, store, "DUPLICATE"))
	})
}

func TestNewSecretStoreSkipsIncompletePairs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  keybackend.KeysConfig
	}{
		{
			name: "inline",
			cfg: keybackend.KeysConfig{
				Inline: []keybackend.KeyPair{
					{AccessKey: "", SecretKey: "secret1"},
					{AccessKey: "NO_SECRET", SecretKey: ""},
					{AccessKey: "", SecretKey: ""},
					{AccessKey: "VALID_KEY", SecretKey: "valid_secret"},
				},
			},
		},
		{
			name: "file",
			cfg: keybackend.KeysConfig{
				File: writeKeysFile(t, `[
					{"access_key": "", "secret_key": "secret1"},
					{"access_key": "NO_SECRET", "secret_key": ""},
					{"access_key": "", "secret_key": ""},
					{"access_key": "VALID_KEY", "secret_key": "valid_secret"}
				]`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := mustStore(t, tt.cfg)
			assert.Equal(t, "valid_secret", lookup(t, store, "VALID_KEY"))

			for _, absent := range []string{"", "NO_SECRET"} {
				_, err := store.Lookup(absent)
				assert.ErrorIs(t, err, keybackend.ErrKeyNotFound)
			}
		})
	}
}

func TestNewSecretStoreReadsFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    map[string]string
	}{
		{
			name:    "empty array",
			content: `[]`,
			want:    map[string]string{},
		},
		{
			name:    "single pair",
			content: `[{"access_key": "SINGLE_KEY", "secret_key": "single_secret"}]`,
			want:    map[string]string{"SINGLE_KEY": "single_secret"},
		},
		{
			name: "last duplicate wins",
			content: `[
				{"access_key": "DUPLICATE", "secret_key": "first_secret"},
				{"access_key": "DUPLICATE", "secret_key": "second_secret"}
			]`,
			want: map[string]string{"DUPLICATE": "second_secret"},
		},
		{
			name: "secrets with special characters",
			content: `[
				{"access_key": "KEY1", "secret_key": "secret/with+special=chars"},
				{"access_key": "KEY2", "secret_key": "secret with spaces"},
				{"access_key": "KEY3", "secret_key": "secret\"with\"quotes"}
			]`,
			want: map[string]string{
				"KEY1": "secret/with+special=chars",
				"KEY2": "secret with spaces",
				"KEY3": `secret"with"quotes`,
			},
		},
		{
			name: "unknown fields are ignored",
			content: `[{
				"access_key": "KEY1",
				"secret_key": "secret1",
				"extra_field": "ignored",
				"another": 123
			}]`,
			want: map[string]string{"KEY1": "secret1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := mustStore(t, keybackend.KeysConfig{File: writeKeysFile(t, tt.content)})
			for accessKey, secret := range tt.want {
				assert.Equal(t, secret, lookup(t, store, accessKey))
			}
		})
	}
}

func TestNewSecretStoreRejectsUnreadableFile(t *testing.T) {
	t.Parallel()

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()

		_, err := keybackend.NewSecretStore(keybackend.KeysConfig{File: "/nonexistent/path/keys.json"})
		assert.ErrorContains(t, err, "read keys file")
	})

	tests := []struct {
		name    string
		content string
	}{
		{name: "not json", content: "this is not json"},
		{name: "object instead of array", content: `{"access_key": "key", "secret_key": "secret"}`},
		{name: "truncated", content: `[{"access_key": "key", "secret_key": "secret"`},
		{name: "array of strings", content: `["key1", "key2"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := keybackend.NewSecretStore(keybackend.KeysConfig{File: writeKeysFile(t, tt.content)})
			assert.ErrorContains(t, err, "parse keys file")
		})
	}
}

func mustStore(t *testing.T, cfg keybackend.KeysConfig) sign.SecretStore {
	t.Helper()

	store, err := keybackend.NewSecretStore(cfg)
	require.NoError(t, err)

	return store
}

func lookup(t *testing.T, store sign.SecretStore, accessKey string) string {
	t.Helper()

	secret, err := store.Lookup(accessKey)
	require.NoError(t, err)

	return secret
}

// writeKeysFile writes content to a keys.json in a temporary directory and
// returns its path.
func writeKeysFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "keys.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}
