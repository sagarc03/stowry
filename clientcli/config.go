package clientcli

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultEndpoint is the default server endpoint URL.
const DefaultEndpoint = "http://localhost:5708"

// Profile holds configuration for a single server profile.
type Profile struct {
	Name      string `yaml:"name"`
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key,omitempty"`
	SecretKey string `yaml:"secret_key,omitempty"`
	Default   bool   `yaml:"default,omitempty"`
}

// ConfigFile holds the full config file structure with multiple profiles.
type ConfigFile struct {
	Profiles []Profile `yaml:"profiles"`
}

// GetProfile returns the profile by name.
// If name is empty, returns the default profile.
func (c *ConfigFile) GetProfile(name string) (*Profile, error) {
	if len(c.Profiles) == 0 {
		return nil, ErrNoProfiles
	}

	if name == "" {
		return c.GetDefaultProfile()
	}

	for i := range c.Profiles {
		if c.Profiles[i].Name == name {
			return &c.Profiles[i], nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrProfileNotFound, name)
}

// GetDefaultProfile returns the default profile.
// If no profile is marked as default, returns the first profile.
func (c *ConfigFile) GetDefaultProfile() (*Profile, error) {
	if len(c.Profiles) == 0 {
		return nil, ErrNoProfiles
	}

	// Look for profile marked as default
	for i := range c.Profiles {
		if c.Profiles[i].Default {
			return &c.Profiles[i], nil
		}
	}

	// Return first profile if none marked as default
	return &c.Profiles[0], nil
}

// AddProfile adds a new profile. Returns ErrProfileExists if a profile
// with the same name already exists. Use UpdateProfile to modify an existing profile.
func (c *ConfigFile) AddProfile(p Profile) error {
	for i := range c.Profiles {
		if c.Profiles[i].Name == p.Name {
			return fmt.Errorf("%w: %s", ErrProfileExists, p.Name)
		}
	}
	c.Profiles = append(c.Profiles, p)
	return nil
}

// UpdateProfile updates an existing profile. Returns ErrProfileNotFound
// if the profile doesn't exist. Use AddProfile to create a new profile.
func (c *ConfigFile) UpdateProfile(p Profile) error {
	for i := range c.Profiles {
		if c.Profiles[i].Name == p.Name {
			c.Profiles[i] = p
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrProfileNotFound, p.Name)
}

// RemoveProfile removes a profile by name.
func (c *ConfigFile) RemoveProfile(name string) error {
	for i := range c.Profiles {
		if c.Profiles[i].Name == name {
			c.Profiles = append(c.Profiles[:i], c.Profiles[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrProfileNotFound, name)
}

// SetDefault sets the default profile by name.
// Clears the default flag from all other profiles.
func (c *ConfigFile) SetDefault(name string) error {
	found := false
	for i := range c.Profiles {
		if c.Profiles[i].Name == name {
			c.Profiles[i].Default = true
			found = true
		} else {
			c.Profiles[i].Default = false
		}
	}

	if !found {
		return fmt.Errorf("%w: %s", ErrProfileNotFound, name)
	}
	return nil
}

// ProfileNames returns a list of all profile names.
func (c *ConfigFile) ProfileNames() []string {
	names := make([]string, len(c.Profiles))
	for i := range c.Profiles {
		names[i] = c.Profiles[i].Name
	}
	return names
}

// Save writes the config to the specified path.
// Creates the parent directory if it doesn't exist.
func (c *ConfigFile) Save(path string) error {
	cleanPath := filepath.Clean(path)

	// Create parent directory if needed
	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(cleanPath, data, 0o600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// LoadConfigFile loads the config file from the specified path.
func LoadConfigFile(path string) (*ConfigFile, error) {
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath) //#nosec G304 -- path is user-provided config file
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg ConfigFile
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

// Config holds resolved client configuration for a single server.
// This is what the Client uses after profile resolution.
type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
}

// Validate checks if required fields are set.
// Use WithDefaults() to get a config with default values applied.
func (c *Config) Validate() error {
	// Validation only - no mutation
	return nil
}

// WithDefaults returns a copy of the config with default values applied.
// If Endpoint is empty, it defaults to DefaultEndpoint.
func (c *Config) WithDefaults() *Config {
	cfg := *c
	if cfg.Endpoint == "" {
		cfg.Endpoint = DefaultEndpoint
	}
	return &cfg
}

// ValidateWithAuth checks if required fields including credentials are set.
func (c *Config) ValidateWithAuth() error {
	if c.AccessKey == "" {
		return ErrAccessKeyRequired
	}
	if c.SecretKey == "" {
		return ErrSecretKeyRequired
	}
	return nil
}

// ConfigFromProfile creates a Config from a Profile.
func ConfigFromProfile(p *Profile) *Config {
	if p == nil {
		return &Config{}
	}
	return &Config{
		Endpoint:  p.Endpoint,
		AccessKey: p.AccessKey,
		SecretKey: p.SecretKey,
	}
}

// ConfigFromEnv loads config from environment variables.
func ConfigFromEnv() *Config {
	return &Config{
		Endpoint:  os.Getenv("STOWRY_ENDPOINT"),
		AccessKey: os.Getenv("STOWRY_ACCESS_KEY"),
		SecretKey: os.Getenv("STOWRY_SECRET_KEY"),
	}
}

// ProfileFromEnv returns the profile name from STOWRY_PROFILE environment variable.
func ProfileFromEnv() string {
	return os.Getenv("STOWRY_PROFILE")
}

// ConfigPathFromEnv returns the config file path from STOWRY_CONFIG environment variable.
func ConfigPathFromEnv() string {
	return os.Getenv("STOWRY_CONFIG")
}

// MergeConfig merges multiple configs, with later configs taking precedence.
// Empty strings in later configs do not override non-empty values in earlier configs.
func MergeConfig(configs ...*Config) *Config {
	result := &Config{}
	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		if cfg.Endpoint != "" {
			result.Endpoint = cfg.Endpoint
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
