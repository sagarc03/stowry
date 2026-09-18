// Package cli implements the stowry command line.
package cli

import (
	"fmt"

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
			files, err := cmd.Flags().GetStringSlice("config")
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
	flags.StringSliceP("config", "c", nil, "config file paths, merged left to right (default ./config.yaml)")
	flags.String("db-type", "", fmt.Sprintf("database type: sqlite or postgres (default %s)", d.Database.Type))
	flags.StringP("db-dsn", "d", "", fmt.Sprintf("database connection string (default %q)", d.Database.DSN))
	flags.StringP("storage-path", "s", "", fmt.Sprintf("storage directory path (default %q)", d.Storage.Path))

	cmd.AddCommand(newServeCmd(), newPopulateCmd())

	return cmd
}
