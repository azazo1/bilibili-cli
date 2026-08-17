package cli

import "github.com/spf13/cobra"

func NewRoot(app *App) *cobra.Command {
	var verbose bool
	root := &cobra.Command{
		Use:           "bili",
		Short:         "Bilibili CLI tool",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			app.setupLogging(verbose)
		},
	}
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "启用调试日志")
	root.AddCommand(
		newAccountCommands(app)...,
	)
	root.AddCommand(newVideoCommand(app), newUserCommand(app), newUserVideosCommand(app), newSearchCommand(app))
	root.AddCommand(newCollectionCommands(app)...)
	root.AddCommand(newHotCommand(app), newRankCommand(app))
	root.AddCommand(newInteractionCommands(app)...)
	root.AddCommand(newAudioCommand(app))
	return root
}

func addStructuredFlags(command *cobra.Command, asJSON, asYAML *bool) {
	command.Flags().BoolVar(asJSON, "json", false, "输出 JSON")
	command.Flags().BoolVar(asYAML, "yaml", false, "输出 YAML")
}
