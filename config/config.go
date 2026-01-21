package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/database"
	stowryhttp "github.com/sagarc03/stowry/http"
	"github.com/sagarc03/stowry/keybackend"
)

// configKey is the context key for storing the loaded configuration.
type configKey struct{}

// WithContext returns a new context with the config stored.
func WithContext(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, configKey{}, cfg)
}

// FromContext retrieves the config from context.
// Returns an error if config is not found.
func FromContext(ctx context.Context) (*Config, error) {
	cfg, ok := ctx.Value(configKey{}).(*Config)
	if !ok || cfg == nil {
		return nil, errors.New("config not found in context")
	}
	return cfg, nil
}

// Config is the root configuration struct for stowry.
type Config struct {
	Server   ServerConfig          `mapstructure:"server"`
	Service  ServiceConfig         `mapstructure:"service"`
	Database database.Config       `mapstructure:"database"`
	Storage  StorageConfig         `mapstructure:"storage"`
	Auth     AuthConfig            `mapstructure:"auth"`
	CORS     stowryhttp.CORSConfig `mapstructure:"cors"`
	Log      LogConfig             `mapstructure:"log"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port          int    `mapstructure:"port" validate:"required,min=1,max=65535"`
	Mode          string `mapstructure:"mode" validate:"required,oneof=store static spa"`
	MaxUploadSize int64  `mapstructure:"max_upload_size" validate:"min=0"`
}

// ServiceConfig holds service-level configuration.
type ServiceConfig struct {
	CleanupTimeout int `mapstructure:"cleanup_timeout" validate:"min=1"`
}

// StorageConfig holds file storage configuration.
type StorageConfig struct {
	Path string `mapstructure:"path" validate:"required"`
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Read  string                `mapstructure:"read" validate:"required,oneof=public private"`
	Write string                `mapstructure:"write" validate:"required,oneof=public private"`
	AWS   stowry.AWSConfig      `mapstructure:"aws"`
	Keys  keybackend.KeysConfig `mapstructure:"keys"`
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level string `mapstructure:"level" validate:"required,oneof=debug info warn error"`
}

// flagToViperKey maps CLI flag names to viper configuration keys.
var flagToViperKey = map[string]string{
	"db-type":      "database.type",
	"db-dsn":       "database.dsn",
	"storage-path": "storage.path",
	"port":         "server.port",
	"mode":         "server.mode",
}

// bindFlags binds CLI flags to viper keys with custom name mapping.
func bindFlags(v *viper.Viper, flags *pflag.FlagSet) {
	flags.VisitAll(func(f *pflag.Flag) {
		// Use custom mapping if it exists, otherwise use flag name as-is
		viperKey := f.Name
		if mapped, ok := flagToViperKey[viperKey]; ok {
			viperKey = mapped
		}

		// Only bind if the flag was explicitly set
		if f.Changed {
			_ = v.BindPFlag(viperKey, f)
		}
	})
}

// setDefaults configures default values on the viper instance.
func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 5708)
	v.SetDefault("server.mode", "store")
	v.SetDefault("server.max_upload_size", 0) // 0 means no limit

	v.SetDefault("service.cleanup_timeout", 30) // seconds

	v.SetDefault("database.type", "sqlite")
	v.SetDefault("database.dsn", "stowry.db")
	v.SetDefault("database.tables.meta_data", "stowry_metadata")

	v.SetDefault("storage.path", "./data")

	v.SetDefault("auth.read", "public")
	v.SetDefault("auth.write", "public")
	v.SetDefault("auth.aws.region", "us-east-1")
	v.SetDefault("auth.aws.service", "s3")

	v.SetDefault("log.level", "info")
}

// Load reads configuration and returns a validated Config struct.
// Order of precedence (highest to lowest): flags > env > config files > defaults
//
// Parameters:
//   - configFiles: list of config file paths (later files override earlier ones)
//   - flags: cobra flag set for flag binding (can be nil)
func Load(configFiles []string, flags *pflag.FlagSet) (*Config, error) {
	v := viper.New()

	// 1. Set defaults
	setDefaults(v)

	// 2. Read config files
	if len(configFiles) > 0 {
		v.SetConfigFile(configFiles[0])
		if err := v.ReadInConfig(); err != nil {
			slog.Warn("error reading config file", "file", configFiles[0], "err", err)
		}

		for _, cf := range configFiles[1:] {
			v.SetConfigFile(cf)
			if err := v.MergeInConfig(); err != nil {
				slog.Warn("error merging config file", "file", cf, "err", err)
			}
		}
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")

		if err := v.ReadInConfig(); err != nil {
			var configNotFound viper.ConfigFileNotFoundError
			if !errors.As(err, &configNotFound) {
				slog.Warn("error reading config file", "err", err)
			}
		}
	}

	// 3. Bind environment variables
	v.SetEnvPrefix("STOWRY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 4. Bind flags (if provided)
	if flags != nil {
		bindFlags(v, flags)
	}

	// 5. Unmarshal into Config struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// 6. Validate using go-playground/validator
	validate := validator.New()
	if err := validate.Struct(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}
