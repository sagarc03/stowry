package cli

import (
	"github.com/spf13/cobra"

	"github.com/sagarc03/stowry/internal/config"
)

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "migrate",
		GroupID: groupServer,
		Short:   "Create the metadata schema",
		Long: `Create the tables and indexes stowry needs, then confirm the result.

Run this before serving from a database on disk: serve migrates an in-memory
database only.

It is idempotent, and it never alters a table that already exists. A table
stowry did not build is reported rather than changed, so migrating onto an
older or foreign schema fails instead of half-working.`,
		Args: cobra.NoArgs,
		RunE: runMigrate,
	}

	d := config.Defaults()
	cmd.Flags().BoolP("populate", "p", false,
		usage("populate", "record the files already in the storage directory once the schema is in place", d.Storage.Populate))

	return cmd
}

func runMigrate(cmd *cobra.Command, _ []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	warnEphemeral(cfg)

	ctx := cmd.Context()

	db, err := openDatabase(ctx, cfg, true)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if !cfg.Storage.Populate {
		return nil
	}

	return populateInto(ctx, cmd, cfg, db)
}
