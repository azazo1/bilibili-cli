package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/auth"
	"github.com/azazo1/bilibili-cli/internal/model"
)

func newVideoCommand(app *App) *cobra.Command {
	var comments, showAI, related bool
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "video BV_OR_URL",
		Short: "查看视频详情",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := app.mode(asJSON, asYAML)
			if err != nil {
				return err
			}
			bvid, err := app.extractBVID(args[0], mode)
			if err != nil {
				return err
			}
			ctx := contextOrBackground(cmd.Context())
			var credential *api.Credential
			if comments || showAI || related {
				credential, _ = app.Auth.GetCredential(ctx, auth.ModeOptional)
			}
			info, err := app.API.GetVideoInfo(ctx, bvid, nil)
			if err != nil {
				return app.apiFailure(err, "获取视频信息失败", mode)
			}
			aiSummary := ""
			commentItems := []map[string]any{}
			relatedItems := []map[string]any{}
			warnings := []map[string]string{}
			if showAI {
				result, fetchErr := app.API.GetVideoAIConclusion(ctx, bvid, credential)
				if fetchErr != nil {
					warnings = append(warnings, map[string]string{"code": "ai_summary_unavailable", "message": "获取 AI 总结失败"})
					app.Logger.Warn("获取 AI 总结失败", "error", fetchErr)
				} else {
					aiSummary = stringValue(mapValue(result["model_result"])["summary"])
				}
			}
			if comments {
				result, fetchErr := app.API.GetVideoComments(ctx, bvid, 1, credential)
				if fetchErr != nil {
					warnings = append(warnings, map[string]string{"code": "comments_unavailable", "message": "获取评论失败"})
					app.Logger.Warn("获取评论失败", "error", fetchErr)
				} else {
					commentItems = firstMapList(result["replies"])
				}
			}
			if related {
				result, fetchErr := app.API.GetRelatedVideos(ctx, bvid, credential)
				if fetchErr != nil {
					warnings = append(warnings, map[string]string{"code": "related_unavailable", "message": "获取相关推荐失败"})
					app.Logger.Warn("获取相关推荐失败", "error", fetchErr)
				} else {
					relatedItems = result
				}
			}
			payload := model.NormalizeVideoCommandPayload(info, aiSummary, commentItems, relatedItems, warnings)
			return app.Complete(payload, mode, func(w io.Writer) {
				renderVideo(w, info, bvid)
				if showAI {
					fmt.Fprintln(w, "\nAI 总结:")
					if aiSummary == "" {
						fmt.Fprintln(w, "该视频暂无 AI 总结")
					} else {
						fmt.Fprintln(w, aiSummary)
					}
				}
				if comments {
					fmt.Fprintln(w, "\n热门评论:")
					for _, item := range commentItems[:min(len(commentItems), 10)] {
						member := mapValue(item["member"])
						content := mapValue(item["content"])
						fmt.Fprintf(w, "%s (赞 %s)\n%s\n", stringValue(member["uname"]), formatCount(item["like"]), truncate(stringValue(content["message"]), 120))
					}
				}
				if related && len(relatedItems) > 0 {
					rows := make([][]string, 0, min(len(relatedItems), 10))
					for index, item := range relatedItems[:min(len(relatedItems), 10)] {
						owner := mapValue(item["owner"])
						stat := mapValue(item["stat"])
						rows = append(rows, []string{fmt.Sprintf("%d", index+1), stringValue(item["bvid"]), truncate(stringValue(item["title"]), 40), truncate(stringValue(owner["name"]), 12), formatCount(stat["view"])})
					}
					renderTable(w, "\n相关推荐", []string{"#", "BV号", "标题", "UP主", "播放"}, rows)
				}
			})
		},
	}
	command.Flags().BoolVarP(&comments, "comments", "c", false, "显示评论")
	command.Flags().BoolVar(&showAI, "ai", false, "显示 AI 总结")
	command.Flags().BoolVarP(&related, "related", "r", false, "显示相关推荐")
	addStructuredFlags(command, &asJSON, &asYAML)
	command.AddCommand(newSubtitleCommand(app))
	return command
}

func renderVideo(w io.Writer, info map[string]any, bvid string) {
	owner := mapValue(info["owner"])
	stat := mapValue(info["stat"])
	rows := [][]string{
		{"BV号", bvid},
		{"标题", stringValue(info["title"])},
		{"UP主", fmt.Sprintf("%s (UID: %s)", stringValue(owner["name"]), stringValue(owner["mid"]))},
		{"时长", formatDuration(info["duration"])},
		{"播放", formatCount(stat["view"])},
		{"弹幕", formatCount(stat["danmaku"])},
		{"点赞", formatCount(stat["like"])},
		{"投币", formatCount(stat["coin"])},
		{"收藏", formatCount(stat["favorite"])},
		{"分享", formatCount(stat["share"])},
		{"链接", "https://www.bilibili.com/video/" + bvid},
	}
	if description := stringValue(info["desc"]); description != "" {
		rows = append(rows, []string{"简介", truncate(description, 200)})
	}
	renderTable(w, "视频详情", []string{"字段", "内容"}, rows)
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
