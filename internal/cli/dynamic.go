package cli

import "github.com/spf13/cobra"

func newDynamicCommand(app *App) *cobra.Command {
	command := &cobra.Command{
		Use:   "dynamic",
		Short: "管理动态",
	}
	command.AddCommand(newDynamicPostCommand(app), newDynamicDeleteCommand(app))
	return command
}
