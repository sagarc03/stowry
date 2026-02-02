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
	Use:   "upload <local-path> <remote-path>",
	Short: "Upload files to the server",
	Long: `Upload files to the server.

Works in all server modes (store, static, spa).

Examples:
  stowry-cli upload ./file.txt path/file.txt
  stowry-cli upload -r ./images/ media/images/
  stowry-cli upload --content-type application/json ./data config.json`,
	Args: cobra.ExactArgs(2),
	RunE: runUpload,
}

func init() {
	uploadCmd.Flags().BoolVarP(&uploadRecursive, "recursive", "r", false, "upload directory recursively")
	uploadCmd.Flags().StringVarP(&uploadContentType, "content-type", "t", "", "override content-type")
}

func runUpload(_ *cobra.Command, args []string) error {
	localPath := args[0]
	remotePath := args[1]

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
		formatter := getFormatter()
		_ = formatter.FormatError(os.Stderr, err)
		return err
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
