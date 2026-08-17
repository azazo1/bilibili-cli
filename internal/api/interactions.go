package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func (c *Client) LikeVideo(ctx context.Context, bvid string, undo bool, cred *Credential) error {
	if err := requireCredential("点赞视频", cred, true); err != nil {
		return err
	}
	requestCredential := c.credentialWithDevice(ctx, cred)
	aid, err := c.videoAID(ctx, bvid, requestCredential)
	if err != nil {
		return err
	}
	status := "1"
	if undo {
		status = "0"
	}
	form := url.Values{"aid": []string{fmt.Sprintf("%d", aid)}, "like": []string{status}}
	var requestErr error
	if cred.ValidForApp() {
		requestErr = c.RequestApp(ctx, http.MethodPost, "/x/v2/view/like", nil, form, cred, nil)
	} else {
		form.Set("bvid", bvid)
		form.Set("csrf", cred.BiliJct)
		requestErr = c.request(ctx, http.MethodPost, "/x/web-interface/archive/like", nil, form, requestCredential, nil)
	}
	if requestErr != nil {
		return withAction("点赞视频", requestErr)
	}
	return nil
}

func (c *Client) CoinVideo(ctx context.Context, bvid string, coins int, cred *Credential) error {
	if err := requireCredential("投币", cred, true); err != nil {
		return err
	}
	if coins < 1 || coins > 2 {
		return NewError(CodeInvalidInput, "投币", "投币数量必须是 1 或 2")
	}
	requestCredential := c.credentialWithDevice(ctx, cred)
	aid, err := c.videoAID(ctx, bvid, requestCredential)
	if err != nil {
		return err
	}
	form := url.Values{
		"aid":        []string{fmt.Sprintf("%d", aid)},
		"multiply":   []string{fmt.Sprintf("%d", coins)},
		"select_like": []string{"0"},
	}
	var requestErr error
	if cred.ValidForApp() {
		requestErr = c.RequestApp(ctx, http.MethodPost, "/x/v2/view/coin/add", nil, form, cred, nil)
	} else {
		form.Set("bvid", bvid)
		form.Set("like", "0")
		form.Set("csrf", cred.BiliJct)
		requestErr = c.request(ctx, http.MethodPost, "/x/web-interface/coin/add", nil, form, requestCredential, nil)
	}
	if requestErr != nil {
		return withAction("投币", requestErr)
	}
	return nil
}

func (c *Client) TripleVideo(ctx context.Context, bvid string, cred *Credential) (map[string]any, error) {
	if err := requireCredential("一键三连", cred, true); err != nil {
		return nil, err
	}
	requestCredential := c.credentialWithDevice(ctx, cred)
	aid, err := c.videoAID(ctx, bvid, requestCredential)
	if err != nil {
		return nil, err
	}
	form := url.Values{
		"aid":        []string{fmt.Sprintf("%d", aid)},
		"eab_x":      []string{"2"},
		"ramval":     []string{"0"},
		"source":     []string{"web_normal"},
		"ga":         []string{"1"},
		"csrf":       []string{cred.BiliJct},
		"spmid":      []string{"333.788.0.0"},
		"statistics": []string{`{"appId":100,"platform":5}`},
	}
	var data map[string]any
	if err := c.request(ctx, http.MethodPost, "/x/web-interface/archive/like/triple", nil, form, requestCredential, &data); err != nil {
		return nil, withAction("一键三连", err)
	}
	return mapValue(data), nil
}

func (c *Client) videoAID(ctx context.Context, bvid string, cred *Credential) (int64, error) {
	info, err := c.GetVideoInfo(ctx, bvid, cred)
	if err != nil {
		return 0, err
	}
	aid := int64Value(info["aid"], 0)
	if aid == 0 {
		return 0, NewError(CodeUpstream, "获取视频信息", "视频信息缺少 aid")
	}
	return aid, nil
}

func (c *Client) UnfollowUser(ctx context.Context, uid int64, cred *Credential) error {
	return c.modifyUserRelation(ctx, uid, "2", cred)
}

func (c *Client) FollowUser(ctx context.Context, uid int64, cred *Credential) error {
	return c.modifyUserRelation(ctx, uid, "1", cred)
}

func (c *Client) modifyUserRelation(ctx context.Context, uid int64, action string, cred *Credential) error {
	if err := requireCredential("修改用户关系", cred, true); err != nil {
		return err
	}
	form := url.Values{
		"fid":    []string{fmt.Sprintf("%d", uid)},
		"act":    []string{action},
		"re_src": []string{"11"},
		"gaia_source": []string{"web_main"},
		"spmid":       []string{"333.1387"},
		"extend_content": []string{fmt.Sprintf(`{"entity":"user","entity_id":%d,"fp":"%s"}`, uid, spaceBrowserUserAgent)},
		"csrf":   []string{cred.BiliJct},
	}
	if err := c.requestWithHeaders(ctx, http.MethodPost, "/x/relation/modify", url.Values{"statistics": []string{`{"appId":100,"platform":5}`}, "x-bili-device-req-json": []string{`{"platform":"web","device":"pc","spmid":"333.1387"}`}}, form, cred, spaceRequestHeaders(uid), nil); err != nil {
		return withAction("修改用户关系", err)
	}
	return nil
}
