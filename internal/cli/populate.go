package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sagarc03/stowry/internal/config"
	"github.com/sagarc03/stowry/internal/database"
	"github.com/sagarc03/stowry/internal/service"
)

func newPopulateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "populate",
		Short: "Record the files already in the storage directory",
		Long: `Record metadata for the files already in the storage directory, so a
directory of existing files can be served without uploading anything.

The files are read, never written: their layout under the storage path becomes
the object paths.`,
		Args: cobra.NoArgs,
		RunE: runPopulate,
	}
}

func runPopulate(cmd *cobra.Command, _ []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	root := cfg.Storage.Path

	// In-memory storage holds no files to record, and the default config asks
	// for it, so this is a no-op rather than a failure.
	if root == config.MemoryPath {
		return nil
	}

	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("read storage directory %s: %w", root, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("storage path %s is not a directory", root)
	}

	ctx := cmd.Context()

	db, err := database.Connect(ctx, cfg.DatabaseConfig())
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Unlike serve, this command is the one that sets a store up, so it brings
	// the schema with it.
	if err := db.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	storage, err := openStorage(root)
	if err != nil {
		return err
	}

	entries, err := service.New(db, storage).Populate(ctx)

	// Populate returns what it wrote before a failure, so the count is
	// reported either way.
	fmt.Fprintf(cmd.OutOrStdout(), "recorded %d files from %s\n", len(entries), root)

	return err
}
