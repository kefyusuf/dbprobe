package main

import "github.com/spf13/cobra"

func newRootCommand() *cobra.Command {
	return newRootCommandWithDependencies(defaultCommandDependencies())
}

func newRootCommandWithDependencies(deps commandDependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "dbprobe",
		Version:       "dev (commit unknown, built unknown)",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetVersionTemplate("dbprobe version {{.Version}}\n")
	cmd.AddCommand(newInspectCommandWithDependencies(deps), newExplainCommand())
	if deps.openHistory != nil {
		cmd.AddCommand(newDiffCommand(deps))
	}
	return cmd
}
