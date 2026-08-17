package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func (c *Client) GetAudioURL(ctx context.Context, bvid string, cred *Credential) (string, error) {
	pages, err := c.getVideoPages(ctx, bvid, cred)
	if err != nil {
		return "", err
	}
	if len(pages) == 0 {
		return "", NewError(CodeNotFound, "获取下载地址", "视频没有可用分P")
	}
	cid := int64Value(pages[0]["cid"], 0)
	if cid == 0 {
		return "", NewError(CodeUpstream, "获取下载地址", "视频分P信息缺少 cid")
	}
	requestCredential := c.credentialWithDevice(ctx, cred)
	query := url.Values{
		"bvid":  []string{bvid},
		"cid":   []string{fmt.Sprintf("%d", cid)},
		"qn":    []string{"80"},
		"fnval": []string{"4048"},
		"fnver": []string{"0"},
		"fourk": []string{"1"},
		"gaia_source":  []string{"pre-load"},
		"isGaiaAvoided": []string{"true"},
		"web_location":  []string{"1315873"},
	}
	var data map[string]any
	var requestErr error = NewError(CodeUpstream, "获取下载地址", "无法获取音频流")
	if signed, signErr := c.signWBI(ctx, query, requestCredential); signErr == nil {
		requestErr = c.request(ctx, http.MethodGet, "/x/player/wbi/playurl", signed, nil, requestCredential, &data)
	} else {
		requestErr = signErr
	}
	if requestErr != nil && CodeOf(requestErr) == CodeRateLimited {
		return "", withAction("获取下载地址", requestErr)
	}
	if requestErr != nil {
		requestErr = c.request(ctx, http.MethodGet, "/x/player/playurl", query, nil, requestCredential, &data)
	}
	if requestErr != nil {
		return "", withAction("获取下载地址", requestErr)
	}
	return audioURLFromPlayURL(data)
}

func audioURLFromPlayURL(data map[string]any) (string, error) {
	data = mapValue(data)
	if dash := mapValue(data["dash"]); len(dash) > 0 {
		for _, item := range mapList(dash["audio"]) {
			for _, key := range []string{"baseUrl", "base_url", "url"} {
				if value := stringValue(item[key]); value != "" {
					return value, nil
				}
			}
			for _, backup := range mapList(item["backupUrl"]) {
				if value := stringValue(backup["url"]); value != "" {
					return value, nil
				}
			}
			for _, backup := range mapList(item["backup_url"]) {
				if value := stringValue(backup["url"]); value != "" {
					return value, nil
				}
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
