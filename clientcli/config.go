package clientcli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultServer is the default server URL.
const DefaultServer = "http://localhost:5708"

// Config holds client configuration.
type Config struct {
	Server    string `yaml:"server" mapstructure:"server"`
	AccessKey string `yaml:"access_key" mapstructure:"access_key"`
	SecretKey string `yaml:"secret_key" mapstructure:"secret_key"`
}

// Validate checks if required fields are set.
// If Server is empty, it defaults to DefaultServer.
func (c *Config) Validate() error {
	if c.Server == "" {
		c.Server = DefaultServer
	}
	return nil
}

// ValidateWithAuth checks if required fields including credentials are set.
func (c *Config) ValidateWithAuth() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.AccessKey == "" {
		return errors.New("access key is required")
	}
	if c.SecretKey == "" {
		return errors.New("secret key is required")
	}
	return nil
}

// LoadConfigFromFile loads config from a YAML file.
func LoadConfigFromFile(path string) (*Config, error) {
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath) //#nosec G304 -- path is user-provided config file
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	return &cfg, nil
}

// DefaultConfigPath returns the default config file path (~/.stowry/config.yaml).
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".stowry", "config.yaml")
}

// MergeConfig merges multiple configs, with later configs taking precedence.
// Empty strings in later configs do not override non-empty values in earlier configs.
func MergeConfig(configs ...*Config) *Config {
	result := &Config{}
	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		if cfg.Server != "" {
			result.Server = cfg.Server
		}
		if cfg.AccessKey != "" {
			result.AccessKey = cfg.AccessKey
		}
		if cfg.SecretKey != "" {
			result.SecretKey = cfg.SecretKey
		}
	}
	return result
}

// ConfigFromEnv loads config from environment variables.
func ConfigFromEnv() *Config {
	return &Config{
		Server:    os.Getenv("STOWRY_SERVER"),
		AccessKey: os.Getenv("STOWRY_ACCESS_KEY"),
		SecretKey: os.Getenv("STOWRY_SECRET_KEY"),
	}
}
