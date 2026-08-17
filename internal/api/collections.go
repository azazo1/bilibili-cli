package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func requireCredential(action string, cred *Credential, write bool) error {
	if cred == nil || !cred.ValidForRead() {
		return NewError(CodeNotAuthenticated, action, "需要登录")
	}
	if write && !cred.ValidForWrite() {
		return NewError(CodePermissionDenied, action, "当前凭证不支持写操作, 缺少 bili_jct")
	}
	return nil
}

func (c *Client) GetFavoriteList(ctx context.Context, cred *Credential) ([]map[string]any, error) {
	if err := requireCredential("获取收藏夹列表", cred, false); err != nil {
		return nil, err
	}
	me, err := c.GetSelfInfo(ctx, cred)
	if err != nil {
		return nil, err
	}
	uid := int64Value(me["mid"], 0)
	if uid == 0 {
		return nil, NewError(CodeUpstream, "获取收藏夹列表", "当前用户信息缺少 mid")
	}
	query := url.Values{"up_mid": []string{fmt.Sprintf("%d", uid)}}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/v3/fav/folder/created/list-all", query, nil, cred, &data); err != nil {
		return nil, withAction("获取收藏夹列表", err)
	}
	return mapList(mapValue(data)["list"]), nil
}

func (c *Client) GetFavoriteVideos(ctx context.Context, favID int64, page int, cred *Credential) (map[string]any, error) {
	if err := requireCredential("获取收藏夹内容", cred, false); err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	query := url.Values{
		"media_id": []string{fmt.Sprintf("%d", favID)},
		"pn":       []string{fmt.Sprintf("%d", page)},
		"ps":       []string{"20"},
		"platform": []string{"web"},
	}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/v3/fav/resource/list", query, nil, cred, &data); err != nil {
		return nil, withAction("获取收藏夹内容", err)
	}
	return mapValue(data), nil
}

func (c *Client) GetWatchLater(ctx context.Context, cred *Credential) (map[string]any, error) {
	if err := requireCredential("获取稍后再看列表", cred, false); err != nil {
		return nil, err
	}
	var object map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/v2/history/toview", nil, nil, cred, &object); err == nil {
		if list := object["list"]; list != nil {
			return map[string]any{"list": list, "count": object["count"]}, nil
		}
	}
	var data []map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/v2/history/toview", nil, nil, cred, &data); err != nil {
		return nil, withAction("获取稍后再看列表", err)
	}
	for _, item := range data {
		if stringValue(item["name"]) == "稍后再看" || intValue(item["id"], 0) == 2 {
			response := mapValue(item["mediaListResponse"])
			return map[string]any{"list": response["list"], "count": response["count"]}, nil
		}
	}
	return map[string]any{"list": []any{}, "count": 0}, nil
}

func (c *Client) GetDynamicFeed(ctx context.Context, offset string, cred *Credential) (map[string]any, error) {
	if err := requireCredential("获取动态时间线", cred, false); err != nil {
		return nil, err
	}
	query := url.Values{"type": []string{"all"}}
	if offset != "" {
		if _, err := strconv.ParseInt(offset, 10, 64); err != nil {
			return nil, NewError(CodeInvalidInput, "获取动态时间线", fmt.Sprintf("offset 非法: %s", offset))
		}
		query.Set("offset", offset)
	}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/polymer/web-dynamic/v1/feed/all", query, nil, cred, &data); err != nil {
		return nil, withAction("获取动态时间线", err)
	}
	return mapValue(data), nil
}

func (c *Client) GetUserDynamics(ctx context.Context, uid int64, offset int64, needTop bool, cred *Credential) (map[string]any, error) {
	if err := requireCredential("获取用户动态", cred, false); err != nil {
		return nil, err
	}
	query := url.Values{
		"host_mid": []string{fmt.Sprintf("%d", uid)},
		"offset":   []string{fmt.Sprintf("%d", offset)},
		"need_top": []string{"0"},
	}
	if needTop {
		query.Set("need_top", "1")
	}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/polymer/web-dynamic/v1/feed/space", query, nil, cred, &data); err != nil {
		return nil, withAction("获取用户动态", err)
	}
	return mapValue(data), nil
}

func (c *Client) PostTextDynamic(ctx context.Context, text string, cred *Credential) (map[string]any, error) {
	if err := requireCredential("发布动态", cred, true); err != nil {
		return nil, err
	}
	if strings.TrimSpace(text) == "" {
		return nil, NewError(CodeInvalidInput, "发布动态", "文本不能为空")
	}
	requestCredential := c.credentialWithDevice(ctx, cred)
	query, err := c.signWBI(ctx, url.Values{
		"platform":                   []string{"web"},
		"csrf":                       []string{cred.BiliJct},
		"x-bili-device-req-json":     []string{`{"platform":"web","device":"pc"}`},
		"x-bili-web-req-json":        []string{`{"spm_id":"333.999"}`},
	}, requestCredential)
	if err != nil {
		return nil, withAction("发布动态", err)
	}
	body := map[string]any{
		"dyn_req": map[string]any{
			"content": map[string]any{
				"contents": []map[string]any{{"raw_text": strings.TrimSpace(text), "biz_id": "", "type": 1}},
			},
			"scene": 1,
			"meta": map[string]any{"app_meta": map[string]any{"from": "create.dynamic.web", "mobi_app": "web"}},
			"attach_card": nil,
		},
	}
	var data map[string]any
	if err := c.requestJSON(ctx, http.MethodPost, "/x/dynamic/feed/create/dyn", query, body, requestCredential, &data); err != nil {
		return nil, withAction("发布动态", err)
	}
	return mapValue(data), nil
}

func (c *Client) DeleteDynamic(ctx context.Context, dynamicID int64, cred *Credential) error {
	if err := requireCredential("删除动态", cred, true); err != nil {
		return err
	}
	requestCredential := c.credentialWithDevice(ctx, cred)
	query := url.Values{"platform": []string{"web"}, "csrf": []string{cred.BiliJct}}
	webBody := map[string]any{"dyn_id_str": fmt.Sprintf("%d", dynamicID)}
	if err := c.requestJSON(ctx, http.MethodPost, "/x/dynamic/feed/operate/remove", query, webBody, requestCredential, nil); err == nil {
		return nil
	} else if c.Logger != nil {
		c.Logger.Debug("新版动态删除接口失败, 回退到兼容接口", "dynamic_id", dynamicID, "error", err)
	}
	form := url.Values{
		"dynamic_id": []string{fmt.Sprintf("%d", dynamicID)},
		"csrf":       []string{cred.BiliJct},
		"csrf_token": []string{cred.BiliJct},
	}
	if err := c.request(ctx, http.MethodPost, c.VCURL("/dynamic_svr/v1/dynamic_svr/rm_dynamic"), nil, form, requestCredential, nil); err != nil {
		return withAction("删除动态", err)
	}
	return nil
}
