package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/model"
	"github.com/azazo1/bilibili-cli/internal/output"
)

func resolveUID(cmd *cobra.Command, app *App, input string, mode output.Mode) (int64, error) {
	if parsed, err := strconv.ParseInt(input, 10, 64); err == nil && parsed > 0 {
		return parsed, nil
	}
	results, err := app.API.SearchUser(contextOrBackground(cmd.Context()), input, 1)
	if err != nil {
		return 0, app.apiFailure(err, "搜索用户失败", mode)
	}
	if len(results) == 0 {
		return 0, app.Fail(api.NewError(api.CodeNotFound, "", "未找到用户: "+input), "", mode)
	}
	uid := int64Value(results[0]["mid"], 0)
	if uid == 0 {
		return 0, app.Fail(api.NewError(api.CodeUpstream, "", "搜索结果缺少 UID: "+input), "", mode)
	}
	return uid, nil
}

func newUserCommand(app *App) *cobra.Command {
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:     "user UID_OR_NAME",
		Aliases: []string{"up"},
		Short:   "查看 UP 主资料",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := app.mode(cmd, asJSON, asYAML)
			if err != nil {
				return err
			}
			uid, err := resolveUID(cmd, app, args[0], mode)
			if err != nil {
				return err
			}
			ctx := contextOrBackground(cmd.Context())
			credential := app.OptionalCredential(ctx)
			info, err := app.API.GetUserInfo(ctx, uid, credential)
			if err != nil {
				return app.apiFailure(err, "获取用户信息失败", mode)
			}
			relation, err := app.API.GetUserRelationInfo(ctx, uid, credential)
			if err != nil {
				return app.apiFailure(err, "获取用户信息失败", mode)
			}
			payload := map[string]any{"user": model.NormalizeUser(info), "relation": model.NormalizeRelation(relation)}
			return app.Complete(payload, mode, func(w io.Writer) {
				fmt.Fprintf(w, "UP 主信息\n用户: %s (UID: %d)\n等级: %d  粉丝: %s  关注: %s\n", stringValue(info["name"]), uid, intValue(info["level"], 0), formatCount(relation["follower"]), formatCount(relation["following"]))
				if sign := strings.TrimSpace(stringValue(info["sign"])); sign != "" {
					fmt.Fprintln(w, sign)
				}
			})
		},
	}
	addStructuredFlags(command, &asJSON, &asYAML)
	command.AddCommand(newUserVideosCommand(app))
	command.AddCommand(newUserInteractionCommands(app)...)
	return command
}

func newUserVideosCommand(app *App) *cobra.Command {
	var count int
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "video UID_OR_NAME",
		Short: "查看 UP 主的视频列表",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := app.mode(cmd, asJSON, asYAML)
			if err != nil {
				return err
			}
			if count < 1 {
				return app.invalidInput(cmd, "--max 必须大于 0", mode)
			}
			uid, err := resolveUID(cmd, app, args[0], mode)
			if err != nil {
				return err
			}
			ctx := contextOrBackground(cmd.Context())
			credential := app.OptionalCredential(ctx)
			videos, err := app.API.GetUserVideos(ctx, uid, count, credential)
			if err != nil {
				return app.apiFailure(err, "获取视频列表失败", mode)
			}
			payload := make([]map[string]any, 0, len(videos))
			for _, video := range videos {
				payload = append(payload, model.NormalizeVideoSummary(video))
			}
			return app.CompleteTable(payload, mode, asJSON, asYAML, func(w io.Writer) {
				rows := make([][]string, 0, len(videos))
				for index, video := range videos {
					rows = append(rows, []string{fmt.Sprintf("%d", index+1), stringValue(video["bvid"]), stringValue(video["title"]), videoLength(video["length"]), formatCount(video["play"])})
				}
				if len(rows) == 0 {
					fmt.Fprintln(w, "该用户暂无视频")
					return
				}
				app.renderTable(w, fmt.Sprintf("最新 %d 个视频", len(rows)), []string{"#", "BV号", "标题", "时长", "播放"}, rows)
			})
		},
	}
	command.Flags().IntVarP(&count, "max", "n", 10, "显示的视频数量")
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func videoLength(value any) string {
	if raw := stringValue(value); strings.Contains(raw, ":") {
		return raw
	}
	if raw := stringValue(value); raw == "" {
		return "00:00"
	}
	if intValue(value, -1) < 0 {
		return "00:00"
	}
	return formatDuration(value)
}
