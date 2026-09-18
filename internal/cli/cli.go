// Package cli implements the stowry command line.
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagarc03/stowry/internal/config"
)

// Execute runs the command named on the command line. The error it returns has
// already been reported to stderr by cobra.
func Execute(version string) error {
	return newRootCmd(version).Execute()
}

func newRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "stowry",
		Version: version,
		Short:   "Self-hosted object storage, simplified",
		Long: `Stowry is a lightweight object storage server. Deploy a single binary,
configure your metadata backend, and start storing files with secure presigned
URL authentication.

It serves three modes - store, static and spa - over SQLite or PostgreSQL, and
is compatible with the AWS SDKs for generating presigned URLs.`,
		// A failure while running a command is not a usage error.
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			files, err := configFiles(cmd)
			if err != nil {
				return err
			}

			cfg, err := config.Load(files, cmd.Flags())
			if err != nil {
				return err
			}

			setupLogging(cfg.Log.Level)
			cmd.SetContext(config.WithContext(cmd.Context(), cfg))

			return nil
		},
	}

	// The flags themselves default to zero so that an unset one never overrides
	// the file or the environment; the real defaults are config.Defaults, and
	// the usage text is built from them so the two cannot drift.
	d := config.Defaults()

	flags := cmd.PersistentFlags()
	flags.StringSliceP("config", "c", nil,
		usage("config", "config file paths, merged left to right", "./config.yaml"))
	flags.String("db-type", "", usage("db-type", "database type: sqlite or postgres", d.Database.Type))
	flags.StringP("db-dsn", "d", "", usage("db-dsn", "database connection string", quoted(d.Database.DSN)))
	flags.StringP("storage-path", "s", "", usage("storage-path", "storage directory path", quoted(d.Storage.Path)))

	cmd.AddCommand(newServeCmd(), newPopulateCmd())

	return cmd
}

// configFiles returns the config files to read, falling back to the
// environment. This one setting cannot go through Load the way the others do:
// it is what Load reads, so nothing has parsed the environment yet when it is
// needed. A set flag still wins, as it does everywhere else.
func configFiles(cmd *cobra.Command) ([]string, error) {
	if cmd.Flags().Changed("config") {
		return cmd.Flags().GetStringSlice("config")
	}

	var files []string

	for _, f := range strings.Split(os.Getenv(config.EnvVar("config")), ",") {
		if f = strings.TrimSpace(f); f != "" {
			files = append(files, f)
		}
	}

	return files, nil
}

// usage builds a flag's help text. A flag is only one of three ways to reach a
// setting, so it names the environment variable and the default too.
func usage(flag, desc string, def any) string {
	return fmt.Sprintf("%s [%s] (default %v)", desc, config.EnvVar(flag), def)
}

func quoted(s string) string { return fmt.Sprintf("%q", s) }
