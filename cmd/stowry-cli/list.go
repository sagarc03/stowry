package main

import (
	"context"
	"os"

	"github.com/sagarc03/stowry/clientcli"
	"github.com/spf13/cobra"
)

var (
	listPrefix string
	listLimit  int
	listAll    bool
	listCursor string
)

var listCmd = &cobra.Command{
	Use:   "list [prefix]",
	Short: "List objects on the server",
	Long: `List objects on the server.

NOTE: This command only works when the server is running in "store" mode.
      In "static" or "spa" modes, this will return a 404 error.

Examples:
  stowry-cli list
  stowry-cli list images/
  stowry-cli list --prefix documents/ --limit 10
  stowry-cli list --all
  stowry-cli list --cursor "eyJwYXRoIjoi..."`,
	Args: cobra.MaximumNArgs(1),
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVar(&listPrefix, "prefix", "", "filter by path prefix")
	listCmd.Flags().IntVarP(&listLimit, "limit", "l", 100, "max results per page (max: 1000)")
	listCmd.Flags().BoolVar(&listAll, "all", false, "fetch all pages")
	listCmd.Flags().StringVar(&listCursor, "cursor", "", "pagination cursor")
}

func runList(_ *cobra.Command, args []string) error {
	// Prefix can come from positional arg or flag
	prefix := listPrefix
	if len(args) > 0 {
		prefix = args[0]
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	opts := clientcli.ListOptions{
		Prefix: prefix,
		Limit:  listLimit,
		Cursor: listCursor,
		All:    listAll,
	}

	result, err := client.List(context.Background(), opts)
	if err != nil {
		formatter := getFormatter()
		_ = formatter.FormatError(os.Stderr, err)
		return err
	}

	formatter := getFormatter()
	return formatter.FormatList(os.Stdout, result)
}
