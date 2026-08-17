package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type GeetestChallenge struct {
	GT             string
	Challenge      string
	RecaptchaToken string
}

func (c *Client) GetLoginGeetestChallenge(ctx context.Context, recaptchaURL string) (GeetestChallenge, error) {
	if challenge, ok := geetestChallengeFromURL(recaptchaURL); ok {
		return challenge, nil
	}
	var data map[string]any
	query := url.Values{"source": []string{"main_web"}}
	if err := c.RequestPassport(ctx, http.MethodGet, "/x/passport-login/captcha", query, nil, nil, &data); err != nil {
		return GeetestChallenge{}, withAction("获取人机验证参数", err)
	}
	geetest := mapValue(data["geetest"])
	challenge := GeetestChallenge{
		GT:             stringValue(geetest["gt"]),
		Challenge:      stringValue(geetest["challenge"]),
		RecaptchaToken: firstStringValue(data["recaptcha_token"], data["token"]),
	}
	if !challenge.Valid() {
		return GeetestChallenge{}, NewError(CodeUpstream, "获取人机验证参数", "响应缺少 Geetest 参数")
	}
	return challenge, nil
}

func (c GeetestChallenge) Valid() bool {
	return strings.TrimSpace(c.GT) != "" && strings.TrimSpace(c.Challenge) != "" && strings.TrimSpace(c.RecaptchaToken) != ""
}

func geetestChallengeFromURL(rawURL string) (GeetestChallenge, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return GeetestChallenge{}, false
	}
	query := parsed.Query()
	challenge := GeetestChallenge{
		GT:             query.Get("gee_gt"),
		Challenge:      query.Get("gee_challenge"),
		RecaptchaToken: query.Get("recaptcha_token"),
	}
	return challenge, challenge.Valid()
}

func (c *Client) GetGeetestConfig(ctx context.Context, challenge GeetestChallenge) (map[string]any, error) {
	if !challenge.Valid() {
		return nil, NewError(CodeInvalidInput, "获取 Geetest 配置", "Geetest 参数不能为空")
	}
	endpoint := joinURL(c.GeetestBaseURL, defaultGeetestBaseURL, "/gettype.php")
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, NewError(CodeInvalidInput, "获取 Geetest 配置", err.Error())
	}
	query := parsed.Query()
	query.Set("gt", challenge.GT)
	parsed.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, NewError(CodeInvalidInput, "获取 Geetest 配置", err.Error())
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, &Error{Code: CodeNetwork, Action: "获取 Geetest 配置", Message: err.Error(), Err: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, &Error{Code: CodeNetwork, Action: "获取 Geetest 配置", HTTPStatus: resp.StatusCode, Message: err.Error(), Err: err}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, NewError(CodeNetwork, "获取 Geetest 配置", fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
	data, err := parseGeetestConfig(body)
	if err != nil {
		return nil, NewError(CodeUpstream, "获取 Geetest 配置", err.Error())
	}
	data["gt"] = challenge.GT
	data["challenge"] = challenge.Challenge
	data["offline"] = false
	data["new_captcha"] = true
	data["product"] = "bind"
	data["width"] = "100%"
	data["https"] = true
	data["protocol"] = "https://"
	return data, nil
}

func parseGeetestConfig(body []byte) (map[string]any, error) {
	text := strings.TrimSpace(string(body))
	start, end := strings.IndexByte(text, '{'), strings.LastIndexByte(text, '}')
	if start < 0 || end <= start {
		return nil, fmt.Errorf("Geetest 配置不是有效 JSONP")
	}
	var response struct {
		Status string         `json:"status"`
		Data   map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &response); err != nil {
		return nil, fmt.Errorf("解析 Geetest 配置失败: %w", err)
	}
	if response.Status != "" && response.Status != "success" {
		return nil, fmt.Errorf("Geetest 返回状态: %s", response.Status)
	}
	if len(response.Data) == 0 {
		return nil, fmt.Errorf("Geetest 配置缺少 data")
	}
	return response.Data, nil
}
