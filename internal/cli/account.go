package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/auth"
	"github.com/azazo1/bilibili-cli/internal/model"
	"github.com/azazo1/bilibili-cli/internal/output"
)

func newAccountCommands(app *App) []*cobra.Command {
	login := &cobra.Command{
		Use:   "login",
		Short: "扫码登录 Bilibili",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := app.Auth.QRLogin(contextOrBackground(cmd.Context()), app.Out.Stdout); err != nil {
				return app.Fail(err, "登录失败", output.ModeRich)
			}
			return nil
		},
	}
	logout := &cobra.Command{
		Use:   "logout",
		Short: "注销并清除保存的凭证",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := app.Auth.Clear(); err != nil {
				return app.Fail(err, "注销失败", output.ModeRich)
			}
			fmt.Fprintln(app.Out.Stdout, "已注销, 凭证已清除")
			return nil
		},
	}
	status := newStatusCommand(app)
	whoami := newWhoamiCommand(app)
	return []*cobra.Command{login, logout, status, whoami}
}

func newStatusCommand(app *App) *cobra.Command {
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "status",
		Short: "检查登录状态",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := app.mode(cmd, asJSON, asYAML)
			if err != nil {
				return err
			}
			credential, err := app.Auth.GetCredential(contextOrBackground(cmd.Context()), authModeRead())
			if err != nil {
				return app.Fail(err, "检查登录状态失败", mode)
			}
			if credential == nil {
				return app.Fail(api.NewError(api.CodeNotAuthenticated, "", "未登录. 使用 bili login 登录"), "", mode)
			}
			info, err := app.API.GetSelfInfo(contextOrBackground(cmd.Context()), credential)
			if err != nil {
				return app.Fail(err, "检查登录状态失败", mode)
			}
			payload := map[string]any{"authenticated": true, "user": model.NormalizeUser(info)}
			return app.Complete(payload, mode, func(w io.Writer) {
				fmt.Fprintf(w, "已登录: %s (UID: %s)\n", stringValue(info["uname"]), stringValue(info["mid"]))
			})
		},
	}
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func newWhoamiCommand(app *App) *cobra.Command {
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "whoami",
		Short: "查看当前登录用户",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := app.mode(cmd, asJSON, asYAML)
			if err != nil {
				return err
			}
			credential, err := app.RequireCredential(contextOrBackground(cmd.Context()), false, mode, "未登录. 使用 bili login 登录")
			if err != nil {
				return err
			}
			info, err := app.API.GetSelfInfo(contextOrBackground(cmd.Context()), credential)
			if err != nil {
				return app.Fail(err, "获取用户信息失败", mode)
			}
			uid := int64Value(info["mid"], 0)
			relation, err := app.API.GetUserRelationInfo(contextOrBackground(cmd.Context()), uid, credential)
			if err != nil {
				return app.Fail(err, "获取用户信息失败", mode)
			}
			payload := map[string]any{"user": model.NormalizeUser(info), "relation": model.NormalizeRelation(relation)}
			return app.Complete(payload, mode, func(w io.Writer) {
				fmt.Fprintf(w, "个人信息\n用户: %s (UID: %d)\n等级: %d  硬币: %d\n粉丝: %s  关注: %s\n", stringValue(info["uname"]), uid, intValue(info["level"], 0), intValue(info["coins"], 0), formatCount(relation["follower"]), formatCount(relation["following"]))
			})
		},
	}
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func authModeRead() auth.Mode { return auth.ModeRead }
