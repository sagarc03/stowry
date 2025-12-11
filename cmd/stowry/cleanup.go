package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/filesystem"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up soft-deleted files",
	Long: `Permanently remove soft-deleted files from storage.

This command processes all files that have been soft-deleted (marked for deletion)
but not yet physically removed from storage. It:
  1. Deletes the physical file from storage
  2. Marks the metadata entry as cleaned up

Run this periodically to reclaim storage space from deleted files.`,
	RunE: runCleanup,
}

var cleanupLimit int

func init() {
	cleanupCmd.Flags().IntVar(&cleanupLimit, "limit", 100, "maximum number of files to clean up per batch")
	rootCmd.AddCommand(cleanupCmd)
}

func runCleanup(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	repo, closeDB, err := initDB(ctx)
	if err != nil {
		return fmt.Errorf("init db: %w", err)
	}
	defer closeDB()

	storagePath := viper.GetString("storage.path")
	_, err = os.Stat(storagePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("storage directory does not exist: %s", storagePath)
	}

	root, err := os.OpenRoot(storagePath)
	if err != nil {
		return fmt.Errorf("open storage root: %w", err)
	}
	defer func() { _ = root.Close() }()

	storage := filesystem.NewFileStorage(root)

	service, err := stowry.NewStowryService(repo, storage, stowry.ModeStore)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	slog.Info("starting cleanup", "limit", cleanupLimit)

	cleaned, err := service.Tombstone(ctx, stowry.ListQuery{Limit: cleanupLimit})
	if err != nil {
		return fmt.Errorf("tombstone: %w", err)
	}

	slog.Info("cleanup complete", "files_cleaned", cleaned)
	return nil
}
