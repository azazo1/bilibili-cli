package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
)

var bvidPattern = regexp.MustCompile(`\bBV[0-9A-Za-z]{10}\b`)

func ExtractBVID(value string) (string, error) {
	if match := bvidPattern.FindString(value); match != "" {
		return match, nil
	}
	return "", NewError(CodeInvalidInput, "", fmt.Sprintf("无法提取 BV 号: %s", value))
}

func (c *Client) GetVideoInfo(ctx context.Context, bvid string, cred *Credential) (map[string]any, error) {
	var data map[string]any
	err := c.request(ctx, http.MethodGet, "/x/web-interface/view", url.Values{"bvid": []string{bvid}}, nil, cred, &data)
	if err != nil {
		return nil, withAction("获取视频信息", err)
	}
	return mapValue(data), nil
}

func (c *Client) getVideoPages(ctx context.Context, bvid string, cred *Credential) ([]map[string]any, error) {
	var data []map[string]any
	err := c.request(ctx, http.MethodGet, "/x/player/pagelist", url.Values{"bvid": []string{bvid}}, nil, cred, &data)
	if err != nil {
		return nil, withAction("获取视频分P信息", err)
	}
	return data, nil
}

func (c *Client) getPlayerInfo(ctx context.Context, bvid string, cid int64, cred *Credential) (map[string]any, error) {
	query := url.Values{"bvid": []string{bvid}, "cid": []string{fmt.Sprintf("%d", cid)}}
	var data map[string]any
	err := c.request(ctx, http.MethodGet, "/x/player/v2", query, nil, cred, &data)
	if err != nil {
		return nil, withAction("获取播放器信息", err)
	}
	return mapValue(data), nil
}

func (c *Client) GetVideoAIConclusion(ctx context.Context, bvid string, cred *Credential) (map[string]any, error) {
	pages, err := c.getVideoPages(ctx, bvid, cred)
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return map[string]any{}, nil
	}
	cid := int64Value(pages[0]["cid"], 0)
	if cid == 0 {
		return map[string]any{}, nil
	}
	query := url.Values{"bvid": []string{bvid}, "cid": []string{fmt.Sprintf("%d", cid)}}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/web-interface/view/conclusion/get", query, nil, cred, &data); err != nil {
		return nil, withAction("获取 AI 总结", err)
	}
	return mapValue(data), nil
}

func (c *Client) GetVideoComments(ctx context.Context, bvid string, page int, cred *Credential) (map[string]any, error) {
	info, err := c.GetVideoInfo(ctx, bvid, cred)
	if err != nil {
		return nil, err
	}
	aid := int64Value(info["aid"], 0)
	if aid == 0 {
		return nil, NewError(CodeUpstream, "获取视频评论", "视频信息缺少 aid")
	}
	if page < 1 {
		page = 1
	}
	query := url.Values{
		"oid":  []string{fmt.Sprintf("%d", aid)},
		"type": []string{"1"},
		"pn":   []string{fmt.Sprintf("%d", page)},
		"ps":   []string{"20"},
		"sort": []string{"2"},
	}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/v2/reply", query, nil, cred, &data); err != nil {
		return nil, withAction("获取视频评论", err)
	}
	return mapValue(data), nil
}

func (c *Client) GetRelatedVideos(ctx context.Context, bvid string, cred *Credential) ([]map[string]any, error) {
	var data []map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/web-interface/archive/related", url.Values{"bvid": []string{bvid}}, nil, cred, &data); err != nil {
		return nil, withAction("获取相关推荐", err)
	}
	return data, nil
}
