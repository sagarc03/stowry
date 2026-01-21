package keybackend

import (
	"github.com/sagarc03/stowry"
)

// KeysConfig holds configuration for loading access keys.
type KeysConfig struct {
	Inline []KeyPair `mapstructure:"inline"` // Inline key pairs from config
	File   string    `mapstructure:"file"`   // Path to JSON file containing key pairs
}

// NewSecretStore creates a SecretStore from the given configuration.
// It loads keys from both inline config and file (if specified),
// merging them into a single store. File keys take precedence over inline keys
// if there are duplicates.
func NewSecretStore(cfg KeysConfig) (stowry.SecretStore, error) {
	keys := make(map[string]string)

	// Load inline keys
	for _, p := range cfg.Inline {
		if p.AccessKey != "" && p.SecretKey != "" {
			keys[p.AccessKey] = p.SecretKey
		}
	}

	// Load keys from file if specified
	if cfg.File != "" {
		fileKeys, err := LoadKeysFromFile(cfg.File)
		if err != nil {
			return nil, err
		}
		for k, v := range fileKeys {
			keys[k] = v
		}
	}

	return NewMapSecretStore(keys), nil
}
