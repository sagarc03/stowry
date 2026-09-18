package cli

import (
	"errors"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/sagarc03/stowry/internal/client"
	"github.com/sagarc03/stowry/internal/config"
	"github.com/sagarc03/stowry/types"
)

// errNoCredentials is what an unconfigured client looks like before any request
// is made, rather than a 401 from the server.
var errNoCredentials = errors.New(
	"no credentials: set auth.access_key and auth.secret_key, or pass --access-key and --secret-key")

func newClient(cmd *cobra.Command) (*client.Client, *config.Config, error) {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return nil, nil, err
	}

	clientCfg := cfg.ClientConfig()
	if clientCfg.AccessKey == "" || clientCfg.SecretKey == "" {
		return nil, nil, errNoCredentials
	}

	c, err := client.New(&clientCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create client: %w", err)
	}

	return c, cfg, nil
}

func newUploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "upload <local-path> [remote-path]",
		GroupID: groupClient,
		Short:   "Upload a file or directory to the server",
		Long: `Upload a file to the server. A directory is uploaded whole, with its
tree preserved under the remote path.

Given no remote path, the local path is used, stripped of any leading "./",
"../" or "/".`,
		Args: cobra.RangeArgs(1, 2),
		RunE: runUpload,
	}

	cmd.Flags().String("content-type", "", "content type to store, for every file uploaded (default: detect per file)")

	return cmd
}

func runUpload(cmd *cobra.Command, args []string) error {
	c, cfg, err := newClient(cmd)
	if err != nil {
		return err
	}

	contentType, err := cmd.Flags().GetString("content-type")
	if err != nil {
		return err
	}

	remote := client.NormalizeLocalToRemotePath(args[0])
	if len(args) == 2 {
		remote = args[1]
	}

	results, err := c.Upload(cmd.Context(), types.UploadOptions{
		LocalPath:   args[0],
		RemotePath:  remote,
		ContentType: contentType,
	})

	out := cmd.OutOrStdout()
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(out, "failed  %s: %s\n", r.LocalPath, r.Err)
			continue
		}

		fmt.Fprintf(out, "%s -> %s/%s (%d bytes, %s)\n",
			r.LocalPath, cfg.Endpoint, r.RemotePath, r.Size, r.ContentType)
	}

	return err
}

func newDownloadCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "download <remote-path> [local-path]",
		GroupID: groupClient,
		Short:   "Download a file from the server",
		Long: `Download a file from the server.

Given no local path, the file is written to the base name of the remote path.
A local path of "-" writes it to stdout.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: runDownload,
	}
}

func runDownload(cmd *cobra.Command, args []string) error {
	c, _, err := newClient(cmd)
	if err != nil {
		return err
	}

	local := ""
	if len(args) == 2 {
		local = args[1]
	}

	result, body, err := c.Download(cmd.Context(), types.DownloadOptions{
		RemotePath: args[0],
		LocalPath:  local,
	})
	if err != nil {
		return err
	}

	// A "-" local path hands back the body rather than writing it.
	if body != nil {
		defer func() { _ = body.Close() }()

		if _, err := io.Copy(cmd.OutOrStdout(), body); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}

		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s -> %s (%d bytes, %s)\n",
		result.RemotePath, result.LocalPath, result.Size, result.ContentType)

	return nil
}

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <remote-path> [remote-path...]",
		GroupID: groupClient,
		Short:   "Delete files from the server",
		Long: `Delete one or more files from the server.

Every path is attempted: one failure does not abandon the rest.`,
		Args: cobra.MinimumNArgs(1),
		RunE: runDelete,
	}
}

func runDelete(cmd *cobra.Command, args []string) error {
	c, _, err := newClient(cmd)
	if err != nil {
		return err
	}

	results, err := c.Delete(cmd.Context(), types.DeleteOptions{Paths: args})

	out := cmd.OutOrStdout()
	for _, r := range results {
		if r.Deleted {
			fmt.Fprintf(out, "deleted %s\n", r.Path)
			continue
		}

		fmt.Fprintf(out, "failed  %s: %s\n", r.Path, r.Err)
	}

	return err
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list [prefix]",
		GroupID: groupClient,
		Short:   "List objects on the server",
		Long: `List objects on the server, most recent last.

The server pages the results: it returns a cursor to pass back for the next
page, or --all to follow them to the end. Store mode only.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runList,
	}

	cmd.Flags().Int("limit", 0, "objects per page (default: the server's own)")
	cmd.Flags().String("cursor", "", "continue from the cursor a previous page returned")
	cmd.Flags().Bool("all", false, "follow the cursor until every object is listed")

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	c, _, err := newClient(cmd)
	if err != nil {
		return err
	}

	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		return err
	}

	cursor, err := cmd.Flags().GetString("cursor")
	if err != nil {
		return err
	}

	all, err := cmd.Flags().GetBool("all")
	if err != nil {
		return err
	}

	var prefix string
	if len(args) == 1 {
		prefix = args[0]
	}

	result, err := c.List(cmd.Context(), types.ListOptions{
		Prefix: prefix,
		Limit:  limit,
		Cursor: cursor,
		All:    all,
	})
	if err != nil {
		return err
	}

	printObjects(cmd.OutOrStdout(), result)

	return nil
}

func printObjects(out io.Writer, result *types.ListResult) {
	if len(result.Items) == 0 {
		fmt.Fprintln(out, "no objects")
		return
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PATH\tSIZE\tCONTENT TYPE\tUPDATED")

	for _, item := range result.Items {
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
			item.Path, item.FileSizeBytes, item.ContentType, item.UpdatedAt.Format(time.RFC3339))
	}

	_ = w.Flush()

	fmt.Fprintf(out, "\n%d objects, %d bytes\n", len(result.Items), result.TotalSize())

	if result.NextCursor != "" {
		fmt.Fprintf(out, "more: --cursor %s\n", result.NextCursor)
	}
}
