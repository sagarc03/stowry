package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/config"
	"github.com/sagarc03/stowry/database"
	"github.com/sagarc03/stowry/filesystem"
)

var removeCmd = &cobra.Command{
	Use:   "remove [flags] <path1> [path2] ...",
	Short: "Remove files from stowry storage",
	Long: `Soft-delete files from stowry storage by marking them for removal.

This command marks files as deleted in the metadata database. Physical
file removal happens later via 'stowry cleanup'.

Examples:
  # Remove a single file
  stowry remove myfile.txt

  # Remove multiple files
  stowry remove file1.txt file2.txt file3.txt

  # Remove all files with a prefix (e.g., a directory)
  stowry remove --prefix images/

  # Remove quietly (suppress per-file output)
  stowry remove -q file.txt`,
	Args: cobra.MinimumNArgs(1),
	RunE: runRemove,
}

var (
	removePrefix bool
	removeQuiet  bool
)

func init() {
	removeCmd.Flags().BoolVarP(&removePrefix, "prefix", "p", false, "treat paths as prefixes and remove all matching files")
	removeCmd.Flags().BoolVarP(&removeQuiet, "quiet", "q", false, "suppress per-file output")
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	db, err := database.Connect(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err = db.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	if err = db.Validate(ctx); err != nil {
		return fmt.Errorf("validate database schema: %w", err)
	}

	repo := db.GetRepo()

	_, err = os.Stat(cfg.Storage.Path)
	if os.IsNotExist(err) {
		return fmt.Errorf("storage directory does not exist: %s", cfg.Storage.Path)
	}

	root, err := os.OpenRoot(cfg.Storage.Path)
	if err != nil {
		return fmt.Errorf("open storage root: %w", err)
	}
	defer func() { _ = root.Close() }()

	storage := filesystem.NewFileStorage(root)

	serviceCfg := stowry.ServiceConfig{Mode: stowry.ModeStore}
	service, err := stowry.NewStowryService(repo, storage, serviceCfg)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	removed := 0
	notFound := 0

	for _, path := range args {
		if removePrefix {
			// Prefix mode: list all files matching the prefix and delete each
			count, nfCount, prefixErr := removeByPrefix(ctx, service, path)
			if prefixErr != nil {
				return prefixErr
			}
			removed += count
			notFound += nfCount
		} else {
			// Direct mode: delete the specific path
			deleteErr := service.Delete(ctx, path)
			if errors.Is(deleteErr, stowry.ErrNotFound) {
				notFound++
				if !removeQuiet {
					slog.Warn("not found", "path", path)
				}
				continue
			}
			if deleteErr != nil {
				return fmt.Errorf("remove %s: %w", path, deleteErr)
			}
			removed++
			if !removeQuiet {
				slog.Info("removed", "path", path)
			}
		}
	}

	slog.Info("remove complete", "removed", removed, "not_found", notFound)
	return nil
}

// removeByPrefix removes all files matching the given prefix.
// Returns the count of removed files, not found count, and any error.
func removeByPrefix(ctx context.Context, service *stowry.StowryService, prefix string) (removed, notFound int, err error) {
	cursor := ""

	for {
		result, listErr := service.List(ctx, stowry.ListQuery{
			PathPrefix: prefix,
			Limit:      100,
			Cursor:     cursor,
		})
		if listErr != nil {
			return removed, notFound, fmt.Errorf("list prefix %s: %w", prefix, listErr)
		}

		if len(result.Items) == 0 {
			break
		}

		for _, item := range result.Items {
			deleteErr := service.Delete(ctx, item.Path)
			if errors.Is(deleteErr, stowry.ErrNotFound) {
				notFound++
				if !removeQuiet {
					slog.Warn("not found", "path", item.Path)
				}
				continue
			}
			if deleteErr != nil {
				return removed, notFound, fmt.Errorf("remove %s: %w", item.Path, deleteErr)
			}
			removed++
			if !removeQuiet {
				slog.Info("removed", "path", item.Path)
			}
		}

		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	return removed, notFound, nil
}
