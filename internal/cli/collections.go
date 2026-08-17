package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/model"
)

func newCollectionCommands(app *App) []*cobra.Command {
	return []*cobra.Command{
		newFavoritesCommand(app),
		newFollowingCommand(app),
		newHistoryCommand(app),
		newWatchLaterCommand(app),
		newFeedCommand(app),
		newMyDynamicsCommand(app),
		newDynamicPostCommand(app),
		newDynamicDeleteCommand(app),
	}
}

func newFavoritesCommand(app *App) *cobra.Command {
	var page int
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "favorites [FAV_ID]",
		Short: "浏览收藏夹",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := app.mode(asJSON, asYAML)
			if err != nil {
				return err
			}
			if page < 1 {
				return app.Fail(api.NewError(api.CodeInvalidInput, "", "--page 必须大于 0"), "", mode)
			}
			credential, err := app.RequireCredential(contextOrBackground(cmd.Context()), false, mode, "需要登录才能查看收藏夹. 使用 bili login 登录")
			if err != nil {
				return err
			}
			if len(args) == 0 {
				folders, fetchErr := app.API.GetFavoriteList(contextOrBackground(cmd.Context()), credential)
				if fetchErr != nil {
					return app.apiFailure(fetchErr, "获取收藏夹列表失败", mode)
				}
				payload := make([]map[string]any, 0, len(folders))
				for _, item := range folders {
					payload = append(payload, model.NormalizeFavoriteFolder(item))
				}
				return app.Complete(payload, mode, func(w io.Writer) {
					rows := make([][]string, 0, len(folders))
					for _, item := range folders {
						rows = append(rows, []string{stringValue(item["id"]), stringValue(item["title"]), stringValue(item["media_count"])})
					}
					renderTable(w, "收藏夹列表", []string{"ID", "名称", "视频数"}, rows)
				})
			}
			favID, parseErr := strconv.ParseInt(args[0], 10, 64)
			if parseErr != nil || favID <= 0 {
				return app.Fail(api.NewError(api.CodeInvalidInput, "", "FAV_ID 必须是正整数"), "", mode)
			}
			data, fetchErr := app.API.GetFavoriteVideos(contextOrBackground(cmd.Context()), favID, page, credential)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "获取收藏夹内容失败", mode)
			}
			medias := firstMapList(data["medias"])
			items := make([]map[string]any, 0, len(medias))
			for _, item := range medias {
				items = append(items, model.NormalizeFavoriteMedia(item))
			}
			payload := map[string]any{"folder_id": favID, "page": page, "has_more": boolValue(data["has_more"]), "items": items}
			return app.Complete(payload, mode, func(w io.Writer) {
				rows := make([][]string, 0, len(medias))
				for index, item := range medias {
					upper := mapValue(item["upper"])
					rows = append(rows, []string{fmt.Sprintf("%d", index+1+(page-1)*20), stringValue(item["bvid"]), truncate(stringValue(item["title"]), 40), truncate(stringValue(upper["name"]), 12), formatDuration(item["duration"])})
				}
				renderTable(w, fmt.Sprintf("收藏夹 #%d (第 %d 页)", favID, page), []string{"#", "BV号", "标题", "UP主", "时长"}, rows)
				if boolValue(data["has_more"]) {
					fmt.Fprintf(w, "还有更多内容, 使用 bili favorites %d --page %d 查看下一页\n", favID, page+1)
				}
			})
		},
	}
	command.Flags().IntVarP(&page, "page", "p", 1, "页码")
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func newFollowingCommand(app *App) *cobra.Command {
	var page int
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "following",
		Short: "查看关注列表",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := app.mode(asJSON, asYAML)
			if err != nil {
				return err
			}
			if page < 1 {
				return app.Fail(api.NewError(api.CodeInvalidInput, "", "--page 必须大于 0"), "", mode)
			}
			credential, err := app.RequireCredential(contextOrBackground(cmd.Context()), false, mode, "需要登录才能查看关注列表")
			if err != nil {
				return err
			}
			ctx := contextOrBackground(cmd.Context())
			me, fetchErr := app.API.GetSelfInfo(ctx, credential)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "获取关注列表失败", mode)
			}
			uid := int64Value(me["mid"], 0)
			data, fetchErr := app.API.GetFollowings(ctx, uid, page, 20, credential)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "获取关注列表失败", mode)
			}
			users := firstMapList(data["list"])
			items := make([]map[string]any, 0, len(users))
			for _, item := range users {
				items = append(items, model.NormalizeFollowingUser(item))
			}
			payload := map[string]any{"page": page, "total": intValue(data["total"], 0), "items": items}
			return app.Complete(payload, mode, func(w io.Writer) {
				rows := make([][]string, 0, len(users))
				for index, item := range users {
					rows = append(rows, []string{fmt.Sprintf("%d", index+1+(page-1)*20), stringValue(item["mid"]), stringValue(item["uname"]), truncate(stringValue(item["sign"]), 40)})
				}
				renderTable(w, fmt.Sprintf("关注列表 (共 %d, 第 %d 页)", intValue(data["total"], 0), page), []string{"#", "UID", "用户名", "签名"}, rows)
				fmt.Fprintf(w, "使用 bili following --page %d 查看下一页\n", page+1)
			})
		},
	}
	command.Flags().IntVarP(&page, "page", "p", 1, "页码")
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func newHistoryCommand(app *App) *cobra.Command {
	var page, count int
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "history",
		Short: "查看观看历史",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := app.mode(asJSON, asYAML)
			if err != nil {
				return err
			}
			if page < 1 || count < 1 || count > 100 {
				return app.Fail(api.NewError(api.CodeInvalidInput, "", "--page 必须大于 0, --max 范围为 1-100"), "", mode)
			}
			credential, err := app.RequireCredential(contextOrBackground(cmd.Context()), false, mode, "需要登录才能查看观看历史")
			if err != nil {
				return err
			}
			data, fetchErr := app.API.GetWatchHistory(contextOrBackground(cmd.Context()), page, count, credential)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "获取观看历史失败", mode)
			}
			items := historyList(data)
			if len(items) > count {
				items = items[:count]
			}
			normalized := make([]map[string]any, 0, len(items))
			for _, item := range items {
				normalized = append(normalized, model.NormalizeHistoryItem(item))
			}
			payload := map[string]any{"page": page, "count": len(normalized), "items": normalized}
			return app.Complete(payload, mode, func(w io.Writer) {
				rows := make([][]string, 0, len(items))
				for index, item := range items {
					historyInfo := mapValue(item["history"])
					owner := mapValue(item["owner"])
					viewAt := int64Value(firstNonNil(historyInfo["view_at"], item["view_at"]), 0)
					rows = append(rows, []string{fmt.Sprintf("%d", index+1), firstString(historyInfo["bvid"], item["bvid"], historyInfo["oid"]), truncate(firstString(item["title"], item["name"]), 36), truncate(firstString(owner["name"], item["author_name"], item["author"]), 12), timestampDisplay(viewAt)})
				}
				renderTable(w, fmt.Sprintf("观看历史 (第 %d 页)", page), []string{"#", "标识", "标题", "UP主", "观看时间"}, rows)
			})
		},
	}
	command.Flags().IntVarP(&page, "page", "p", 1, "页码")
	command.Flags().IntVarP(&count, "max", "n", 30, "显示数量")
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func newWatchLaterCommand(app *App) *cobra.Command {
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "watch-later",
		Short: "查看稍后再看列表",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := app.mode(asJSON, asYAML)
			if err != nil {
				return err
			}
			credential, err := app.RequireCredential(contextOrBackground(cmd.Context()), false, mode, "需要登录才能查看稍后再看")
			if err != nil {
				return err
			}
			data, fetchErr := app.API.GetWatchLater(contextOrBackground(cmd.Context()), credential)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "获取稍后再看失败", mode)
			}
			items := firstMapList(data["list"])
			normalized := make([]map[string]any, 0, len(items))
			for _, item := range items {
				normalized = append(normalized, model.NormalizeWatchLaterItem(item))
			}
			payload := map[string]any{"count": intValue(data["count"], len(items)), "items": normalized}
			return app.Complete(payload, mode, func(w io.Writer) {
				rows := make([][]string, 0, min(len(items), 30))
				for index, item := range items[:min(len(items), 30)] {
					owner := mapValue(item["owner"])
					rows = append(rows, []string{fmt.Sprintf("%d", index+1), stringValue(item["bvid"]), truncate(stringValue(item["title"]), 36), truncate(stringValue(owner["name"]), 12), formatDuration(item["duration"])})
				}
				renderTable(w, fmt.Sprintf("稍后再看 (共 %d 个)", intValue(data["count"], len(items))), []string{"#", "BV号", "标题", "UP主", "时长"}, rows)
			})
		},
	}
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func newFeedCommand(app *App) *cobra.Command {
	var offset string
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "feed",
		Short: "查看动态时间线",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := app.mode(asJSON, asYAML)
			if err != nil {
				return err
			}
			credential, err := app.RequireCredential(contextOrBackground(cmd.Context()), false, mode, "需要登录才能查看动态")
			if err != nil {
				return err
			}
			data, fetchErr := app.API.GetDynamicFeed(contextOrBackground(cmd.Context()), offset, credential)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "获取动态失败", mode)
			}
			items := firstMapList(data["items"])
			normalized := make([]map[string]any, 0, len(items))
			for _, item := range items {
				normalized = append(normalized, model.NormalizeDynamicItem(item))
			}
			next := firstString(data["next_offset"], data["offset"])
			payload := map[string]any{"items": normalized, "next_offset": next}
			return app.Complete(payload, mode, func(w io.Writer) {
				for _, item := range items[:min(len(items), 15)] {
					modules := mapValue(item["modules"])
					author := mapValue(modules["module_author"])
					dynamic := mapValue(modules["module_dynamic"])
					desc := mapValue(dynamic["desc"])
					fmt.Fprintf(w, "%s  %s\n", stringValue(author["name"]), stringValue(author["pub_time"]))
					if major := mapValue(dynamic["major"]); len(major) > 0 {
						archive := mapValue(major["archive"])
						article := mapValue(major["article"])
						title := firstString(archive["title"], article["title"])
						if title != "" {
							fmt.Fprintln(w, "  "+title)
						}
					}
					if text := stringValue(desc["text"]); text != "" {
						fmt.Fprintln(w, "  "+truncate(text, 100))
					}
					stat := mapValue(modules["module_stat"])
					comment := mapValue(stat["comment"])
					like := mapValue(stat["like"])
					if intValue(comment["count"], 0) > 0 || intValue(like["count"], 0) > 0 {
						fmt.Fprintf(w, "  赞 %d  评论 %d\n", intValue(like["count"], 0), intValue(comment["count"], 0))
					}
					fmt.Fprintln(w)
				}
				if next != "" {
					fmt.Fprintf(w, "下一页: bili feed --offset %s\n", next)
				}
			})
		},
	}
	command.Flags().StringVar(&offset, "offset", "", "分页游标")
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func newMyDynamicsCommand(app *App) *cobra.Command {
	var offset int64
	var count int
	var needTop, noTop bool
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "my-dynamics",
		Short: "查看我发布的动态",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := app.mode(asJSON, asYAML)
			if err != nil {
				return err
			}
			if offset < 0 || count < 1 || count > 50 {
				return app.Fail(api.NewError(api.CodeInvalidInput, "", "--offset 必须非负, --max 范围为 1-50"), "", mode)
			}
			if noTop {
				needTop = false
			}
			credential, err := app.RequireCredential(contextOrBackground(cmd.Context()), false, mode, "需要登录才能查看动态")
			if err != nil {
				return err
			}
			ctx := contextOrBackground(cmd.Context())
			me, fetchErr := app.API.GetSelfInfo(ctx, credential)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "获取我的动态失败", mode)
			}
			uid := int64Value(me["mid"], 0)
			if uid == 0 {
				return app.Fail(api.NewError(api.CodeUpstream, "", "当前用户信息缺少 mid"), "获取我的动态失败", mode)
			}
			data, fetchErr := app.API.GetUserDynamics(ctx, uid, offset, needTop, credential)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "获取我的动态失败", mode)
			}
			cards := firstMapList(data["cards"])
			if len(cards) > count {
				cards = cards[:count]
			}
			normalized := make([]map[string]any, 0, len(cards))
			for _, card := range cards {
				normalized = append(normalized, model.NormalizeDynamicItem(card))
			}
			payload := map[string]any{"offset": offset, "next_offset": firstString(data["next_offset"], data["offset"]), "items": normalized}
			return app.Complete(payload, mode, func(w io.Writer) {
				rows := make([][]string, 0, len(cards))
				for _, card := range cards {
					rows = append(rows, []string{fmt.Sprintf("%d", dynamicID(card)), timestampDisplay(dynamicTimestamp(card)), truncate(decodeDynamicText(card), 60)})
				}
				renderTable(w, fmt.Sprintf("我的动态 (offset=%d)", offset), []string{"动态ID", "发布时间", "内容"}, rows)
				next := firstString(data["next_offset"], data["offset"])
				if next != "" && next != strconv.FormatInt(offset, 10) {
					fmt.Fprintf(w, "下一页: bili my-dynamics --offset %s\n", next)
				}
			})
		},
	}
	command.Flags().Int64Var(&offset, "offset", 0, "分页偏移量")
	command.Flags().BoolVar(&needTop, "top", false, "包含置顶动态")
	command.Flags().BoolVar(&noTop, "no-top", false, "不包含置顶动态")
	command.Flags().IntVarP(&count, "max", "n", 20, "显示条数")
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func newDynamicPostCommand(app *App) *cobra.Command {
	var fromFile string
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "dynamic-post [TEXT]",
		Short: "发布一条纯文本动态",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := app.mode(asJSON, asYAML)
			if err != nil {
				return err
			}
			credential, err := app.RequireCredential(contextOrBackground(cmd.Context()), true, mode, "未登录. 使用 bili login 登录")
			if err != nil {
				return err
			}
			text := ""
			if len(args) > 0 {
				text = args[0]
			}
			if fromFile != "" {
				data, readErr := os.ReadFile(fromFile)
				if readErr != nil {
					return app.Fail(readErr, "读取动态文本失败", mode)
				}
				text = string(data)
			}
			text = strings.TrimSpace(text)
			if text == "" {
				return app.Fail(api.NewError(api.CodeInvalidInput, "", "请提供动态文本. 可用 TEXT 或 --from-file FILE"), "", mode)
			}
			data, fetchErr := app.API.PostTextDynamic(contextOrBackground(cmd.Context()), text, credential)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "发布动态失败", mode)
			}
			dynamicID := firstString(data["dynamic_id"], data["dynamic_id_str"], data["dyn_id"])
			payload := model.ActionResult("dynamic_post", map[string]any{"dynamic_id": dynamicID, "text": text})
			return app.Complete(payload, mode, func(w io.Writer) {
				if dynamicID == "" {
					fmt.Fprintln(w, "已发布动态")
				} else {
					fmt.Fprintf(w, "已发布动态: %s\n", dynamicID)
				}
			})
		},
	}
	command.Flags().StringVar(&fromFile, "from-file", "", "从文件读取动态文本")
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func newDynamicDeleteCommand(app *App) *cobra.Command {
	var yes bool
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "dynamic-delete DYNAMIC_ID",
		Short: "删除一条动态",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := app.mode(asJSON, asYAML)
			if err != nil {
				return err
			}
			id, parseErr := strconv.ParseInt(args[0], 10, 64)
			if parseErr != nil || id <= 0 {
				return app.Fail(api.NewError(api.CodeInvalidInput, "", "DYNAMIC_ID 必须是正整数"), "", mode)
			}
			credential, err := app.RequireCredential(contextOrBackground(cmd.Context()), true, mode, "未登录. 使用 bili login 登录")
			if err != nil {
				return err
			}
			if !yes && !confirm(app.Out.Stdout, id) {
				return nil
			}
			if fetchErr := app.API.DeleteDynamic(contextOrBackground(cmd.Context()), id, credential); fetchErr != nil {
				return app.apiFailure(fetchErr, "删除动态失败", mode)
			}
			payload := model.ActionResult("dynamic_delete", map[string]any{"dynamic_id": fmt.Sprintf("%d", id)})
			return app.Complete(payload, mode, func(w io.Writer) { fmt.Fprintf(w, "已删除动态: %d\n", id) })
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "跳过确认")
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func historyList(value any) []map[string]any {
	if items := model.Maps(value); len(items) > 0 {
		return items
	}
	data := model.Map(value)
	for _, key := range []string{"list", "items", "data"} {
		if items := model.Maps(data[key]); len(items) > 0 {
			return items
		}
	}
	return []map[string]any{}
}

func boolValue(value any) bool {
	if typed, ok := value.(bool); ok {
		return typed
	}
	return stringValue(value) == "1" || strings.EqualFold(stringValue(value), "true")
}

func firstString(values ...any) string {
	for _, value := range values {
		if text := stringValue(value); text != "" {
			return text
		}
	}
	return ""
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func confirm(w io.Writer, id int64) bool {
	fmt.Fprintf(w, "确认删除动态 %d 吗? [y/N] ", id)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y" || answer == "yes"
}
