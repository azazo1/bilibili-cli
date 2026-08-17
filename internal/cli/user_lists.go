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

func newUserListsCommand(app *App) *cobra.Command {
	var page int
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:   "lists UID_OR_NAME_OR_URL",
		Short: "查看 UP 主的合集和系列列表",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := app.mode(cmd, asJSON, asYAML)
			if err != nil {
				return err
			}
			if page < 1 {
				return app.invalidInput(cmd, "--page 必须大于 0", mode)
			}
			ctx := contextOrBackground(cmd.Context())
			reference, err := resolveUserListsReference(cmd, app, args[0], mode)
			if err != nil {
				return err
			}
			credential := app.OptionalCredential(ctx)
			if !reference.HasList() {
				directory, fetchErr := app.API.GetUserListDirectory(ctx, reference.OwnerID, page, credential)
				if fetchErr != nil {
					return app.apiFailure(fetchErr, "获取用户列表失败", mode)
				}
				payload := userListDirectoryPayload(directory)
				return app.CompleteTable(payload, mode, asJSON, asYAML, func(writer io.Writer) {
					renderUserListDirectory(app, writer, directory)
				})
			}

			list, fetchErr := app.API.GetUserList(ctx, reference, page, credential)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "获取用户列表失败", mode)
			}
			items := normalizeUserListVideos(list.Archives, list.Metadata.OwnerID)
			payload := userListPayload(list, items)
			return app.CompleteTable(payload, mode, asJSON, asYAML, func(writer io.Writer) {
				renderUserList(app, writer, list, items)
			})
		},
	}
	command.Flags().IntVarP(&page, "page", "p", 1, "页码")
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func resolveUserListsReference(cmd *cobra.Command, app *App, input string, mode output.Mode) (api.UserListReference, error) {
	input = strings.TrimSpace(input)
	ctx := contextOrBackground(cmd.Context())
	if isUserListsURLInput(input) {
		reference, err := app.API.ResolveUserListReference(ctx, input)
		if err != nil {
			return api.UserListReference{}, app.invalidInput(cmd, err.Error(), mode)
		}
		return reference, nil
	}

	ownerInput, sid, err := splitUserListsInput(input)
	if err != nil {
		return api.UserListReference{}, app.invalidInput(cmd, err.Error(), mode)
	}
	ownerID, err := resolveUID(cmd, app, ownerInput, mode)
	if err != nil {
		return api.UserListReference{}, err
	}
	canonical := strconv.FormatInt(ownerID, 10)
	if sid != "" {
		canonical += "/" + sid
	}
	reference, err := app.API.ResolveUserListReference(ctx, canonical)
	if err != nil {
		return api.UserListReference{}, app.invalidInput(cmd, err.Error(), mode)
	}
	return reference, nil
}

func isUserListsURLInput(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "b23.tv/")
}

func splitUserListsInput(value string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) == 1 && strings.TrimSpace(parts[0]) != "" {
		return parts[0], "", nil
	}
	if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
		return parts[0], parts[1], nil
	}
	return "", "", api.NewError(api.CodeInvalidInput, "", "用户列表引用必须是 UID_OR_NAME 或 UID_OR_NAME/SID")
}

func userListDirectoryPayload(directory api.UserListDirectory) map[string]any {
	items := make([]map[string]any, 0, len(directory.Items))
	for _, item := range directory.Items {
		items = append(items, userListMetadataPayload(item))
	}
	return map[string]any{
		"owner": map[string]any{"id": strconv.FormatInt(directory.OwnerID, 10)},
		"page":  userListPagePayload(directory.Page),
		"items": items,
	}
}

func userListPayload(list api.UserList, items []map[string]any) map[string]any {
	metadata := userListMetadataPayload(list.Metadata)
	return map[string]any{
		"owner":       metadata["owner"],
		"id":          metadata["id"],
		"title":       metadata["title"],
		"description": metadata["description"],
		"cover":       metadata["cover"],
		"total":       metadata["total"],
		"page":        userListPagePayload(list.Page),
		"items":       items,
	}
}

func userListMetadataPayload(metadata api.UserListMetadata) map[string]any {
	return map[string]any{
		"owner":       map[string]any{"id": strconv.FormatInt(metadata.OwnerID, 10)},
		"id":          strconv.FormatInt(metadata.ID, 10),
		"title":       metadata.Title,
		"description": metadata.Description,
		"cover":       metadata.Cover,
		"total":       metadata.Total,
	}
}

func userListPagePayload(page api.UserListPage) map[string]any {
	return map[string]any{
		"number": page.Number,
		"size":   page.Size,
		"total":  page.Total,
	}
}

func normalizeUserListVideos(archives []map[string]any, ownerID int64) []map[string]any {
	items := make([]map[string]any, 0, len(archives))
	ownerIDText := strconv.FormatInt(ownerID, 10)
	for _, archive := range archives {
		item := model.NormalizeVideoSummary(archive)
		owner := model.Map(item["owner"])
		if strings.TrimSpace(model.String(owner["id"])) == "" {
			owner["id"] = ownerIDText
		}
		item["owner"] = owner
		items = append(items, item)
	}
	return items
}

func renderUserListDirectory(app *App, writer io.Writer, directory api.UserListDirectory) {
	rows := make([][]string, 0, len(directory.Items))
	for index, item := range directory.Items {
		rows = append(rows, []string{
			fmt.Sprintf("%d", index+1+(directory.Page.Number-1)*directory.Page.Size),
			strconv.FormatInt(item.ID, 10),
			item.Title,
			fmt.Sprintf("%d", item.Total),
		})
	}
	title := fmt.Sprintf("用户 UID: %d 的列表 (第 %d 页)", directory.OwnerID, directory.Page.Number)
	if len(rows) == 0 {
		fmt.Fprintln(writer, title)
		fmt.Fprintln(writer, "该用户暂无列表")
		return
	}
	app.renderTable(writer, title, []string{"#", "ID", "标题", "视频数"}, rows)
}

func renderUserList(app *App, writer io.Writer, list api.UserList, items []map[string]any) {
	metadataRows := [][]string{
		{"用户 UID", strconv.FormatInt(list.Metadata.OwnerID, 10)},
		{"列表 ID", strconv.FormatInt(list.Metadata.ID, 10)},
		{"标题", list.Metadata.Title},
		{"视频数", fmt.Sprintf("%d", list.Metadata.Total)},
	}
	if description := strings.TrimSpace(list.Metadata.Description); description != "" {
		metadataRows = append(metadataRows, []string{"简介", description})
	}
	app.renderTable(writer, "列表详情", []string{"字段", "内容"}, metadataRows)
	if len(items) == 0 {
		fmt.Fprintln(writer, "\n该列表暂无视频")
		return
	}

	includePublishedAt := hasPublishedAt(items)
	rows := make([][]string, 0, len(items))
	for index, item := range items {
		stats := mapValue(item["stats"])
		row := []string{
			fmt.Sprintf("%d", index+1+(list.Page.Number-1)*list.Page.Size),
			stringValue(item["bvid"]),
			stringValue(item["title"]),
		}
		if includePublishedAt {
			row = append(row, publishedTime(item))
		}
		row = append(row, stringValue(item["duration"]), formatCount(stats["view"]))
		rows = append(rows, row)
	}
	headers := []string{"#", "BV号", "标题"}
	if includePublishedAt {
		headers = append(headers, "发布时间")
	}
	headers = append(headers, "时长", "播放")
	app.renderTable(writer, "\n视频列表", headers, rows)
}
