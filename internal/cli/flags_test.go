package cli

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sagarc03/stowry/internal/config"
	"github.com/sagarc03/stowry/types"
)

// Every config setting is settable from a flag. config.TestFlagReachesEverySetting
// pins that each key has one; this pins that each one arrives, with its type
// intact through viper's decode.
func TestFlagsReachConfig(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want any
		got  func(*config.Config) any
	}{
		{"port", []string{"--port", "9090"}, 9090, func(c *config.Config) any { return c.Server.Port }},
		{"mode", []string{"--mode", "spa"}, types.ModeSPA, func(c *config.Config) any { return c.Server.Mode }},
		{"max-upload-size", []string{"--max-upload-size", "1048576"}, int64(1048576), func(c *config.Config) any { return c.Server.MaxUploadSize }},
		{"error-document", []string{"--error-document", "404.html"}, "404.html", func(c *config.Config) any { return c.Server.ErrorDocument }},
		{"cleanup-timeout", []string{"--cleanup-timeout", "45s"}, 45 * time.Second, func(c *config.Config) any { return c.Service.CleanupTimeout }},
		{"db-type", []string{"--db-type", "postgres"}, "postgres", func(c *config.Config) any { return c.Database.Type }},
		{"db-dsn", []string{"--db-dsn", "pg://x"}, "pg://x", func(c *config.Config) any { return c.Database.DSN }},
		{"db-table", []string{"--db-table", "custom_meta"}, "custom_meta", func(c *config.Config) any { return c.Database.Tables.MetaData }},
		{"migrate", []string{"--migrate"}, true, func(c *config.Config) any { return c.Database.Migrate }},
		{"storage-path", []string{"--storage-path", "/data"}, "/data", func(c *config.Config) any { return c.Storage.Path }},
		{"populate", []string{"--populate"}, true, func(c *config.Config) any { return c.Storage.Populate }},
		{"auth-read", []string{"--auth-read", "private"}, config.AccessPrivate, func(c *config.Config) any { return c.Auth.Read }},
		{"auth-write", []string{"--auth-write", "private"}, config.AccessPrivate, func(c *config.Config) any { return c.Auth.Write }},
		{"access-key", []string{"--access-key", "AKIA", "--secret-key", "shh"}, "AKIA", func(c *config.Config) any { return c.Auth.AccessKey }},
		{"secret-key", []string{"--access-key", "AKIA", "--secret-key", "shh"}, "shh", func(c *config.Config) any { return c.Auth.SecretKey }},
		{"keys-file", []string{"--keys-file", "/keys.json"}, "/keys.json", func(c *config.Config) any { return c.Auth.Keys.File }},
		{"aws-region", []string{"--aws-region", "eu-west-1"}, "eu-west-1", func(c *config.Config) any { return c.Auth.AWS.Region }},
		{"aws-service", []string{"--aws-service", "s4"}, "s4", func(c *config.Config) any { return c.Auth.AWS.Service }},
		{"cors-origins", []string{"--cors-origins", "https://a.example,https://b.example"}, []string{"https://a.example", "https://b.example"}, func(c *config.Config) any { return c.CORS.AllowedOrigins }},
		{"cors-methods", []string{"--cors-methods", "GET,PUT"}, []string{"GET", "PUT"}, func(c *config.Config) any { return c.CORS.AllowedMethods }},
		{"cors-headers", []string{"--cors-headers", "X-One"}, []string{"X-One"}, func(c *config.Config) any { return c.CORS.AllowedHeaders }},
		{"cors-expose", []string{"--cors-expose", "ETag"}, []string{"ETag"}, func(c *config.Config) any { return c.CORS.ExposedHeaders }},
		{"cors-credentials", []string{"--cors-credentials"}, true, func(c *config.Config) any { return c.CORS.AllowCredentials }},
		{"cors-max-age", []string{"--cors-max-age", "600"}, 600, func(c *config.Config) any { return c.CORS.MaxAge }},
		{"log-level", []string{"--log-level", "debug"}, "debug", func(c *config.Config) any { return c.Log.Level }},
	}

	// serve inherits every persistent flag, so its set is the widest one.
	serve := func(t *testing.T) *cobra.Command {
		t.Helper()

		for _, c := range newRootCmd("test").Commands() {
			if c.Name() == "serve" {
				return c
			}
		}

		t.Fatal("no serve command")

		return nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := serve(t)
			require.NoError(t, cmd.ParseFlags(tt.args))

			cfg, err := config.Load(nil, cmd.Flags())
			require.NoError(t, err)

			assert.Equal(t, tt.want, tt.got(cfg))
		})
	}
}
