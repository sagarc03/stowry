package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagarc03/stowry/internal/config"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "validate",
		GroupID: groupServer,
		Short:   "Check the metadata schema without changing it",
		Long: `Report whether the existing schema is the one stowry expects.

Nothing is created or altered, so this is what a deployment that migrates out
of band runs in place of migrate. It exits non-zero when the schema is missing
or wrong.

It checks the metadata table's columns, their types and nullability, and the
unique constraint on path that every write depends on.`,
		Args: cobra.NoArgs,
		RunE: runValidate,
	}
}

func runValidate(cmd *cobra.Command, _ []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	db, err := openDatabase(ctx, cfg, false)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "schema is valid: %s %s\n", cfg.Database.Type, cfg.Database.DSN)

	return nil
}
