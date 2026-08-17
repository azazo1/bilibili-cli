package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

const spaceBrowserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.2 Safari/605.1.15"

func spaceRequestHeaders(uid int64) http.Header {
	headers := make(http.Header)
	headers.Set("Origin", "https://space.bilibili.com")
	headers.Set("Referer", fmt.Sprintf("https://space.bilibili.com/%d", uid))
	headers.Set("User-Agent", spaceBrowserUserAgent)
	return headers
}

func (c *Client) GetSelfInfo(ctx context.Context, cred *Credential) (map[string]any, error) {
	if cred == nil || !cred.ValidForRead() {
		return nil, NewError(CodeNotAuthenticated, "获取当前登录用户信息", "需要有效的 SESSDATA")
	}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/web-interface/nav", nil, nil, cred, &data); err != nil {
		return nil, withAction("获取当前登录用户信息", err)
	}
	return mapValue(data), nil
}

func (c *Client) GetUserInfo(ctx context.Context, uid int64, cred *Credential) (map[string]any, error) {
	query := url.Values{
		"mid":          []string{fmt.Sprintf("%d", uid)},
		"token":        []string{""},
		"platform":     []string{"web"},
		"web_location": []string{"1550101"},
	}
	requestCredential := c.credentialWithDevice(ctx, cred)
	if webID := c.userWebID(ctx, uid, requestCredential); webID != "" {
		query.Set("w_webid", webID)
	}
	var data map[string]any
	if signed, signErr := c.signWBI(ctx, query, requestCredential); signErr == nil {
		if err := c.requestWithHeaders(ctx, http.MethodGet, "/x/space/wbi/acc/info", signed, nil, requestCredential, spaceRequestHeaders(uid), &data); err == nil {
			return mapValue(data), nil
		} else if CodeOf(err) == CodeRateLimited {
			return nil, withAction("获取用户信息", err)
		} else if c.Logger != nil {
			c.Logger.Debug("WBI 用户资料请求失败", "uid", uid, "error", err)
		}
	} else if c.Logger != nil {
		c.Logger.Debug("WBI 用户资料签名失败", "uid", uid, "error", signErr)
	}
	if err := c.request(ctx, http.MethodGet, "/x/space/acc/info", query, nil, requestCredential, &data); err != nil {
		return nil, withAction("获取用户信息", err)
	}
	return mapValue(data), nil
}

func (c *Client) GetUserRelationInfo(ctx context.Context, uid int64, cred *Credential) (map[string]any, error) {
	query := url.Values{"vmid": []string{fmt.Sprintf("%d", uid)}}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/relation/stat", query, nil, cred, &data); err != nil {
		return nil, withAction("获取用户关系信息", err)
	}
	return mapValue(data), nil
}

func (c *Client) GetUserVideos(ctx context.Context, uid int64, count int, cred *Credential) ([]map[string]any, error) {
	if count < 1 {
		return []map[string]any{}, nil
	}
	if count > 1000 {
		count = 1000
	}
	perPage := count
	if perPage > 50 {
		perPage = 50
	}
	result := make([]map[string]any, 0, count)
	requestCredential := c.credentialWithDevice(ctx, cred)
	if requestCredential.ValidForApp() && count <= 20 {
		query := url.Values{
			"vmid":       []string{fmt.Sprintf("%d", uid)},
			"ps":         []string{"20"},
			"pn":         []string{"1"},
			"qn":         []string{"80"},
			"order":      []string{"pubdate"},
			"build":      []string{"8430300"},
			"version":    []string{"8.43.0"},
			"channel":    []string{"master"},
			"mobi_app":   []string{"android"},
			"platform":   []string{"android"},
			"c_locale":  []string{"zh_CN"},
			"s_locale":  []string{"zh_CN"},
			"statistics": []string{`{"appId":1,"platform":3,"version":"8.43.0","abtest":""}`},
		}
		var appData map[string]any
		if err := c.RequestApp(ctx, http.MethodGet, "/x/v2/space/archive/cursor", query, nil, requestCredential, &appData); err == nil {
			if items := mapList(mapValue(appData)["item"]); len(items) > 0 {
				if len(items) > count {
					items = items[:count]
				}
				return items, nil
			}
			if c.Logger != nil {
				c.Logger.Debug("App 用户视频响应为空, 回退 WBI", "uid", uid)
			}
		} else if c.Logger != nil {
			c.Logger.Debug("App 用户视频请求失败, 回退 WBI", "uid", uid, "error", err)
		}
	}
	for page := 1; len(result) < count && page <= 20; page++ {
		query := url.Values{
			"mid":            []string{fmt.Sprintf("%d", uid)},
			"ps":             []string{fmt.Sprintf("%d", perPage)},
			"pn":             []string{fmt.Sprintf("%d", page)},
			"tid":            []string{"0"},
			"keyword":        []string{""},
			"order":          []string{"pubdate"},
			"platform":       []string{"web"},
			"web_location":   []string{"333.1387"},
			"order_avoided":  []string{"true"},
		}
		var data map[string]any
		var requestErr error
		if signed, signErr := c.signWBI(ctx, query, requestCredential); signErr == nil {
			requestErr = c.requestWithHeaders(ctx, http.MethodGet, "/x/space/wbi/arc/search", signed, nil, requestCredential, spaceRequestHeaders(uid), &data)
			if requestErr != nil && c.Logger != nil {
				c.Logger.Debug("WBI 用户视频请求失败", "uid", uid, "page", page, "error", requestErr)
			}
		} else {
			requestErr = signErr
			if c.Logger != nil {
				c.Logger.Debug("WBI 用户视频签名失败", "uid", uid, "page", page, "error", signErr)
			}
		}
		if requestErr != nil {
			if page == 1 {
				return nil, withAction("获取用户视频列表", requestErr)
			}
			break
		}
		listData := mapValue(data["list"])
		videos := mapList(listData["vlist"])
		if len(videos) == 0 {
			break
		}
		for _, video := range videos {
			result = append(result, video)
			if len(result) >= count {
				break
			}
		}
	}
	return result, nil
}

func (c *Client) SearchUser(ctx context.Context, keyword string, page int) ([]map[string]any, error) {
	return c.search(ctx, keyword, "bili_user", page, "搜索用户")
}

func (c *Client) SearchVideo(ctx context.Context, keyword string, page int) ([]map[string]any, error) {
	return c.search(ctx, keyword, "video", page, "搜索视频")
}

func (c *Client) search(ctx context.Context, keyword, searchType string, page int, action string) ([]map[string]any, error) {
	if page < 1 {
		page = 1
	}
	query := url.Values{
		"keyword":     []string{keyword},
		"search_type": []string{searchType},
		"page":        []string{fmt.Sprintf("%d", page)},
		"page_size":   []string{"20"},
		"platform":    []string{"pc"},
		"web_location": []string{"1430654"},
		"order":       []string{"pubdate"},
		"order_avoided": []string{"true"},
	}
	requestCredential := c.credentialWithDevice(ctx, nil)
	var data map[string]any
	var requestErr error
	if signed, signErr := c.signWBI(ctx, query, requestCredential); signErr == nil {
		requestErr = c.request(ctx, http.MethodGet, "/x/web-interface/wbi/search/type", signed, nil, requestCredential, &data)
	} else {
		requestErr = signErr
	}
	if requestErr != nil {
		if CodeOf(requestErr) == CodeRateLimited {
			return nil, withAction(action, requestErr)
		}
		fallbackQuery := url.Values{
			"keyword":     []string{keyword},
			"search_type": []string{searchType},
			"page":        []string{fmt.Sprintf("%d", page)},
		}
		requestErr = c.request(ctx, http.MethodGet, "/x/web-interface/search/type", fallbackQuery, nil, requestCredential, &data)
	}
	if requestErr != nil {
		return nil, withAction(action, requestErr)
	}
	return mapList(mapValue(data)["result"]), nil
}

func (c *Client) GetFollowings(ctx context.Context, uid int64, page, size int, cred *Credential) (map[string]any, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	query := url.Values{
		"vmid": []string{fmt.Sprintf("%d", uid)},
		"pn":   []string{fmt.Sprintf("%d", page)},
		"ps":   []string{fmt.Sprintf("%d", size)},
	}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/relation/followings", query, nil, cred, &data); err != nil {
		return nil, withAction("获取关注列表", err)
	}
	return mapValue(data), nil
}

func (c *Client) GetWatchHistory(ctx context.Context, page, count int, cred *Credential) (any, error) {
	if cred == nil || !cred.ValidForRead() {
		return nil, NewError(CodeNotAuthenticated, "获取观看历史", "需要登录")
	}
	if page < 1 {
		page = 1
	}
	if count < 1 {
		count = 1
	}
	if count > 100 {
		count = 100
	}
	query := url.Values{
		"type":   []string{"archive"},
		"ps":     []string{fmt.Sprintf("%d", count)},
		"max":    []string{"0"},
		"view_at": []string{"0"},
	}
	if page > 1 {
		query.Set("view_at", fmt.Sprintf("%d", (page-1)*count))
	}
	var object map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/web-interface/history/cursor", query, nil, cred, &object); err != nil {
		return nil, withAction("获取观看历史", err)
	}
	return mapValue(object), nil
}
