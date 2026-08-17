package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/azazo1/bilibili-cli/internal/config"
	"github.com/azazo1/bilibili-cli/internal/output"
)

func newConfigCommand(app *App) *cobra.Command {
	command := &cobra.Command{
		Use:   "config",
		Short: "管理本地配置",
	}
	var force bool
	init := &cobra.Command{
		Use:   "init",
		Short: "创建默认 config.toml",
		RunE: func(_ *cobra.Command, _ []string) error {
			exists := app.ConfigStore.Exists()
			if force {
				if err := app.ConfigStore.Save(config.Default()); err != nil {
					return app.Fail(err, "创建默认配置失败", output.ModeRich)
				}
				fmt.Fprintf(app.Out.Stdout, "已重置默认配置: %s\n", app.ConfigStore.File)
				return nil
			}
			if _, err := app.ConfigStore.Ensure(); err != nil {
				return app.Fail(err, "创建默认配置失败", output.ModeRich)
			}
			if exists {
				fmt.Fprintf(app.Out.Stdout, "配置文件已存在: %s\n", app.ConfigStore.File)
			} else {
				fmt.Fprintf(app.Out.Stdout, "已创建默认配置: %s\n", app.ConfigStore.File)
			}
			return nil
		},
	}
	init.Flags().BoolVar(&force, "force", false, "覆盖已有配置")
	command.AddCommand(init)
	return command
}

