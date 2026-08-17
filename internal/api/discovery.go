package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func (c *Client) GetHotVideos(ctx context.Context, page, size int) (map[string]any, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	query := url.Values{"pn": []string{fmt.Sprintf("%d", page)}, "ps": []string{fmt.Sprintf("%d", size)}}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/web-interface/popular", query, nil, nil, &data); err != nil {
		return nil, withAction("获取热门视频", err)
	}
	return mapValue(data), nil
}

func (c *Client) GetRankVideos(ctx context.Context, day int) (map[string]any, error) {
	if day != 7 {
		day = 3
	}
	query := url.Values{"rid": []string{"0"}, "type": []string{"all"}, "day": []string{fmt.Sprintf("%d", day)}}
	requestCredential := c.credentialWithDevice(ctx, nil)
	signed, err := c.signWBI(ctx, query, requestCredential)
	if err != nil {
		return nil, withAction("获取排行榜", err)
	}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/web-interface/ranking/v2", signed, nil, requestCredential, &data); err != nil {
		return nil, withAction("获取排行榜", err)
	}
	return mapValue(data), nil
}
