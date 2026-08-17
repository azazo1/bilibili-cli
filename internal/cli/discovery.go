package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/azazo1/bilibili-cli/internal/model"
)

func newHotCommand(app *App) *cobra.Command {
	var page, count int
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "hot",
		Short: "查看热门视频",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := app.mode(cmd, asJSON, asYAML)
			if err != nil {
				return err
			}
			if page < 1 || count < 1 {
				return app.invalidInput(cmd, "--page 和 --max 必须大于 0", mode)
			}
			data, fetchErr := app.API.GetHotVideos(contextOrBackground(cmd.Context()), page, count)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "获取热门视频失败", mode)
			}
			items := firstMapList(data["list"])
			if len(items) > count {
				items = items[:count]
			}
			payloadItems := make([]map[string]any, 0, len(items))
			for _, item := range items {
				payloadItems = append(payloadItems, model.NormalizeVideoSummary(item))
			}
			payload := map[string]any{"items": payloadItems, "page": page, "count": count}
			return app.Complete(payload, mode, func(w io.Writer) {
				rows := videoRows(items, false)
				renderTable(w, "热门视频", []string{"#", "BV号", "标题", "UP主", "播放", "点赞"}, rows)
			})
		},
	}
	command.Flags().IntVarP(&page, "page", "p", 1, "页码")
	command.Flags().IntVarP(&count, "max", "n", 20, "显示数量")
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func newRankCommand(app *App) *cobra.Command {
	var day, count int
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "rank",
		Short: "查看全站排行榜",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := app.mode(cmd, asJSON, asYAML)
			if err != nil {
				return err
			}
			if (day != 3 && day != 7) || count < 1 {
				return app.invalidInput(cmd, "--day 必须是 3 或 7, --max 必须大于 0", mode)
			}
			data, fetchErr := app.API.GetRankVideos(contextOrBackground(cmd.Context()), day)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "获取排行榜失败", mode)
			}
			items := firstMapList(data["list"])
			if len(items) > count {
				items = items[:count]
			}
			payloadItems := make([]map[string]any, 0, len(items))
			for _, item := range items {
				payloadItems = append(payloadItems, model.NormalizeVideoSummary(item))
			}
			payload := map[string]any{"items": payloadItems, "day": day, "count": count}
			return app.Complete(payload, mode, func(w io.Writer) {
				rows := make([][]string, 0, len(items))
				for index, item := range items {
					owner := mapValue(item["owner"])
					stat := mapValue(item["stat"])
					rows = append(rows, []string{fmt.Sprintf("%d", index+1), stringValue(item["bvid"]), truncate(stringValue(item["title"]), 36), truncate(stringValue(owner["name"]), 12), formatCount(stat["view"]), stringValue(item["score"])})
				}
				renderTable(w, "全站排行榜", []string{"#", "BV号", "标题", "UP主", "播放", "综合分"}, rows)
			})
		},
	}
	command.Flags().IntVar(&day, "day", 3, "排行周期: 3 或 7 天")
	command.Flags().IntVarP(&count, "max", "n", 20, "显示数量")
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func videoRows(items []map[string]any, rank bool) [][]string {
	rows := make([][]string, 0, len(items))
	for index, item := range items {
		owner := mapValue(item["owner"])
		stat := mapValue(item["stat"])
		last := formatCount(stat["like"])
		if rank {
			last = stringValue(item["score"])
		}
		rows = append(rows, []string{fmt.Sprintf("%d", index+1), stringValue(item["bvid"]), truncate(stringValue(item["title"]), 36), truncate(stringValue(owner["name"]), 12), formatCount(stat["view"]), last})
	}
	return rows
}
