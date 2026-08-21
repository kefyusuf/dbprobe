package main

import (
	"fmt"
	"time"

	"github.com/kefyusuf/dbprobe/internal/app/inspect"
	"github.com/kefyusuf/dbprobe/internal/core/collection"
	jsonsurface "github.com/kefyusuf/dbprobe/internal/surfaces/json"
	"github.com/kefyusuf/dbprobe/internal/surfaces/terminal"
	"github.com/spf13/cobra"
)

func newInspectCommand() *cobra.Command {
	var format string
	var sampleWindow time.Duration

	cmd := &cobra.Command{
		Use:   "inspect <target>",
		Short: "Inspect a database target",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" {
				return fmt.Errorf("unsupported format %q: expected text or json", format)
			}
			if sampleWindow <= 0 {
				return fmt.Errorf("sample window must be positive")
			}

			registry, err := newAdapterRegistry()
			if err != nil {
				return err
			}
			planner := collection.New(collection.RealWaiter{}, time.Now)
			service := inspect.New(registry, planner)
			report, err := service.Run(cmd.Context(), args[0], sampleWindow)
			if err != nil {
				return err
			}

			switch format {
			case "json":
				return jsonsurface.Render(cmd.OutOrStdout(), report)
			default:
				return terminal.Render(cmd.OutOrStdout(), report)
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	cmd.Flags().DurationVar(&sampleWindow, "sample-window", time.Second, "counter sampling window")
	return cmd
}
