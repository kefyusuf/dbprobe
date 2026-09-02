package main

import "github.com/spf13/cobra"

func newRootCommand() *cobra.Command {
	return newRootCommandWithDependencies(defaultCommandDependencies())
}

func newRootCommandWithDependencies(deps commandDependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "dbprobe",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newInspectCommandWithDependencies(deps), newExplainCommand())
	if deps.openHistory != nil {
		cmd.AddCommand(newDiffCommand(deps))
	}
	return cmd
}
