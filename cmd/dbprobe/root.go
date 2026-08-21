package main

import "github.com/spf13/cobra"

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "dbprobe",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newInspectCommand(), newExplainCommand())
	return cmd
}
