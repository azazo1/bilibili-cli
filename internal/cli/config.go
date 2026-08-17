package cli

import (
	"fmt"
	"io"
	"strings"

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

	var statusJSON, statusYAML bool
	status := &cobra.Command{
		Use:   "status",
		Short: "查看配置加载状态",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := app.mode(cmd, statusJSON, statusYAML)
			if err != nil {
				return err
			}
			report := app.ConfigStore.Status()
			return app.Complete(report, mode, func(w io.Writer) {
				renderConfigStatus(w, report)
			})
		},
	}
	addStructuredFlags(status, &statusJSON, &statusYAML)
	command.AddCommand(status)

	var upgradeJSON, upgradeYAML bool
	upgrade := &cobra.Command{
		Use:   "upgrade",
		Short: "升级到当前支持的配置格式",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := app.mode(cmd, upgradeJSON, upgradeYAML)
			if err != nil {
				return err
			}
			exists := app.ConfigStore.Exists()
			settings, upgradeErr := app.ConfigStore.Upgrade()
			if upgradeErr != nil {
				return app.Fail(upgradeErr, "升级配置失败", mode)
			}
			app.Config = settings
			app.ConfigErr = nil
			app.API.SetTimeout(settings.Network.TimeoutSeconds)
			app.Out.DefaultMode = settings.Output.Format
			payload := map[string]any{
				"file":    app.ConfigStore.File,
				"created": !exists,
				"version": settings.Version,
				"config":  settings,
			}
			return app.Complete(payload, mode, func(w io.Writer) {
				if exists {
					fmt.Fprintf(w, "已升级配置: %s (版本 %d)\n", app.ConfigStore.File, settings.Version)
				} else {
					fmt.Fprintf(w, "已创建并升级配置: %s (版本 %d)\n", app.ConfigStore.File, settings.Version)
				}
			})
		},
	}
	addStructuredFlags(upgrade, &upgradeJSON, &upgradeYAML)
	command.AddCommand(upgrade)
	return command
}

func renderConfigStatus(w io.Writer, report config.StatusReport) {
	fmt.Fprintf(w, "配置文件: %s\n", report.File)
	fmt.Fprintf(w, "文件存在: %t\n", report.Exists)
	fmt.Fprintf(w, "已加载: %t\n", report.Loaded)
	fmt.Fprintf(w, "需要升级: %t\n", report.NeedsUpgrade)
	fmt.Println()
	currentSection := ""
	for _, field := range report.Fields {
		parts := strings.SplitN(field.Path, ".", 2)
		if len(parts) == 1 {
			if currentSection != "" {
				fmt.Fprintln(w)
				currentSection = ""
			}
			fmt.Fprintf(w, "%s = %v (%s)", parts[0], field.Value, field.Status)
		} else {
			if currentSection != parts[0] {
				if currentSection != "" {
					fmt.Fprintln(w)
				} else {
					fmt.Println()
				}
				fmt.Fprintf(w, "%s:\n", parts[0])
				currentSection = parts[0]
			}
			fmt.Fprintf(w, "  %s = %v (%s)", parts[1], field.Value, field.Status)
		}
		if field.Error != "" {
			fmt.Fprintf(w, " error: %s", field.Error)
		}
		fmt.Fprintln(w)
	}
	for _, message := range report.Errors {
		fmt.Fprintf(w, "错误:\n  %s\n", message)
	}
}
