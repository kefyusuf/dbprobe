package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/collection"
	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/spf13/cobra"
)

var historyUnavailableWarning = collection.Warning{
	CollectorID: "history",
	Reason:      "local history unavailable; inspection was not persisted",
}

func newInspectCommand() *cobra.Command {
	return newInspectCommandWithDependencies(commandDependencies{})
}

func newInspectCommandWithDependencies(deps commandDependencies) *cobra.Command {
	var format string
	var sampleWindow time.Duration

	cmd := &cobra.Command{
		Use:   "inspect <target>",
		Short: "Inspect a database target",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("inspect target is required")
			}
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
			runWithoutHistory := func() error {
				return runInspect(cmd.Context(), cmd.OutOrStdout(), args[0], format, sampleWindow, registry, nil, historyUnavailableWarning)
			}
			if deps.openHistory == nil {
				return runInspect(cmd.Context(), cmd.OutOrStdout(), args[0], format, sampleWindow, registry, nil)
			}
			path, err := deps.resolveHistoryPath()
			if err != nil {
				return runWithoutHistory()
			}
			store, err := openHistoryStore(cmd.Context(), path, deps.openHistory)
			if err != nil {
				return runWithoutHistory()
			}
			return withOpenedHistoryStore(store, func(store temporal.Store) error {
				return runInspect(cmd.Context(), cmd.OutOrStdout(), args[0], format, sampleWindow, registry, store)
			})
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	cmd.Flags().DurationVar(&sampleWindow, "sample-window", time.Second, "counter sampling window")
	return cmd
}
