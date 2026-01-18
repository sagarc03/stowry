// Package keybackend provides SecretStore implementations for key retrieval.
package keybackend

import (
	"fmt"

	"github.com/sagarc03/stowry"
)

// MapSecretStore retrieves keys from an in-memory map.
// Suitable for configuration file-based key storage.
type MapSecretStore struct {
	keys map[string]string
}

// NewMapSecretStore creates a new map-based secret store with the given access key to secret key mapping.
func NewMapSecretStore(keys map[string]string) *MapSecretStore {
	return &MapSecretStore{keys: keys}
}

// Lookup retrieves the secret key for the given access key from the map.
func (s *MapSecretStore) Lookup(accessKey string) (string, error) {
	secretKey, found := s.keys[accessKey]
	if !found {
		return "", fmt.Errorf("access key not found: %w", stowry.ErrUnauthorized)
	}
	return secretKey, nil
}
