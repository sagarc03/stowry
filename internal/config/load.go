package config

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/sagarc03/stowry/types"
)

const envPrefix = "STOWRY"

// flagToKey maps a CLI flag to the key it overrides; unlisted flags override
// the key of the same name.
var flagToKey = map[string]string{
	"port":             "server.port",
	"mode":             "server.mode",
	"max-upload-size":  "server.max_upload_size",
	"error-document":   "server.error_document",
	"cleanup-timeout":  "service.cleanup_timeout",
	"db-type":          "database.type",
	"db-dsn":           "database.dsn",
	"db-table":         "database.tables.meta_data",
	"migrate":          "database.migrate",
	"storage-path":     "storage.path",
	"populate":         "storage.populate",
	"auth-read":        "auth.read",
	"auth-write":       "auth.write",
	"access-key":       "auth.access_key",
	"secret-key":       "auth.secret_key",
	"keys-file":        "auth.keys.file",
	"aws-region":       "auth.aws.region",
	"aws-service":      "auth.aws.service",
	"cors-origins":     "cors.allowed_origins",
	"cors-methods":     "cors.allowed_methods",
	"cors-headers":     "cors.allowed_headers",
	"cors-expose":      "cors.exposed_headers",
	"cors-credentials": "cors.allow_credentials",
	"cors-max-age":     "cors.max_age",
	"log-level":        "log.level",
}

// envOverride names the environment variable for keys that do not take the one
// their path spells, so the credentials are not STOWRY_AUTH_ACCESS_KEY. The
// path form does not also work: setDefaults binds one name per key.
var envOverride = map[string]string{
	"auth.access_key": envPrefix + "_ACCESS_KEY",
	"auth.secret_key": envPrefix + "_SECRET_KEY",
}

// EnvVarForKey returns the environment variable that reaches key.
func EnvVarForKey(key string) string {
	if name, ok := envOverride[key]; ok {
		return name
	}

	return envPrefix + "_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
}

// EnvVar returns the environment variable that reaches the same setting as the
// named flag, so help text cannot drift from what Load actually honours.
func EnvVar(flag string) string {
	key := flag
	if mapped, ok := flagToKey[flag]; ok {
		key = mapped
	}

	return EnvVarForKey(key)
}

// Load reads the configuration. Later sources win: flags, then environment,
// then the files in order, then Defaults. A missing file is only a warning.
func Load(files []string, flags *pflag.FlagSet) (*Config, error) {
	v := viper.New()

	if err := setDefaults(v); err != nil {
		return nil, err
	}

	readFiles(v, files)

	if flags != nil {
		bindFlags(v, flags)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg, viper.DecodeHook(decodeHook())); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// setDefaults registers every leaf of Defaults with viper, zero values
// included, and binds the environment variable that reaches it. A field missing
// from Defaults is therefore unreachable from the environment.
//
// The variables are bound by name rather than left to AutomaticEnv, which
// derives one from the key path and cannot be told otherwise. Binding is what
// lets envOverride hold, and it gives every setting exactly one variable: with
// AutomaticEnv also applied, an overridden key would answer to both names and
// the derived one would win.
func setDefaults(v *viper.Viper) error {
	var nested map[string]any
	if err := mapstructure.Decode(Defaults(), &nested); err != nil {
		return fmt.Errorf("encode defaults: %w", err)
	}

	for key, value := range flatten("", nested) {
		v.SetDefault(key, value)

		if err := v.BindEnv(key, EnvVarForKey(key)); err != nil {
			return fmt.Errorf("bind %s: %w", EnvVarForKey(key), err)
		}
	}

	return nil
}

// flatten turns nested maps into the dotted keys viper indexes by.
func flatten(prefix string, m map[string]any) map[string]any {
	out := make(map[string]any, len(m))

	for k, value := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		if nested, ok := value.(map[string]any); ok {
			for nk, nv := range flatten(key, nested) {
				out[nk] = nv
			}
			continue
		}

		out[key] = value
	}

	return out
}

// readFiles merges files in order, each overriding the last.
func readFiles(v *viper.Viper, files []string) {
	if len(files) == 0 {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")

		var notFound viper.ConfigFileNotFoundError
		if err := v.ReadInConfig(); err != nil && !errors.As(err, &notFound) {
			slog.Warn("read config file", "error", err)
		}

		return
	}

	for i, file := range files {
		v.SetConfigFile(file)

		merge := v.MergeInConfig
		if i == 0 {
			merge = v.ReadInConfig
		}

		if err := merge(); err != nil {
			slog.Warn("read config file", "file", file, "error", err)
		}
	}
}

// bindFlags binds only the flags the caller set, so an unset flag cannot
// override the environment or the file with its zero value.
func bindFlags(v *viper.Viper, flags *pflag.FlagSet) {
	flags.VisitAll(func(f *pflag.Flag) {
		if !f.Changed {
			return
		}

		key := f.Name
		if mapped, ok := flagToKey[key]; ok {
			key = mapped
		}

		_ = v.BindPFlag(key, f)
	})
}

// decodeHook replaces viper's default to add stringToSlice.
func decodeHook() mapstructure.DecodeHookFunc {
	return mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		stringToSlice(),
	)
}

// stringToSlice splits on commas and trims, so "a, b" and "a,b" are one list.
func stringToSlice() mapstructure.DecodeHookFuncKind {
	return func(from, to reflect.Kind, data any) (any, error) {
		if from != reflect.String || to != reflect.Slice {
			return data, nil
		}

		raw, _ := data.(string)
		if strings.TrimSpace(raw) == "" {
			return []string{}, nil
		}

		parts := strings.Split(raw, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}

		return parts, nil
	}
}

// Validate reports every way cfg is unusable.
func Validate(cfg *Config) error {
	v := validator.New()

	if err := v.RegisterValidation("table_name", func(fl validator.FieldLevel) bool {
		return types.IsValidTableName(fl.Field().String())
	}); err != nil {
		return fmt.Errorf("register table_name validator: %w", err)
	}

	if err := v.Struct(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	return nil
}

// ValidateForServe rejects auth settings the configured mode cannot honour.
// Static and SPA modes serve browsers, which cannot sign requests, so no
// verifier is wired up: auth.read private there would serve everything publicly
// while the operator believed otherwise.
//
// It is separate from Load so the offline commands, which route no requests,
// still run on a config the server would reject.
func (c *Config) ValidateForServe() error {
	if c.Server.Mode == types.ModeStore {
		return nil
	}

	if c.Auth.Read.Private() {
		return fmt.Errorf(
			"auth.read is %q but server.mode is %q: %s mode cannot verify signed requests, "+
				"set auth.read to public to serve this content publicly, or use store mode",
			c.Auth.Read, c.Server.Mode, c.Server.Mode,
		)
	}

	// Writes are not routed in these modes, so this is inert rather than unsafe.
	if c.Auth.Write.Private() {
		slog.Warn("auth.write is ignored in this mode: writes are not served at all",
			"auth.write", c.Auth.Write, "mode", c.Server.Mode)
	}

	return nil
}

// FlagForKey returns the flag that overrides key, or "" when none does.
func FlagForKey(key string) string {
	for flag, k := range flagToKey {
		if k == key {
			return flag
		}
	}

	return ""
}
