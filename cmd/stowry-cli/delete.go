package main

import (
	"context"
	"os"

	"github.com/sagarc03/stowry/clientcli"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <remote-path> [remote-path...]",
	Short: "Delete files from the server",
	Long: `Delete one or more files from the server.

Works in all server modes (store, static, spa).

Examples:
  stowry-cli delete path/file.txt
  stowry-cli delete old/a.txt old/b.txt old/c.txt
  stowry-cli delete -q temp/file.txt`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDelete,
}

func runDelete(_ *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	opts := clientcli.DeleteOptions{
		Paths: args,
	}

	results, err := client.Delete(context.Background(), opts)
	if err != nil {
		formatter := getFormatter()
		_ = formatter.FormatError(os.Stderr, err)
		return err
	}

	formatter := getFormatter()
	if err := formatter.FormatDelete(os.Stdout, results); err != nil {
		return err
	}

	// Return error if any deletes failed
	if clientcli.HasDeleteErrors(results) {
		return &exitError{code: 1}
	}

	return nil
}

// exitError is returned when we want to exit with a specific code
// but don't want cobra to print an error message.
type exitError struct {
	code int
}

func (e *exitError) Error() string {
	return ""
}
