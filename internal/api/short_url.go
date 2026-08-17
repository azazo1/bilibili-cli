package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) resolveB23Target(ctx context.Context, value string) (*url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, value, nil)
	if err != nil {
		return nil, NewError(CodeInvalidInput, "", "b23 短链无法解析")
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, &Error{Code: CodeNetwork, Action: "解析 b23 短链", Message: err.Error(), Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return nil, NewError(CodeNetwork, "解析 b23 短链", fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
	if resp.Request == nil || resp.Request.URL == nil {
		return nil, NewError(CodeUpstream, "解析 b23 短链", "短链响应缺少最终地址")
	}
	return resp.Request.URL, nil
}

func isB23Host(host string) bool {
	switch strings.ToLower(host) {
	case "b23.tv", "www.b23.tv":
		return true
	default:
		return false
	}
}
