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
	"db-type":      "database.type",
	"db-dsn":       "database.dsn",
	"storage-path": "storage.path",
	"port":         "server.port",
	"mode":         "server.mode",
}

// Load reads the configuration. Later sources win: flags, then environment,
// then the files in order, then Defaults. A missing file is only a warning.
func Load(files []string, flags *pflag.FlagSet) (*Config, error) {
	v := viper.New()

	if err := setDefaults(v); err != nil {
		return nil, err
	}

	readFiles(v, files)

	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

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
// included. That is what lets the environment reach any setting: viper resolves
// an environment variable during Unmarshal only for a key it already knows.
func setDefaults(v *viper.Viper) error {
	var nested map[string]any
	if err := mapstructure.Decode(Defaults(), &nested); err != nil {
		return fmt.Errorf("encode defaults: %w", err)
	}

	for key, value := range flatten("", nested) {
		v.SetDefault(key, value)
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
