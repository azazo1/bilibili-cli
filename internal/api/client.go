package api

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultBaseURL = "https://api.bilibili.com"
const defaultVCBaseURL = "https://api.vc.bilibili.com"
const defaultPassportBaseURL = "https://passport.bilibili.com"
const defaultAppBaseURL = "https://app.bilibili.com"
const defaultGeetestBaseURL = "https://api.geetest.com"

const appKey = "dfca71928277209b"
const appSecret = "b5475a8825547a4fc26c7d518eaaa02e"
const appUserAgent = "Mozilla/5.0 BiliDroid/8.43.0 (bbcallen@gmail.com) os/android model/android mobi_app/android build/8430300 channel/master innerVer/8430300 osVer/15 network/2"

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.2 Safari/605.1.15"

type Client struct {
	BaseURL         string
	VCBaseURL       string
	PassportBaseURL string
	AppBaseURL      string
	GeetestBaseURL  string
	HTTP            *http.Client
	UserAgent       string
	Logger          *slog.Logger
	wbiMu           sync.Mutex
	wbiKey          string
	wbiExpires      time.Time
	deviceMu        sync.Mutex
	device          *Credential
	deviceExpires   time.Time
	passportMu       sync.Mutex
	passportBuvid    string
	passportDeviceID string
	webIDMu         sync.Mutex
	webIDs          map[int64]webIDEntry
}

func NewClient() *Client {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("BILI_API_BASE_URL")), "/")
	if base == "" {
		base = defaultBaseURL
	}
	vcBase := strings.TrimRight(strings.TrimSpace(os.Getenv("BILI_VC_API_BASE_URL")), "/")
	if vcBase == "" {
		vcBase = defaultVCBaseURL
	}
	passportBase := strings.TrimRight(strings.TrimSpace(os.Getenv("BILI_PASSPORT_BASE_URL")), "/")
	if passportBase == "" {
		passportBase = defaultPassportBaseURL
	}
	appBase := strings.TrimRight(strings.TrimSpace(os.Getenv("BILI_APP_BASE_URL")), "/")
	if appBase == "" {
		appBase = defaultAppBaseURL
	}
	timeout := 30 * time.Second
	if raw := strings.TrimSpace(os.Getenv("BILI_HTTP_TIMEOUT")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}
	return &Client{
		BaseURL:         base,
		VCBaseURL:       vcBase,
		PassportBaseURL: passportBase,
		AppBaseURL:      appBase,
		GeetestBaseURL:  defaultGeetestBaseURL,
		HTTP:            &http.Client{Timeout: timeout},
		UserAgent:       userAgent,
		Logger:          slog.Default(),
	}
}

func (c *Client) SetTimeout(seconds int) {
	if seconds <= 0 {
		return
	}
	if c.HTTP == nil {
		c.HTTP = &http.Client{}
	}
	c.HTTP.Timeout = time.Duration(seconds) * time.Second
}

func (c *Client) URL(path string) string {
	return joinURL(c.BaseURL, defaultBaseURL, path)
}

func (c *Client) VCURL(path string) string {
	return joinURL(c.VCBaseURL, defaultVCBaseURL, path)
}

func (c *Client) PassportURL(path string) string {
	return joinURL(c.PassportBaseURL, defaultPassportBaseURL, path)
}

func (c *Client) AppURL(path string) string {
	return joinURL(c.AppBaseURL, defaultAppBaseURL, path)
}

func joinURL(base, fallback, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if strings.TrimSpace(base) == "" {
		base = fallback
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func (c *Client) Request(ctx context.Context, method, path string, query url.Values, form url.Values, cred *Credential, out any) error {
	return c.request(ctx, method, path, query, form, cred, out)
}

func (c *Client) RequestPassport(ctx context.Context, method, path string, query url.Values, form url.Values, cred *Credential, out any) error {
	return c.requestAtBase(ctx, method, c.PassportBaseURL, defaultPassportBaseURL, path, query, form, cred, out)
}

func (c *Client) RequestApp(ctx context.Context, method, path string, query url.Values, form url.Values, cred *Credential, out any) error {
	return c.requestAppAtBase(ctx, method, c.AppBaseURL, defaultAppBaseURL, path, query, form, cred, out)
}

func (c *Client) RequestPassportApp(ctx context.Context, method, path string, query url.Values, form url.Values, cred *Credential, out any) error {
	return c.requestAppAtBase(ctx, method, c.PassportBaseURL, defaultPassportBaseURL, path, query, form, cred, out)
}

func (c *Client) RequestJSON(ctx context.Context, method, path string, query url.Values, payload any, cred *Credential, out any) error {
	return c.requestJSON(ctx, method, path, query, payload, cred, out)
}

type envelope struct {
	Code    *int            `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, form url.Values, cred *Credential, out any) error {
	return c.requestAtBase(ctx, method, c.BaseURL, defaultBaseURL, path, query, form, cred, out)
}

func (c *Client) requestAtBase(ctx context.Context, method, base, fallback, path string, query url.Values, form url.Values, cred *Credential, out any) error {
	return c.requestAtBaseWithHeaders(ctx, method, base, fallback, path, query, form, cred, nil, out)
}

func (c *Client) requestAtBaseWithHeaders(ctx context.Context, method, base, fallback, path string, query url.Values, form url.Values, cred *Credential, headers http.Header, out any) error {
	var body io.Reader
	contentType := ""
	if form != nil {
		body = strings.NewReader(form.Encode())
		contentType = "application/x-www-form-urlencoded"
	}
	return c.requestWithBody(ctx, method, joinURL(base, fallback, path), query, body, contentType, cred, headers, out)
}

func (c *Client) requestWithHeaders(ctx context.Context, method, path string, query url.Values, form url.Values, cred *Credential, headers http.Header, out any) error {
	return c.requestAtBaseWithHeaders(ctx, method, c.BaseURL, defaultBaseURL, path, query, form, cred, headers, out)
}

func (c *Client) requestAppAtBase(ctx context.Context, method, base, fallback, path string, query url.Values, form url.Values, cred *Credential, out any) error {
	headers := appRequestHeaders(cred)
	if form != nil {
		return c.requestAtBaseWithHeaders(ctx, method, base, fallback, path, query, c.signAppValues(form, cred), nil, headers, out)
	}
	return c.requestAtBaseWithHeaders(ctx, method, base, fallback, path, c.signAppValues(query, cred), nil, nil, headers, out)
}

func (c *Client) requestJSON(ctx context.Context, method, path string, query url.Values, payload any, cred *Credential, out any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return &Error{Code: CodeInvalidInput, Message: "JSON 请求体编码失败", Err: err}
	}
	return c.requestWithBody(ctx, method, c.URL(path), query, bytes.NewReader(encoded), "application/json", cred, nil, out)
}

func (c *Client) requestWithBody(ctx context.Context, method, requestURL string, query url.Values, body io.Reader, contentType string, cred *Credential, headers http.Header, out any) error {
	attempts := 1
	if method == http.MethodGet && body == nil {
		attempts = 3
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		err := c.requestWithBodyOnce(ctx, method, requestURL, query, body, contentType, cred, headers, out)
		if err == nil || attempt == attempts || !isRetryableReadError(err) {
			return err
		}
		lastErr = err
		if c.Logger != nil {
			c.Logger.Warn("读取请求失败, 准备重试", "method", method, "url", requestURL, "attempt", attempt, "error", err)
		}
		wait := time.Duration(attempt) * 500 * time.Millisecond
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return lastErr
}

func (c *Client) requestWithBodyOnce(ctx context.Context, method, requestURL string, query url.Values, body io.Reader, contentType string, cred *Credential, headers http.Header, out any) error {
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return &Error{Code: CodeInvalidInput, Message: err.Error(), Err: err}
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Referer", "https://www.bilibili.com/")
	for name, values := range headers {
		req.Header.Del(name)
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if cred != nil {
		cred.Apply(req)
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return &Error{Code: CodeNetwork, Message: err.Error(), Err: err}
	}
	defer resp.Body.Close()
	payload, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return &Error{Code: CodeNetwork, HTTPStatus: resp.StatusCode, Message: readErr.Error(), Err: readErr}
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusPreconditionFailed {
		return &Error{Code: CodeRateLimited, HTTPStatus: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &Error{Code: CodeNotAuthenticated, HTTPStatus: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &Error{Code: CodeNetwork, HTTPStatus: resp.StatusCode, Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	var env envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return &Error{Code: CodeUpstream, HTTPStatus: resp.StatusCode, Message: "响应不是有效 JSON", Err: err}
	}
	if env.Code != nil && *env.Code != 0 {
		apiErr := mapAPIError(*env.Code, env.Message)
		apiErr.Data = append(json.RawMessage(nil), env.Data...)
		return apiErr
	}
	if out == nil || len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := decodeJSON(env.Data, out); err != nil {
		return &Error{Code: CodeUpstream, Message: "响应数据格式异常", Err: err}
	}
	return nil
}

func (c *Client) signAppValues(values url.Values, cred *Credential) url.Values {
	signed := make(url.Values, len(values)+3)
	for name, items := range values {
		signed[name] = append([]string(nil), items...)
	}
	if cred != nil && cred.AccessKey != "" {
		signed.Set("access_key", cred.AccessKey)
	}
	signed.Set("appkey", appKey)
	signed.Set("ts", strconv.FormatInt(time.Now().Unix(), 10))
	sum := md5.Sum([]byte(signed.Encode() + appSecret))
	signed.Set("sign", hex.EncodeToString(sum[:]))
	return signed
}

func appRequestHeaders(cred *Credential) http.Header {
	headers := make(http.Header)
	headers.Set("User-Agent", appUserAgent)
	headers.Set("env", "prod")
	headers.Set("app-key", "android64")
	headers.Set("x-bili-aurora-zone", "sh001")
	if cred != nil && cred.DedeUserID != "" {
		headers.Set("x-bili-mid", cred.DedeUserID)
	}
	return headers
}

func isRetryableReadError(err error) bool {
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Code == CodeNetwork && (apiErr.HTTPStatus == 0 || apiErr.HTTPStatus >= http.StatusInternalServerError)
}

func decodeJSON(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode(out)
}

func (c *Client) rawJSON(ctx context.Context, path string, query url.Values, cred *Credential) (json.RawMessage, error) {
	requestURL := c.URL(path)
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	if cred != nil {
		cred.Apply(req)
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, &Error{Code: CodeNetwork, Message: err.Error(), Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusPreconditionFailed {
		return nil, &Error{Code: CodeRateLimited, HTTPStatus: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &Error{Code: CodeNetwork, HTTPStatus: resp.StatusCode, Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &Error{Code: CodeNetwork, Message: err.Error(), Err: err}
	}
	return json.RawMessage(data), nil
}

func mapAPIError(code int, message string) *Error {
	if message == "" {
		message = "Bilibili API error"
	}
	result := &Error{APIStatus: code, Message: fmt.Sprintf("[%d] %s", code, message)}
	switch code {
	case -101, -111:
		result.Code = CodeNotAuthenticated
	case -404, 62002, 62004:
		result.Code = CodeNotFound
	case -412, 412, -799:
		result.Code = CodeRateLimited
	default:
		result.Code = CodeUpstream
	}
	return result
}

func withAction(action string, err error) error {
	if err == nil {
		return nil
	}
	var apiErr *Error
	if errors.As(err, &apiErr) {
		copy := *apiErr
		copy.Action = action
		return &copy
	}
	return &Error{Code: CodeInternal, Action: action, Message: err.Error(), Err: err}
}
