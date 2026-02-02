package main

import (
	"context"
	"os"

	"github.com/sagarc03/stowry/clientcli"
	"github.com/spf13/cobra"
)

var (
	uploadRecursive   bool
	uploadContentType string
)

var uploadCmd = &cobra.Command{
	Use:   "upload <local-path> [remote-path]",
	Short: "Upload files to the server",
	Long: `Upload files to the server.

Works in all server modes (store, static, spa).

If remote-path is omitted, the local path is used (normalized):
  ./foo/bar.txt       -> foo/bar.txt
  /abs/path/file.txt  -> abs/path/file.txt
  ../sibling/file.txt -> sibling/file.txt

Examples:
  stowry-cli upload ./file.txt
  stowry-cli upload ./images/photo.jpg
  stowry-cli upload -r ./images/
  stowry-cli upload ./file.txt custom/path.txt
  stowry-cli upload -r ./local/images/ remote/media/`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runUpload,
}

func init() {
	uploadCmd.Flags().BoolVarP(&uploadRecursive, "recursive", "r", false, "upload directory recursively")
	uploadCmd.Flags().StringVarP(&uploadContentType, "content-type", "t", "", "override content-type")
}

func runUpload(_ *cobra.Command, args []string) error {
	localPath := args[0]

	// Derive remote path from local path if not specified
	remotePath := ""
	if len(args) > 1 {
		remotePath = args[1]
	} else {
		remotePath = clientcli.NormalizeLocalToRemotePath(localPath)
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	opts := clientcli.UploadOptions{
		LocalPath:   localPath,
		RemotePath:  remotePath,
		ContentType: uploadContentType,
		Recursive:   uploadRecursive,
	}

	results, err := client.Upload(context.Background(), opts)
	if err != nil {
		return handleError(os.Stderr, err)
	}

	formatter := getFormatter()
	if err := formatter.FormatUpload(os.Stdout, results); err != nil {
		return err
	}

	// Check for any errors in results
	for i := range results {
		if results[i].Err != nil {
			return results[i].Err
		}
	}

	return nil
}
