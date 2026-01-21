package keybackend

import (
	"encoding/json"
	"fmt"
	"os"
)

// KeyPair represents an access key and secret key pair.
type KeyPair struct {
	AccessKey string `json:"access_key" mapstructure:"access_key"`
	SecretKey string `json:"secret_key" mapstructure:"secret_key"`
}

// LoadKeysFromFile loads access keys from a JSON file.
// The file should contain an array of key pairs:
//
//	[
//	  {"access_key": "AKIAIOSFODNN7EXAMPLE", "secret_key": "wJalrXUt..."},
//	  {"access_key": "ANOTHER_KEY", "secret_key": "another_secret"}
//	]
//
// Returns a map of access key to secret key.
func LoadKeysFromFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // Path is from trusted config file
	if err != nil {
		return nil, fmt.Errorf("read keys file: %w", err)
	}

	var pairs []KeyPair
	if err := json.Unmarshal(data, &pairs); err != nil {
		return nil, fmt.Errorf("parse keys file: %w", err)
	}

	keys := make(map[string]string, len(pairs))
	for _, p := range pairs {
		if p.AccessKey != "" && p.SecretKey != "" {
			keys[p.AccessKey] = p.SecretKey
		}
	}

	return keys, nil
}
