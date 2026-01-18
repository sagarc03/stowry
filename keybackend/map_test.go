package keybackend_test

import (
	"testing"

	"github.com/sagarc03/stowry/keybackend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapSecretStore_Lookup(t *testing.T) {
	tests := []struct {
		name      string
		keys      map[string]string
		accessKey string
		wantKey   string
		wantErr   error
	}{
		{
			name: "returns secret key when access key exists",
			keys: map[string]string{
				"access1": "secret1",
				"access2": "secret2",
			},
			accessKey: "access1",
			wantKey:   "secret1",
			wantErr:   nil,
		},
		{
			name: "returns ErrKeyNotFound when access key does not exist",
			keys: map[string]string{
				"access1": "secret1",
			},
			accessKey: "nonexistent",
			wantKey:   "",
			wantErr:   keybackend.ErrKeyNotFound,
		},
		{
			name:      "returns ErrKeyNotFound for empty store",
			keys:      map[string]string{},
			accessKey: "anykey",
			wantKey:   "",
			wantErr:   keybackend.ErrKeyNotFound,
		},
		{
			name:      "returns ErrKeyNotFound for nil store",
			keys:      nil,
			accessKey: "anykey",
			wantKey:   "",
			wantErr:   keybackend.ErrKeyNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := keybackend.NewMapSecretStore(tt.keys)
			gotKey, err := store.Lookup(tt.accessKey)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, gotKey)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantKey, gotKey)
			}
		})
	}
}
