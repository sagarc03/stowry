package main

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/sagarc03/stowry/clientcli"
	"github.com/spf13/cobra"
)

var (
	downloadOutput string
	downloadStdout bool
)

var downloadCmd = &cobra.Command{
	Use:   "download <remote-path> [local-path]",
	Short: "Download a file from the server",
	Long: `Download a file from the server.

The download behavior depends on the server mode:
  - store:  Returns the file, or 404 if not found
  - static: Returns the file, tries path/index.html for directories, or 404
  - spa:    Returns the file, or falls back to /index.html for missing paths

Examples:
  stowry-cli download path/file.txt
  stowry-cli download path/file.txt ./local-file.txt
  stowry-cli download --stdout config.json | jq .
  stowry-cli download -o ./output.txt path/file.txt`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runDownload,
}

func init() {
	downloadCmd.Flags().StringVarP(&downloadOutput, "output", "o", "", "output file path")
	downloadCmd.Flags().BoolVar(&downloadStdout, "stdout", false, "write to stdout")
}

func runDownload(_ *cobra.Command, args []string) error {
	remotePath := args[0]

	// Determine local path
	localPath := ""
	if len(args) > 1 {
		localPath = args[1]
	}
	if downloadOutput != "" {
		localPath = downloadOutput
	}
	if downloadStdout {
		localPath = "-"
	}

	// If no local path specified, derive from remote
	if localPath == "" {
		localPath = filepath.Base(remotePath)
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	opts := clientcli.DownloadOptions{
		RemotePath: remotePath,
		LocalPath:  localPath,
	}

	result, reader, err := client.Download(context.Background(), opts)
	if err != nil {
		return handleError(os.Stderr, err)
	}

	// If stdout, write content to stdout
	if reader != nil {
		defer func() { _ = reader.Close() }()
		_, err := io.Copy(os.Stdout, reader)
		if err != nil {
			return err
		}
		// Don't print metadata when writing to stdout (unless JSON mode)
		if jsonOutput {
			formatter := getFormatter()
			return formatter.FormatDownload(os.Stderr, result)
		}
		return nil
	}

	// Otherwise, format the result
	formatter := getFormatter()
	return formatter.FormatDownload(os.Stdout, result)
}
