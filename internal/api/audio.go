package api

import (
	"context"
	"errors"
)

func (c *Client) GetAudioURL(ctx context.Context, bvid string, cred *Credential) (string, error) {
	urls, err := c.GetVideoDownloadURLs(ctx, bvid, cred)
	if err != nil {
		var apiErr *Error
		if errors.As(err, &apiErr) && apiErr.Message == "无法获取音视频流" {
			return "", NewError(CodeNotFound, "获取下载地址", "无法获取音频流")
		}
		return "", err
	}
	if urls.AudioURL != "" {
		return urls.AudioURL, nil
	}
	if urls.CombinedURL != "" {
		return urls.CombinedURL, nil
	}
	return "", NewError(CodeNotFound, "获取下载地址", "无法获取音频流")
}
