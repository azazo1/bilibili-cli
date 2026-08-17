package cli

import (
	"github.com/spf13/cobra"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/output"
)

func NewRoot(app *App) *cobra.Command {
	var verbose bool
	root := &cobra.Command{
		Use:           "bili",
		Short:         "Bilibili CLI tool",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(command *cobra.Command, _ []string) error {
			app.setupLogging(verbose)
			if app.ConfigErr != nil && command.CommandPath() != "bili config init" {
				return app.Fail(api.NewError(api.CodeInvalidInput, "", "加载 config.toml 失败: "+app.ConfigErr.Error()), "", output.ModeRich)
			}
			return nil
		},
	}
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "启用调试日志")
	root.PersistentFlags().BoolVar(&app.Out.NoTruncate, "no-truncate", false, "表格不按终端宽度截断")
	root.AddCommand(newMeCommand(app))
	root.AddCommand(newVideoCommand(app), newUserCommand(app), newSearchCommand(app), newDynamicCommand(app))
	root.AddCommand(newConfigCommand(app))
	configureUsageErrors(root, app)
	return root
}

func addStructuredFlags(command *cobra.Command, asJSON, asYAML *bool) {
	command.Flags().BoolVar(asJSON, "json", false, "输出 JSON")
	command.Flags().BoolVar(asYAML, "yaml", false, "输出 YAML")
}
