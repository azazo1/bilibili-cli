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
		status = "2"
	}
	form := url.Values{"aid": []string{fmt.Sprintf("%d", aid)}, "bvid": []string{bvid}, "like": []string{status}, "csrf": []string{cred.BiliJct}}
	if err := c.request(ctx, http.MethodPost, "/x/web-interface/archive/like", nil, form, requestCredential, nil); err != nil {
		return withAction("点赞视频", err)
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
		"aid":      []string{fmt.Sprintf("%d", aid)},
		"bvid":     []string{bvid},
		"multiply": []string{fmt.Sprintf("%d", coins)},
		"like":     []string{"0"},
		"csrf":     []string{cred.BiliJct},
	}
	if err := c.request(ctx, http.MethodPost, "/x/web-interface/coin/add", nil, form, requestCredential, nil); err != nil {
		return withAction("投币", err)
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
	form := url.Values{"aid": []string{fmt.Sprintf("%d", aid)}, "bvid": []string{bvid}, "csrf": []string{cred.BiliJct}}
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
		"csrf":   []string{cred.BiliJct},
	}
	if err := c.request(ctx, http.MethodPost, "/x/relation/modify", nil, form, cred, nil); err != nil {
		return withAction("修改用户关系", err)
	}
	return nil
}
