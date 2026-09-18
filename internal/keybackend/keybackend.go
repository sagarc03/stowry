// Package keybackend loads access keys into a sign.SecretStore.
package keybackend

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"

	"github.com/sagarc03/stowry/sign"
)

// ErrKeyNotFound is returned when the access key does not exist in the store.
var ErrKeyNotFound = errors.New("access key not found")

// KeyPair is an access key and the secret it is signed with.
type KeyPair struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

// KeysConfig says where to find access keys.
type KeysConfig struct {
	// Inline holds key pairs written directly in the config.
	Inline []KeyPair
	// File is the path to a JSON array of key pairs:
	//
	//	[
	//	  {"access_key": "AKIAIOSFODNN7EXAMPLE", "secret_key": "wJalrXUt..."},
	//	  {"access_key": "ANOTHER_KEY", "secret_key": "another_secret"}
	//	]
	File string
}

// NewSecretStore builds a store from cfg, merging the inline keys with those in
// the file. A key present in both takes its secret from the file. Pairs missing
// an access or secret key are ignored.
func NewSecretStore(cfg KeysConfig) (sign.SecretStore, error) {
	keys := indexKeys(cfg.Inline)

	if cfg.File != "" {
		pairs, err := readKeysFile(cfg.File)
		if err != nil {
			return nil, err
		}
		maps.Copy(keys, indexKeys(pairs))
	}

	return NewMapSecretStore(keys), nil
}

// indexKeys maps pairs by access key, dropping any that is incomplete.
func indexKeys(pairs []KeyPair) map[string]string {
	keys := make(map[string]string, len(pairs))
	for _, p := range pairs {
		if p.AccessKey != "" && p.SecretKey != "" {
			keys[p.AccessKey] = p.SecretKey
		}
	}

	return keys
}

// readKeysFile reads the JSON array of key pairs at path.
func readKeysFile(path string) ([]KeyPair, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path comes from the trusted config file
	if err != nil {
		return nil, fmt.Errorf("read keys file: %w", err)
	}

	var pairs []KeyPair
	if err := json.Unmarshal(data, &pairs); err != nil {
		return nil, fmt.Errorf("parse keys file: %w", err)
	}

	return pairs, nil
}

// MapSecretStore looks access keys up in an in-memory map. It is safe for
// concurrent use as long as the map is not modified after construction, since
// Lookup only reads.
type MapSecretStore struct {
	keys map[string]string
}

var _ sign.SecretStore = (*MapSecretStore)(nil)

// NewMapSecretStore returns a store over a map of access key to secret key.
func NewMapSecretStore(keys map[string]string) *MapSecretStore {
	return &MapSecretStore{keys: keys}
}

// Lookup returns the secret key for accessKey, or ErrKeyNotFound.
func (s *MapSecretStore) Lookup(accessKey string) (string, error) {
	secretKey, found := s.keys[accessKey]
	if !found {
		return "", ErrKeyNotFound
	}

	return secretKey, nil
}
