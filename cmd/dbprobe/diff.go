package main

import (
	"fmt"
	"strings"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/spf13/cobra"
)

func newDiffCommand(deps commandDependencies) *cobra.Command {
	var format string
	var passwordStdin bool

	cmd := &cobra.Command{
		Use:   "diff <target>",
		Short: "Compare the two latest stored snapshots for a target",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("diff target is required")
			}
			if format != "text" && format != "json" {
				return fmt.Errorf("unsupported format %q: expected text or json", format)
			}
			if deps.openHistory == nil {
				return fmt.Errorf("persistent history is not configured")
			}
			target, err := resolveCommandTarget(args[0], passwordStdin, cmd.InOrStdin())
			if err != nil {
				return err
			}

			registry, err := newAdapterRegistry()
			if err != nil {
				return err
			}
			if err := validateTarget(registry, target); err != nil {
				return err
			}
			path, err := deps.resolveHistoryPath()
			if err != nil {
				return err
			}
			return withHistoryStore(cmd.Context(), path, deps.openHistory, func(store temporal.Store) error {
				return runDiff(cmd.Context(), cmd.OutOrStdout(), target, format, registry, store)
			})
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read the MySQL password from stdin instead of target URL userinfo")
	return cmd
}
