package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type SearchType string

const (
	SearchTypeAll      SearchType = "all"
	SearchTypeArticle  SearchType = "article"
	SearchTypeVideo    SearchType = "video"
	SearchTypeUser     SearchType = "user"
	SearchTypeBangumi  SearchType = "bangumi"
	SearchTypeLive     SearchType = "live"
	SearchTypeMedia    SearchType = "media"
)

type SearchOrder string

const (
	SearchOrderComprehensive SearchOrder = "totalrank"
	SearchOrderMostPlayed    SearchOrder = "click"
	SearchOrderLatest        SearchOrder = "pubdate"
	SearchOrderMostDanmaku   SearchOrder = "dm"
	SearchOrderMostFavorite  SearchOrder = "stow"
)

type SearchOptions struct {
	Type  SearchType
	Order SearchOrder
	Page  int
}

func ParseSearchType(value string) (SearchType, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "all", "综合", "综合搜索":
		return SearchTypeAll, true
	case "article", "专栏":
		return SearchTypeArticle, true
	case "video", "视频":
		return SearchTypeVideo, true
	case "user", "bili_user", "用户":
		return SearchTypeUser, true
	case "bangumi", "media_bangumi", "番剧":
		return SearchTypeBangumi, true
	case "live", "live_room", "直播":
		return SearchTypeLive, true
	case "media", "movie", "film", "media_ft", "影视":
		return SearchTypeMedia, true
	default:
		return "", false
	}
}

func ParseSearchOrder(value string) (SearchOrder, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "totalrank", "综合排序":
		return SearchOrderComprehensive, true
	case "click", "最多播放":
		return SearchOrderMostPlayed, true
	case "pubdate", "最新发布":
		return SearchOrderLatest, true
	case "dm", "最多弹幕":
		return SearchOrderMostDanmaku, true
	case "stow", "最多收藏":
		return SearchOrderMostFavorite, true
	default:
		return "", false
	}
}

func (t SearchType) Label() string {
	switch t {
	case SearchTypeAll:
		return "综合"
	case SearchTypeArticle:
		return "专栏"
	case SearchTypeVideo:
		return "视频"
	case SearchTypeUser:
		return "用户"
	case SearchTypeBangumi:
		return "番剧"
	case SearchTypeLive:
		return "直播"
	case SearchTypeMedia:
		return "影视"
	default:
		return string(t)
	}
}

func (o SearchOrder) Label() string {
	switch o {
	case SearchOrderComprehensive:
		return "综合排序"
	case SearchOrderMostPlayed:
		return "最多播放"
	case SearchOrderLatest:
		return "最新发布"
	case SearchOrderMostDanmaku:
		return "最多弹幕"
	case SearchOrderMostFavorite:
		return "最多收藏"
	default:
		return string(o)
	}
}

func (c *Client) Search(ctx context.Context, keyword string, options SearchOptions) ([]map[string]any, error) {
	if strings.TrimSpace(keyword) == "" {
		return nil, NewError(CodeInvalidInput, "搜索", "搜索关键词不能为空")
	}
	if options.Type == "" {
		options.Type = SearchTypeAll
	}
	if options.Order == "" {
		options.Order = SearchOrderComprehensive
	}
	order, ok := ParseSearchOrder(string(options.Order))
	if !ok {
		return nil, NewError(CodeInvalidInput, "搜索", "不支持的排序方式: "+string(options.Order))
	}
	options.Order = order
	if options.Page < 1 {
		options.Page = 1
	}
	if options.Type == SearchTypeAll {
		return c.searchAll(ctx, keyword, options.Order, options.Page, "搜索综合")
	}
	searchType, ok := options.Type.apiValue()
	if !ok {
		return nil, NewError(CodeInvalidInput, "搜索", "不支持的搜索类型: "+string(options.Type))
	}
	return c.search(ctx, keyword, searchType, options.Order, options.Page, "搜索"+options.Type.Label())
}

func (c *Client) SearchUser(ctx context.Context, keyword string, page int) ([]map[string]any, error) {
	return c.Search(ctx, keyword, SearchOptions{Type: SearchTypeUser, Order: SearchOrderComprehensive, Page: page})
}

func (c *Client) SearchVideo(ctx context.Context, keyword string, page int) ([]map[string]any, error) {
	return c.Search(ctx, keyword, SearchOptions{Type: SearchTypeVideo, Order: SearchOrderComprehensive, Page: page})
}

func (c *Client) search(ctx context.Context, keyword, searchType string, order SearchOrder, page int, action string) ([]map[string]any, error) {
	query := url.Values{
		"keyword":       []string{keyword},
		"search_type":   []string{searchType},
		"page":          []string{fmt.Sprintf("%d", page)},
		"page_size":     []string{"20"},
		"platform":      []string{"pc"},
		"web_location":  []string{"1430654"},
		"order":         []string{string(order)},
		"order_avoided": []string{"true"},
	}
	requestCredential := c.credentialWithDevice(ctx, nil)
	headers := searchRequestHeaders(keyword, searchType)
	var data map[string]any
	var requestErr error
	if signed, signErr := c.signWBI(ctx, query, requestCredential); signErr == nil {
		requestErr = c.requestWithHeaders(ctx, http.MethodGet, "/x/web-interface/wbi/search/type", signed, nil, requestCredential, headers, &data)
	} else {
		requestErr = signErr
	}
	if requestErr != nil {
		if CodeOf(requestErr) == CodeRateLimited {
			return nil, withAction(action, requestErr)
		}
		requestErr = c.requestWithHeaders(ctx, http.MethodGet, "/x/web-interface/search/type", query, nil, requestCredential, headers, &data)
	}
	if requestErr != nil {
		return nil, withAction(action, requestErr)
	}
	return mapList(mapValue(data)["result"]), nil
}

func (c *Client) searchAll(ctx context.Context, keyword string, order SearchOrder, page int, action string) ([]map[string]any, error) {
	query := url.Values{
		"keyword": []string{keyword},
		"page":    []string{fmt.Sprintf("%d", page)},
		"order":   []string{string(order)},
	}
	requestCredential := c.credentialWithDevice(ctx, nil)
	headers := searchRequestHeaders(keyword, "all")
	var data map[string]any
	var requestErr error
	if signed, signErr := c.signWBI(ctx, query, requestCredential); signErr == nil {
		requestErr = c.requestWithHeaders(ctx, http.MethodGet, "/x/web-interface/wbi/search/all/v2", signed, nil, requestCredential, headers, &data)
	} else {
		requestErr = signErr
	}
	if requestErr != nil {
		if CodeOf(requestErr) == CodeRateLimited {
			return nil, withAction(action, requestErr)
		}
		requestErr = c.requestWithHeaders(ctx, http.MethodGet, "/x/web-interface/search/all/v2", query, nil, requestCredential, headers, &data)
	}
	if requestErr != nil {
		return nil, withAction(action, requestErr)
	}
	groups := mapList(mapValue(data)["result"])
	results := make([]map[string]any, 0)
	for _, group := range groups {
		resultType := stringValue(group["result_type"])
		for _, item := range mapList(group["data"]) {
			copy := make(map[string]any, len(item)+1)
			for key, value := range item {
				copy[key] = value
			}
			if resultType != "" {
				copy["result_type"] = resultType
			}
			results = append(results, copy)
		}
	}
	return results, nil
}

func (t SearchType) apiValue() (string, bool) {
	switch t {
	case SearchTypeAll:
		return "all", true
	case SearchTypeArticle:
		return "article", true
	case SearchTypeVideo:
		return "video", true
	case SearchTypeUser:
		return "bili_user", true
	case SearchTypeBangumi:
		return "media_bangumi", true
	case SearchTypeLive:
		return "live_room", true
	case SearchTypeMedia:
		return "media_ft", true
	default:
		return "", false
	}
}

func searchRequestHeaders(keyword, searchType string) http.Header {
	headers := make(http.Header)
	headers.Set("Origin", "https://search.bilibili.com")
	headers.Set("Referer", "https://search.bilibili.com/"+searchType+"?keyword="+url.QueryEscape(keyword))
	headers.Set("User-Agent", userAgent)
	return headers
}
