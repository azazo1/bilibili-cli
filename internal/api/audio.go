package api

import (
	"context"
	"net/http"
	"net/url"
)

func (c *Client) GetAudioURL(ctx context.Context, bvid string, cred *Credential) (string, error) {
	query := url.Values{
		"bvid":  []string{bvid},
		"fnval": []string{"16"},
		"fnver": []string{"0"},
		"fourk": []string{"1"},
	}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/player/playurl", query, nil, cred, &data); err != nil {
		return "", withAction("获取下载地址", err)
	}
	data = mapValue(data)
	if dash := mapValue(data["dash"]); len(dash) > 0 {
		for _, item := range mapList(dash["audio"]) {
			if value := stringValue(item["baseUrl"]); value != "" {
				return value, nil
			}
			if value := stringValue(item["base_url"]); value != "" {
				return value, nil
			}
		}
	}
	for _, key := range []string{"durl", "dash"} {
		if values := mapList(data[key]); len(values) > 0 {
			for _, item := range values {
				if value := stringValue(item["url"]); value != "" {
					return value, nil
				}
			}
		}
	}
	return "", NewError(CodeNotFound, "获取下载地址", "无法获取音频流")
}
