package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const userListIndexPageSize = 10
const userListVideoPageSize = 30
const userListIndexPath = "/x/polymer/web-space/seasons_series_list"

type UserListKind string

const (
	userListKindSeason UserListKind = "season"
	userListKindSeries UserListKind = "series"
)

type UserListReference struct {
	OwnerID  int64
	ListID   int64
	KindHint UserListKind
}

func (r UserListReference) HasList() bool {
	return r.ListID > 0
}

type UserListMetadata struct {
	OwnerID     int64
	ID          int64
	Title       string
	Description string
	Cover       string
	Total       int
}

type UserListPage struct {
	Number int
	Size   int
	Total  int
}

type UserListDirectory struct {
	OwnerID int64
	Page    UserListPage
	Items   []UserListMetadata
}

type UserList struct {
	Metadata UserListMetadata
	Page     UserListPage
	Archives []map[string]any
}

type userListCandidate struct {
	Kind     UserListKind
	Metadata UserListMetadata
}

type userListIndexPage struct {
	Page       UserListPage
	Candidates []userListCandidate
}

func (c *Client) ResolveUserListReference(ctx context.Context, value string) (UserListReference, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return UserListReference{}, NewError(CodeInvalidInput, "", "用户列表引用不能为空")
	}
	if strings.HasPrefix(strings.ToLower(value), "b23.tv/") {
		value = "https://" + value
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return UserListReference{}, NewError(CodeInvalidInput, "", "用户列表链接无法解析")
	}
	if parsed.Scheme == "" && parsed.Host == "" {
		if strings.Contains(value, "://") || strings.HasPrefix(value, "//") {
			return UserListReference{}, NewError(CodeInvalidInput, "", "用户列表链接无法解析")
		}
		return parseUserListIdentifier(value)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return UserListReference{}, NewError(CodeInvalidInput, "", "用户列表链接仅支持 HTTP 或 HTTPS")
	}
	if isB23Host(parsed.Hostname()) {
		finalURL, resolveErr := c.resolveB23Target(ctx, parsed.String())
		if resolveErr != nil {
			return UserListReference{}, resolveErr
		}
		return parseUserListURL(finalURL)
	}
	return parseUserListURL(parsed)
}

func (c *Client) GetUserListDirectory(ctx context.Context, ownerID int64, page int, cred *Credential) (UserListDirectory, error) {
	if ownerID <= 0 {
		return UserListDirectory{}, NewError(CodeInvalidInput, "获取用户列表", "UID 必须是正整数")
	}
	if page < 1 {
		return UserListDirectory{}, NewError(CodeInvalidInput, "获取用户列表", "页码必须大于 0")
	}

	indexPage, err := c.getUserListIndexPage(ctx, ownerID, page, cred)
	if err != nil {
		return UserListDirectory{}, err
	}
	items := make([]UserListMetadata, 0, len(indexPage.Candidates))
	for _, candidate := range indexPage.Candidates {
		items = append(items, candidate.Metadata)
	}
	return UserListDirectory{OwnerID: ownerID, Page: indexPage.Page, Items: items}, nil
}

func (c *Client) GetUserList(ctx context.Context, reference UserListReference, page int, cred *Credential) (UserList, error) {
	if reference.OwnerID <= 0 || !reference.HasList() {
		return UserList{}, NewError(CodeInvalidInput, "获取用户列表", "UID 和列表 ID 必须是正整数")
	}
	if page < 1 {
		return UserList{}, NewError(CodeInvalidInput, "获取用户列表", "页码必须大于 0")
	}

	candidate, err := c.resolveUserList(ctx, reference, cred)
	if err != nil {
		if CodeOf(err) == CodeNotFound && reference.KindHint != userListKindSeries {
			return c.getUnindexedUserSeason(ctx, reference, page, cred, err)
		}
		return UserList{}, err
	}
	data, err := c.getUserListData(ctx, reference, candidate.Kind, page, cred)
	if err != nil {
		return UserList{}, err
	}
	return userListFromData(candidate, data, page)
}

func (c *Client) getUnindexedUserSeason(ctx context.Context, reference UserListReference, page int, cred *Credential, missingErr error) (UserList, error) {
	data, err := c.getUserSeasonArchives(ctx, reference, page, cred)
	if err != nil {
		if CodeOf(err) == CodeNotFound {
			return UserList{}, missingErr
		}
		return UserList{}, err
	}
	detailMetadata := mapValue(data["meta"])
	candidate, ok := userListCandidateFromSeasonDetail(detailMetadata, reference)
	if !ok {
		return UserList{}, missingErr
	}
	if c.Logger != nil {
		c.Logger.Debug("用户列表索引未包含合集, 使用详情回退", "uid", reference.OwnerID, "sid", reference.ListID)
	}
	return userListFromData(candidate, data, page)
}

func (c *Client) getUserListData(ctx context.Context, reference UserListReference, kind UserListKind, page int, cred *Credential) (map[string]any, error) {
	switch kind {
	case userListKindSeason:
		return c.getUserSeasonArchives(ctx, reference, page, cred)
	case userListKindSeries:
		return c.getUserSeriesArchives(ctx, reference, page, cred)
	default:
		return nil, NewError(CodeUpstream, "获取用户列表", "用户列表索引返回了未知类型")
	}
}

func userListFromData(candidate userListCandidate, data map[string]any, page int) (UserList, error) {
	detailMetadata := mapValue(data["meta"])
	if err := validateUserListDetailMetadata(detailMetadata, candidate); err != nil {
		return UserList{}, err
	}
	metadata := candidate.Metadata
	mergeUserListMetadata(&metadata, detailMetadata)
	result := UserList{
		Metadata: metadata,
		Page:     userListPageFromData(mapValue(data["page"]), page, userListVideoPageSize),
		Archives: mapList(data["archives"]),
	}
	if result.Page.Total > 0 {
		result.Metadata.Total = result.Page.Total
	}
	return result, nil
}

func (c *Client) resolveUserList(ctx context.Context, reference UserListReference, cred *Credential) (userListCandidate, error) {
	matches := make([]userListCandidate, 0, 1)
	totalPages := 1
	for page := 1; page <= totalPages; page++ {
		indexPage, err := c.getUserListIndexPage(ctx, reference.OwnerID, page, cred)
		if err != nil {
			return userListCandidate{}, err
		}
		if pageCount := userListPageCount(indexPage.Page); pageCount > totalPages {
			totalPages = pageCount
		}
		for _, candidate := range indexPage.Candidates {
			if candidate.Metadata.ID != reference.ListID {
				continue
			}
			if reference.KindHint != "" && candidate.Kind != reference.KindHint {
				continue
			}
			matches = append(matches, candidate)
		}
	}

	switch len(matches) {
	case 0:
		return userListCandidate{}, NewError(CodeNotFound, "获取用户列表", "该用户没有指定列表")
	case 1:
		return matches[0], nil
	default:
		return userListCandidate{}, NewError(CodeInvalidInput, "获取用户列表", "该用户存在多个同 ID 列表, 请使用原始列表链接")
	}
}

func (c *Client) getUserListIndexPage(ctx context.Context, ownerID int64, page int, cred *Credential) (userListIndexPage, error) {
	query := url.Values{
		"mid":       []string{strconv.FormatInt(ownerID, 10)},
		"page_num":  []string{strconv.Itoa(page)},
		"page_size": []string{strconv.Itoa(userListIndexPageSize)},
	}
	var data map[string]any
	if err := c.requestWithHeaders(ctx, http.MethodGet, userListIndexPath, query, nil, cred, spaceRequestHeaders(ownerID), &data); err != nil {
		return userListIndexPage{}, withAction("获取用户列表", err)
	}
	items := mapValue(data["items_lists"])
	return userListIndexPage{
		Page:       userListPageFromData(mapValue(items["page"]), page, userListIndexPageSize),
		Candidates: userListCandidates(items, ownerID),
	}, nil
}

func userListCandidates(data map[string]any, ownerID int64) []userListCandidate {
	items := make([]userListCandidate, 0)
	for _, source := range []struct {
		Kind  UserListKind
		Key   string
		IDKey string
	}{
		{Kind: userListKindSeason, Key: "seasons_list", IDKey: "season_id"},
		{Kind: userListKindSeries, Key: "series_list", IDKey: "series_id"},
	} {
		for _, item := range mapList(data[source.Key]) {
			metadata := mapValue(item["meta"])
			if int64Value(metadata["mid"], 0) != ownerID {
				continue
			}
			id := int64Value(metadata[source.IDKey], 0)
			if id <= 0 {
				continue
			}
			items = append(items, userListCandidate{
				Kind:     source.Kind,
				Metadata: newUserListMetadata(ownerID, id, metadata),
			})
		}
	}
	return items
}

func userListCandidateFromSeasonDetail(value map[string]any, reference UserListReference) (userListCandidate, bool) {
	if int64Value(value["mid"], 0) != reference.OwnerID || int64Value(value["season_id"], 0) != reference.ListID {
		return userListCandidate{}, false
	}
	return userListCandidate{
		Kind:     userListKindSeason,
		Metadata: newUserListMetadata(reference.OwnerID, reference.ListID, value),
	}, true
}

func (c *Client) getUserSeasonArchives(ctx context.Context, reference UserListReference, page int, cred *Credential) (map[string]any, error) {
	query := url.Values{
		"mid":          []string{strconv.FormatInt(reference.OwnerID, 10)},
		"season_id":    []string{strconv.FormatInt(reference.ListID, 10)},
		"sort_reverse": []string{"false"},
		"page_size":    []string{strconv.Itoa(userListVideoPageSize)},
		"page_num":     []string{strconv.Itoa(page)},
		"web_location": []string{"333.1387"},
	}
	var data map[string]any
	if err := c.requestWithHeaders(ctx, http.MethodGet, "/x/polymer/web-space/seasons_archives_list", query, nil, cred, spaceRequestHeaders(reference.OwnerID), &data); err != nil {
		return nil, withAction("获取用户列表视频", err)
	}
	return mapValue(data), nil
}

func (c *Client) getUserSeriesArchives(ctx context.Context, reference UserListReference, page int, cred *Credential) (map[string]any, error) {
	query := url.Values{
		"mid":          []string{strconv.FormatInt(reference.OwnerID, 10)},
		"series_id":    []string{strconv.FormatInt(reference.ListID, 10)},
		"sort":         []string{"desc"},
		"ps":           []string{strconv.Itoa(userListVideoPageSize)},
		"pn":           []string{strconv.Itoa(page)},
		"web_location": []string{"333.1387"},
	}
	var data map[string]any
	if err := c.requestWithHeaders(ctx, http.MethodGet, "/x/series/archives", query, nil, cred, spaceRequestHeaders(reference.OwnerID), &data); err != nil {
		return nil, withAction("获取用户列表视频", err)
	}
	return mapValue(data), nil
}

func parseUserListIdentifier(value string) (UserListReference, error) {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) == 1 {
		ownerID, err := parsePositiveUserListID(parts[0], "UID")
		if err != nil {
			return UserListReference{}, err
		}
		return UserListReference{OwnerID: ownerID}, nil
	}
	if len(parts) != 2 {
		return UserListReference{}, NewError(CodeInvalidInput, "", "用户列表引用必须是 UID 或 UID/SID")
	}
	ownerID, err := parsePositiveUserListID(parts[0], "UID")
	if err != nil {
		return UserListReference{}, err
	}
	listID, err := parsePositiveUserListID(parts[1], "SID")
	if err != nil {
		return UserListReference{}, err
	}
	return UserListReference{OwnerID: ownerID, ListID: listID}, nil
}

func parseUserListURL(parsed *url.URL) (UserListReference, error) {
	parts, err := userListURLPath(parsed)
	if err != nil {
		return UserListReference{}, err
	}
	ownerID, err := parsePositiveUserListID(parts[0], "UID")
	if err != nil {
		return UserListReference{}, err
	}
	if len(parts) == 1 {
		return UserListReference{OwnerID: ownerID}, nil
	}

	isListsPath := parts[1] == "lists"
	isChannelDetailPath := len(parts) >= 3 && parts[1] == "channel" && (parts[2] == "collectiondetail" || parts[2] == "seriesdetail")
	if !isListsPath && !isChannelDetailPath {
		return UserListReference{}, NewError(CodeInvalidInput, "", "不支持的 Bilibili 用户列表链接")
	}

	kindHint := parseUserListKindHint(parsed.Query().Get("type"))
	if isChannelDetailPath {
		kindHint = userListKindFromChannelPath(parts[2])
	}
	queryID := strings.TrimSpace(parsed.Query().Get("sid"))
	pathID := ""
	if isListsPath && len(parts) >= 3 {
		pathID = strings.TrimSpace(parts[2])
	}
	if queryID == "" && pathID == "" {
		if isListsPath {
			return UserListReference{OwnerID: ownerID, KindHint: kindHint}, nil
		}
		return UserListReference{}, NewError(CodeInvalidInput, "", "用户列表链接缺少 sid")
	}

	listID, err := parseUserListIDPair(queryID, pathID)
	if err != nil {
		return UserListReference{}, err
	}
	return UserListReference{OwnerID: ownerID, ListID: listID, KindHint: kindHint}, nil
}

func userListURLPath(parsed *url.URL) ([]string, error) {
	if parsed == nil {
		return nil, NewError(CodeInvalidInput, "", "用户列表链接无法解析")
	}
	pathValue := strings.Trim(parsed.Path, "/")
	if pathValue == "" {
		return nil, NewError(CodeInvalidInput, "", "用户列表链接缺少 UID")
	}
	parts := strings.Split(pathValue, "/")
	switch strings.ToLower(parsed.Hostname()) {
	case "space.bilibili.com":
		return parts, nil
	case "bilibili.com", "www.bilibili.com", "m.bilibili.com":
		if len(parts) < 2 || parts[0] != "space" {
			return nil, NewError(CodeInvalidInput, "", "仅支持 Bilibili 用户列表链接")
		}
		return parts[1:], nil
	default:
		return nil, NewError(CodeInvalidInput, "", "仅支持 Bilibili 用户列表链接")
	}
}

func parseUserListIDPair(queryID, pathID string) (int64, error) {
	var listID int64
	var err error
	if queryID != "" {
		listID, err = parsePositiveUserListID(queryID, "SID")
		if err != nil {
			return 0, err
		}
	}
	if pathID != "" {
		pathListID, pathErr := parsePositiveUserListID(pathID, "列表 ID")
		if pathErr != nil {
			return 0, pathErr
		}
		if listID != 0 && listID != pathListID {
			return 0, NewError(CodeInvalidInput, "", "用户列表链接中的 sid 与路径 ID 不一致")
		}
		listID = pathListID
	}
	return listID, nil
}

func parsePositiveUserListID(value, name string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return 0, NewError(CodeInvalidInput, "", name+" 必须是正整数")
	}
	return parsed, nil
}

func parseUserListKindHint(value string) UserListKind {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "season", "collection":
		return userListKindSeason
	case "series":
		return userListKindSeries
	default:
		return ""
	}
}

func userListKindFromChannelPath(value string) UserListKind {
	switch value {
	case "collectiondetail":
		return userListKindSeason
	case "seriesdetail":
		return userListKindSeries
	default:
		return ""
	}
}

func newUserListMetadata(ownerID, listID int64, value map[string]any) UserListMetadata {
	return UserListMetadata{
		OwnerID:     ownerID,
		ID:          listID,
		Title:       firstUserListString(value["title"], value["name"]),
		Description: stringValue(value["description"]),
		Cover:       stringValue(value["cover"]),
		Total:       intValue(value["total"], 0),
	}
}

func mergeUserListMetadata(metadata *UserListMetadata, value map[string]any) {
	if metadata == nil {
		return
	}
	if title := firstUserListString(value["title"], value["name"]); title != "" {
		metadata.Title = title
	}
	if description := stringValue(value["description"]); description != "" {
		metadata.Description = description
	}
	if cover := stringValue(value["cover"]); cover != "" {
		metadata.Cover = cover
	}
	if total := intValue(value["total"], 0); total > 0 {
		metadata.Total = total
	}
}

func validateUserListDetailMetadata(value map[string]any, candidate userListCandidate) error {
	if len(value) == 0 {
		return nil
	}
	if ownerID := int64Value(value["mid"], 0); ownerID > 0 && ownerID != candidate.Metadata.OwnerID {
		return NewError(CodeUpstream, "获取用户列表", "列表详情返回了其他用户的数据")
	}
	idKey := "season_id"
	if candidate.Kind == userListKindSeries {
		idKey = "series_id"
	}
	if listID := int64Value(value[idKey], 0); listID > 0 && listID != candidate.Metadata.ID {
		return NewError(CodeUpstream, "获取用户列表", "列表详情返回了其他列表的数据")
	}
	return nil
}

func userListPageFromData(value map[string]any, fallbackPage, fallbackSize int) UserListPage {
	number := firstPositiveUserListInt(value["page_num"], value["num"])
	if number == 0 {
		number = fallbackPage
	}
	size := firstPositiveUserListInt(value["page_size"], value["size"])
	if size == 0 {
		size = fallbackSize
	}
	return UserListPage{Number: number, Size: size, Total: intValue(value["total"], 0)}
}

func userListPageCount(page UserListPage) int {
	if page.Total < 1 || page.Size < 1 {
		return 1
	}
	return (page.Total + page.Size - 1) / page.Size
}

func firstUserListString(values ...any) string {
	for _, value := range values {
		if text := strings.TrimSpace(stringValue(value)); text != "" {
			return text
		}
	}
	return ""
}

func firstPositiveUserListInt(values ...any) int {
	for _, value := range values {
		if number := intValue(value, 0); number > 0 {
			return number
		}
	}
	return 0
}
