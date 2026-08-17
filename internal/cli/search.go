package cli

import (
	"fmt"
	"io"

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
			includePublishedAt := kind == api.SearchTypeVideo || kind == api.SearchTypeArticle
			return app.CompleteTable(payload, mode, asJSON, asYAML, func(w io.Writer) {
				app.renderTable(w, searchTitle(kind, order, args[0]), searchHeaders(kind, includePublishedAt), searchRows(kind, payload, includePublishedAt))
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

func searchHeaders(kind api.SearchType, includePublishedAt bool) []string {
	switch kind {
	case api.SearchTypeArticle:
		headers := []string{"#", "ID", "标题", "作者", "浏览", "点赞", "评论"}
		if includePublishedAt {
			headers = append(headers, "发布时间")
		}
		return headers
	case api.SearchTypeVideo:
		headers := []string{"#", "BV号", "标题", "UP主", "播放", "弹幕", "收藏"}
		if includePublishedAt {
			headers = append(headers, "发布时间")
		}
		return append(headers, "时长")
	case api.SearchTypeUser:
		headers := []string{"#", "UID", "用户名", "粉丝", "视频数", "签名"}
		if includePublishedAt {
			headers = append(headers, "发布时间")
		}
		return headers
	case api.SearchTypeBangumi, api.SearchTypeMedia:
		headers := []string{"#", "ID", "标题", "地区", "评分"}
		if includePublishedAt {
			headers = append(headers, "发布时间")
		}
		return append(headers, "进度")
	case api.SearchTypeLive:
		headers := []string{"#", "房间ID", "标题", "主播", "在线", "分区"}
		if includePublishedAt {
			headers = append(headers, "发布时间")
		}
		return headers
	default:
		headers := []string{"#", "ID", "标题", "作者"}
		if includePublishedAt {
			headers = append(headers, "发布时间")
		}
		return headers
	}
}

func searchRows(kind api.SearchType, items []map[string]any, includePublishedAt bool) [][]string {
	rows := make([][]string, 0, len(items))
	for index, item := range items {
		prefix := fmt.Sprintf("%d", index+1)
		switch kind {
		case api.SearchTypeArticle:
			row := []string{prefix, stringValue(item["id"]), stringValue(item["title"]), stringValue(item["author"]), formatCount(item["view"]), formatCount(item["like"]), formatCount(item["reply"])}
			if includePublishedAt {
				row = append(row, publishedTime(item))
			}
			rows = append(rows, row)
		case api.SearchTypeVideo:
			row := []string{prefix, stringValue(item["bvid"]), stringValue(item["title"]), stringValue(item["author"]), formatCount(item["play"]), formatCount(item["danmaku"]), formatCount(item["favorite"])}
			if includePublishedAt {
				row = append(row, publishedTime(item))
			}
			rows = append(rows, append(row, stringValue(item["duration"])))
		case api.SearchTypeUser:
			row := []string{prefix, stringValue(item["uid"]), stringValue(item["name"]), formatCount(item["fans"]), fmt.Sprintf("%d", intValue(item["videos"], 0)), stringValue(item["sign"])}
			if includePublishedAt {
				row = append(row, publishedTime(item))
			}
			rows = append(rows, row)
		case api.SearchTypeBangumi, api.SearchTypeMedia:
			row := []string{prefix, stringValue(item["id"]), stringValue(item["title"]), stringValue(item["areas"]), stringValue(item["score"])}
			if includePublishedAt {
				row = append(row, publishedTime(item))
			}
			rows = append(rows, append(row, stringValue(item["index_show"])))
		case api.SearchTypeLive:
			row := []string{prefix, stringValue(item["room_id"]), stringValue(item["title"]), stringValue(item["author"]), formatCount(item["online"]), stringValue(item["category"])}
			if includePublishedAt {
				row = append(row, publishedTime(item))
			}
			rows = append(rows, row)
		default:
			row := []string{prefix, stringValue(item["id"]), stringValue(item["title"]), stringValue(item["author"])}
			if includePublishedAt {
				row = append(row, publishedTime(item))
			}
			rows = append(rows, row)
		}
	}
	return rows
}
