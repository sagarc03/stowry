package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagarc03/stowry/internal/config"
	"github.com/sagarc03/stowry/internal/database"
	"github.com/sagarc03/stowry/internal/service"
)

func newPopulateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "populate",
		GroupID: groupServer,
		Short:   "Record the files already in the storage directory",
		Long: `Record metadata for the files already in the storage directory, so a
directory of existing files can be served without uploading anything.

The files are read, never written: their layout under the storage path becomes
the object paths.`,
		Args: cobra.NoArgs,
		RunE: runPopulate,
	}

	d := config.Defaults()
	cmd.Flags().BoolP("migrate", "m", false,
		usage("migrate", "create the metadata schema first", d.Database.Migrate))

	return cmd
}

func runPopulate(cmd *cobra.Command, _ []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	warnEphemeral(cfg)

	ctx := cmd.Context()

	db, err := openDatabase(ctx, cfg, cfg.Database.Migrate)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	return populateInto(ctx, cmd, cfg, db)
}

// populateInto records the storage directory against an already-prepared store,
// which is what lets migrate --populate reuse the connection it just readied.
func populateInto(ctx context.Context, cmd *cobra.Command, cfg *config.Config, db database.Database) error {
	storage, err := openStorage(cfg.Storage.Path)
	if err != nil {
		return err
	}

	entries, err := service.New(db, storage).Populate(ctx)

	// Populate returns what it wrote before a failure, so the count is
	// reported either way.
	fmt.Fprintf(cmd.OutOrStdout(), "recorded %d files from %s\n", len(entries), cfg.Storage.Path)

	return err
}
