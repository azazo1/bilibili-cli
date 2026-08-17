package api

import (
	"bytes"
	"context"
	"encoding/json"
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

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0"

type Client struct {
	BaseURL   string
	VCBaseURL string
	HTTP      *http.Client
	UserAgent string
	Logger    *slog.Logger
	wbiMu     sync.Mutex
	wbiKey    string
	wbiExpires time.Time
	deviceMu  sync.Mutex
	device    *Credential
	deviceExpires time.Time
	webIDMu   sync.Mutex
	webIDs    map[int64]webIDEntry
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
	timeout := 30 * time.Second
	if raw := strings.TrimSpace(os.Getenv("BILI_HTTP_TIMEOUT")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}
	return &Client{
		BaseURL:   base,
		VCBaseURL: vcBase,
		HTTP:      &http.Client{Timeout: timeout},
		UserAgent: userAgent,
		Logger:    slog.Default(),
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
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(c.BaseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func (c *Client) VCURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(c.VCBaseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func (c *Client) Request(ctx context.Context, method, path string, query url.Values, form url.Values, cred *Credential, out any) error {
	return c.request(ctx, method, path, query, form, cred, out)
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
	var body io.Reader
	contentType := ""
	if form != nil {
		body = strings.NewReader(form.Encode())
		contentType = "application/x-www-form-urlencoded"
	}
	return c.requestWithBody(ctx, method, path, query, body, contentType, cred, out)
}

func (c *Client) requestJSON(ctx context.Context, method, path string, query url.Values, payload any, cred *Credential, out any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return &Error{Code: CodeInvalidInput, Message: "JSON 请求体编码失败", Err: err}
	}
	return c.requestWithBody(ctx, method, path, query, bytes.NewReader(encoded), "application/json", cred, out)
}

func (c *Client) requestWithBody(ctx context.Context, method, path string, query url.Values, body io.Reader, contentType string, cred *Credential, out any) error {
	requestURL := c.URL(path)
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
		return mapAPIError(*env.Code, env.Message)
	}
	if out == nil || len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := decodeJSON(env.Data, out); err != nil {
		return &Error{Code: CodeUpstream, Message: "响应数据格式异常", Err: err}
	}
	return nil
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
	case -412, 412:
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
