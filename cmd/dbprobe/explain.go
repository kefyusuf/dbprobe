package main

import (
	"fmt"
	"strings"

	appexplain "github.com/kefyusuf/dbprobe/internal/app/explain"
	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	jsonsurface "github.com/kefyusuf/dbprobe/internal/surfaces/json"
	"github.com/kefyusuf/dbprobe/internal/surfaces/terminal"
	"github.com/spf13/cobra"
)

type registryFactory func() (*adapterregistry.Registry, error)

func newExplainCommand() *cobra.Command {
	return newExplainCommandWithRegistry(newAdapterRegistry)
}

func newExplainCommandWithRegistry(factory registryFactory) *cobra.Command {
	var statement string
	var format string

	cmd := &cobra.Command{
		Use:   "explain <target>",
		Short: "Inspect an estimated query plan without executing the query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" {
				return fmt.Errorf("unsupported format %q: expected text or json", format)
			}
			if strings.TrimSpace(statement) == "" {
				return fmt.Errorf("explain statement is required")
			}
			if factory == nil {
				return fmt.Errorf("adapter registry factory is required")
			}

			registry, err := factory()
			if err != nil {
				return err
			}
			report, err := appexplain.New(registry).Run(cmd.Context(), args[0], statement)
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return jsonsurface.RenderExplain(cmd.OutOrStdout(), report)
			default:
				return terminal.RenderExplain(cmd.OutOrStdout(), report)
			}
		},
	}
	cmd.Flags().StringVar(&statement, "statement", "", "single SELECT statement to explain")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return cmd
}
