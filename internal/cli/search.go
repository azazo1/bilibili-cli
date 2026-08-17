package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/model"
)

func newSearchCommand(app *App) *cobra.Command {
	var searchType, searchOrder string
	var page, count int
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "search KEYWORD",
		Short: "搜索专栏 视频 用户 番剧 直播或影视",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := app.mode(cmd, asJSON, asYAML)
			if err != nil {
				return err
			}
			if page < 1 || count < 1 {
				return app.invalidInput(cmd, "--page 和 --max 必须大于 0", mode)
			}
			kind, ok := api.ParseSearchType(searchType)
			if !ok {
				return app.invalidInput(cmd, "--type 仅支持 article, video, user, bangumi, live 或 media", mode)
			}
			order, ok := api.ParseSearchOrder(searchOrder)
			if !ok {
				return app.invalidInput(cmd, "--order 仅支持 totalrank, click, pubdate, dm 或 stow", mode)
			}
			results, fetchErr := app.API.Search(contextOrBackground(cmd.Context()), args[0], api.SearchOptions{
				Type:  kind,
				Order: order,
				Page:  page,
			})
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "搜索"+kind.Label()+"失败", mode)
			}
			if len(results) > count {
				results = results[:count]
			}
			payload := make([]map[string]any, 0, len(results))
			for _, item := range results {
				payload = append(payload, model.NormalizeSearchResult(string(kind), item))
			}
			return app.CompleteTable(payload, mode, asJSON, asYAML, func(w io.Writer) {
				app.renderTable(w, searchTitle(kind, order, args[0]), searchHeaders(kind), searchRows(kind, payload))
			})
		},
	}
	command.Flags().StringVar(&searchType, "type", "user", "搜索类型: article, video, user, bangumi, live 或 media")
	command.Flags().StringVar(&searchOrder, "order", "totalrank", "排序: totalrank, click, pubdate, dm 或 stow")
	command.Flags().IntVar(&page, "page", 1, "页码")
	command.Flags().IntVarP(&count, "max", "n", 20, "显示数量")
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func searchTitle(kind api.SearchType, order api.SearchOrder, keyword string) string {
	return fmt.Sprintf("%s搜索: %s (%s)", kind.Label(), keyword, order.Label())
}

func searchHeaders(kind api.SearchType) []string {
	switch kind {
	case api.SearchTypeArticle:
		return []string{"#", "ID", "标题", "作者", "浏览", "点赞", "评论", "发布时间"}
	case api.SearchTypeVideo:
		return []string{"#", "BV号", "标题", "UP主", "播放", "弹幕", "收藏", "发布时间", "时长"}
	case api.SearchTypeUser:
		return []string{"#", "UID", "用户名", "粉丝", "视频数", "签名", "发布时间"}
	case api.SearchTypeBangumi, api.SearchTypeMedia:
		return []string{"#", "ID", "标题", "地区", "评分", "发布时间", "进度"}
	case api.SearchTypeLive:
		return []string{"#", "房间ID", "标题", "主播", "在线", "分区", "发布时间"}
	default:
		return []string{"#", "ID", "标题", "作者", "发布时间"}
	}
}

func searchRows(kind api.SearchType, items []map[string]any) [][]string {
	rows := make([][]string, 0, len(items))
	for index, item := range items {
		prefix := fmt.Sprintf("%d", index+1)
		switch kind {
		case api.SearchTypeArticle:
			rows = append(rows, []string{prefix, stringValue(item["id"]), stringValue(item["title"]), stringValue(item["author"]), formatCount(item["view"]), formatCount(item["like"]), formatCount(item["reply"]), searchPublishedTime(item)})
		case api.SearchTypeVideo:
			rows = append(rows, []string{prefix, stringValue(item["bvid"]), stringValue(item["title"]), stringValue(item["author"]), formatCount(item["play"]), formatCount(item["danmaku"]), formatCount(item["favorite"]), searchPublishedTime(item), stringValue(item["duration"])})
		case api.SearchTypeUser:
			rows = append(rows, []string{prefix, stringValue(item["uid"]), stringValue(item["name"]), formatCount(item["fans"]), fmt.Sprintf("%d", intValue(item["videos"], 0)), stringValue(item["sign"]), searchPublishedTime(item)})
		case api.SearchTypeBangumi, api.SearchTypeMedia:
			rows = append(rows, []string{prefix, stringValue(item["id"]), stringValue(item["title"]), stringValue(item["areas"]), stringValue(item["score"]), searchPublishedTime(item), stringValue(item["index_show"])})
		case api.SearchTypeLive:
			rows = append(rows, []string{prefix, stringValue(item["room_id"]), stringValue(item["title"]), stringValue(item["author"]), formatCount(item["online"]), stringValue(item["category"]), searchPublishedTime(item)})
		default:
			rows = append(rows, []string{prefix, stringValue(item["id"]), stringValue(item["title"]), stringValue(item["author"]), searchPublishedTime(item)})
		}
	}
	return rows
}

func searchPublishedTime(item map[string]any) string {
	value := strings.TrimSpace(stringValue(item["published_at"]))
	if value == "" {
		return "-"
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.Local().Format("2006-01-02 15:04")
	}
	return value
}
