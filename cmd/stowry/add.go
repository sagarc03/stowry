package main

import (
	"fmt"
	"io/fs"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/config"
	"github.com/sagarc03/stowry/database"
	"github.com/sagarc03/stowry/filesystem"
)

var addCmd = &cobra.Command{
	Use:   "add [flags] <file1> [file2] ...",
	Short: "Import files into stowry storage",
	Long: `Import files from external paths into stowry storage.

This command copies files to the storage directory and registers
their metadata in the database. Files are identified by their
destination path in storage.

Examples:
  # Add a single file
  stowry add /path/to/file.txt

  # Add with a destination prefix
  stowry add --dest images/ /path/to/photo.jpg

  # Add a directory recursively
  stowry add -r /path/to/assets

  # Skip existing files
  stowry add --no-clobber /path/to/file.txt`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAdd,
}

var (
	addDest      string
	addRecursive bool
	addNoClobber bool
	addQuiet     bool
)

func init() {
	addCmd.Flags().StringVarP(&addDest, "dest", "d", "", "destination path prefix in storage")
	addCmd.Flags().BoolVarP(&addRecursive, "recursive", "r", false, "recursively add directories")
	addCmd.Flags().BoolVarP(&addNoClobber, "no-clobber", "n", false, "skip existing files instead of overwriting")
	addCmd.Flags().BoolVarP(&addQuiet, "quiet", "q", false, "suppress per-file output")
	rootCmd.AddCommand(addCmd)
}

// fileEntry represents a file to be added with its source and destination paths.
type fileEntry struct {
	sourcePath string
	destPath   string
}

func runAdd(cmd *cobra.Command, args []string) error {
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

	// Collect files from all arguments
	var files []fileEntry
	for _, arg := range args {
		entries, collectErr := collectFiles(arg, addRecursive, addDest)
		if collectErr != nil {
			return fmt.Errorf("collect files from %s: %w", arg, collectErr)
		}
		files = append(files, entries...)
	}

	if len(files) == 0 {
		slog.Info("no files to add")
		return nil
	}

	added := 0
	skipped := 0

	for _, entry := range files {
		// Check if file exists when --no-clobber is set
		if addNoClobber {
			_, getErr := repo.Get(ctx, entry.destPath)
			if getErr == nil {
				skipped++
				if !addQuiet {
					slog.Info("skipped (exists)", "path", entry.destPath)
				}
				continue
			}
		}

		// Open source file
		f, openErr := os.Open(entry.sourcePath)
		if openErr != nil {
			return fmt.Errorf("open %s: %w", entry.sourcePath, openErr)
		}

		contentType := detectContentType(entry.sourcePath)

		obj := stowry.CreateObject{
			Path:        entry.destPath,
			ContentType: contentType,
		}

		_, createErr := service.Create(ctx, obj, f)
		_ = f.Close()

		if createErr != nil {
			return fmt.Errorf("add %s: %w", entry.destPath, createErr)
		}

		added++
		if !addQuiet {
			slog.Info("added", "path", entry.destPath, "content_type", contentType)
		}
	}

	slog.Info("add complete", "added", added, "skipped", skipped)
	return nil
}

// collectFiles gathers files from a path, optionally recursively.
// Returns a list of file entries with source and destination paths.
func collectFiles(path string, recursive bool, destPrefix string) ([]fileEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Normalize dest prefix - ensure it ends with / if non-empty
	destPrefix = strings.TrimPrefix(destPrefix, "/")
	if destPrefix != "" && !strings.HasSuffix(destPrefix, "/") {
		destPrefix += "/"
	}

	if !info.IsDir() {
		// Single file
		destPath := destPrefix + filepath.Base(path)
		return []fileEntry{{sourcePath: path, destPath: destPath}}, nil
	}

	// Directory
	if !recursive {
		return nil, fmt.Errorf("%s is a directory (use -r to add recursively)", path)
	}

	var entries []fileEntry
	walkErr := filepath.WalkDir(path, func(walkPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			return nil
		}

		// Compute relative path from the base directory
		relPath, relErr := filepath.Rel(path, walkPath)
		if relErr != nil {
			return relErr
		}

		// Convert to forward slashes for storage path
		destPath := destPrefix + filepath.ToSlash(relPath)

		entries = append(entries, fileEntry{
			sourcePath: walkPath,
			destPath:   destPath,
		})
		return nil
	})

	if walkErr != nil {
		return nil, walkErr
	}

	return entries, nil
}

// detectContentType determines the MIME type from a file's extension.
func detectContentType(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return "application/octet-stream"
	}

	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		return "application/octet-stream"
	}

	return contentType
}
