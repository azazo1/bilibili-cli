package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/azazo1/bilibili-cli/internal/api"
)

type qrGenerateResponse struct {
	URL       string `json:"url"`
	QRCodeKey string `json:"qrcode_key"`
}

type qrPollEnvelope struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

const qrWebSource = "main-fe-header"

func (s *Store) QRLogin(ctx context.Context, out io.Writer) (*api.Credential, error) {
	session, err := s.newQRLoginSession()
	if err != nil {
		return nil, api.NewError(api.CodeInternal, "登录", err.Error())
	}
	return session.qrLogin(ctx, out)
}

func (s *Store) newQRLoginSession() (*Store, error) {
	if s.Client == nil {
		return nil, fmt.Errorf("登录客户端未初始化")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	baseClient := s.Client.HTTP
	if baseClient == nil {
		baseClient = http.DefaultClient
	}
	httpClient := *baseClient
	httpClient.Jar = jar
	apiClient := *s.Client
	apiClient.HTTP = &httpClient
	session := *s
	session.Client = &apiClient
	return &session, nil
}

func (s *Store) qrLogin(ctx context.Context, out io.Writer) (*api.Credential, error) {
	var generated qrGenerateResponse
	query := url.Values{"source": []string{qrWebSource}}
	if err := s.Client.RequestPassport(ctx, http.MethodGet, "/x/passport-login/web/qrcode/generate", query, nil, nil, &generated); err != nil {
		return nil, api.NewError(api.CodeNetwork, "登录", err.Error())
	}
	if generated.URL == "" || generated.QRCodeKey == "" {
		return nil, api.NewError(api.CodeUpstream, "登录", "二维码响应缺少地址")
	}
	if out != nil {
		fmt.Fprintln(out, "请使用 Bilibili App 扫描以下二维码登录:")
		if rendered, err := renderQR(generated.URL); err == nil {
			fmt.Fprintln(out, rendered)
		} else {
			fmt.Fprintln(out, generated.URL)
		}
		fmt.Fprintf(out, "%s\n", generated.URL)
		fmt.Fprintln(out, "扫码后请在手机上确认登录...")
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		credential, state, err := s.pollQRCode(ctx, generated.QRCodeKey)
		if err != nil {
			return nil, err
		}
		switch state {
		case "done":
			if err := s.Save(credential); err != nil {
				return nil, api.NewError(api.CodeInternal, "登录", err.Error())
			}
			if out != nil {
				fmt.Fprintln(out, "登录成功, 凭证已保存")
			}
			return credential, nil
		case "timeout":
			return nil, api.NewError(api.CodeInvalidInput, "登录", "二维码已过期, 请重试")
		case "confirmed":
			if out != nil {
				fmt.Fprintln(out, "已扫码, 请在手机上确认...")
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Store) pollQRCode(ctx context.Context, key string) (*api.Credential, string, error) {
	query := url.Values{
		"qrcode_key": []string{key},
		"source":     []string{qrWebSource},
	}
	requestURL := s.Client.PassportURL("/x/passport-login/web/qrcode/poll") + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", s.Client.UserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://www.bilibili.com/")
	client := s.Client.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", api.NewError(api.CodeNetwork, "登录", err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", api.NewError(api.CodeNetwork, "登录", fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", api.NewError(api.CodeNetwork, "登录", err.Error())
	}
	var envelope qrPollEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, "", api.NewError(api.CodeUpstream, "登录", err.Error())
	}
	pollCode := qrPollCode(envelope.Data, envelope.Code)
	switch pollCode {
	case 0:
		loginURL := apiString(envelope.Data["url"])
		credential := credentialFromPollData(envelope.Data, resp.Cookies())
		if credential == nil && loginURL != "" {
			credential, _ = s.credentialFromLoginRedirect(ctx, loginURL)
		}
		if credential == nil {
			return nil, "", api.NewError(api.CodeUpstream, "登录", "登录响应缺少凭证")
		}
		if refreshToken := apiString(envelope.Data["refresh_token"]); refreshToken != "" {
			credential.AcTimeValue = refreshToken
		}
		return credential, "done", nil
	case 86038:
		return nil, "timeout", nil
	case 86090:
		return nil, "confirmed", nil
	case 86101:
		return nil, "waiting", nil
	default:
		message := apiString(envelope.Data["message"])
		if message == "" {
			message = envelope.Message
		}
		return nil, "", api.NewError(api.CodeUpstream, "登录", fmt.Sprintf("[%d] %s", pollCode, message))
	}
}

func qrPollCode(data map[string]any, fallback int) int {
	value, ok := data["code"]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed)
		}
	}
	return fallback
}

func credentialFromLoginURL(raw string) *api.Credential {
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	values := rawQueryValues(parsed.RawQuery)
	return credentialFromCookies(map[string]string{
		"SESSDATA":    values["SESSDATA"],
		"bili_jct":    values["bili_jct"],
		"ac_time_value": values["ac_time_value"],
		"buvid3":      values["buvid3"],
		"buvid4":      values["buvid4"],
		"DedeUserID":  values["DedeUserID"],
	})
}

func credentialFromPollData(data map[string]any, responseCookies []*http.Cookie) *api.Credential {
	if credential := credentialFromLoginURL(apiString(data["url"])); credential != nil {
		return credential
	}
	if credential := credentialFromCookieInfo(data["cookie_info"]); credential != nil {
		return credential
	}
	return credentialFromResponseCookies(responseCookies)
}

func credentialFromCookieInfo(value any) *api.Credential {
	info, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	items, ok := info["cookies"].([]any)
	if !ok {
		return nil
	}
	values := make(map[string]string, len(items))
	for _, item := range items {
		cookie, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := apiString(cookie["name"])
		if name != "" {
			values[name] = apiString(cookie["value"])
		}
	}
	return credentialFromCookies(values)
}

func credentialFromResponseCookies(cookies []*http.Cookie) *api.Credential {
	values := make(map[string]string, len(cookies))
	for _, cookie := range cookies {
		if cookie != nil && cookie.Name != "" {
			values[cookie.Name] = cookie.Value
		}
	}
	return credentialFromCookies(values)
}

func (s *Store) credentialFromLoginRedirect(ctx context.Context, rawURL string) (*api.Credential, error) {
	if strings.HasPrefix(rawURL, "//") {
		rawURL = "https:" + rawURL
	}
	loginURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	host := strings.ToLower(loginURL.Hostname())
	if loginURL.Scheme == "" || (host != "bilibili.com" && !strings.HasSuffix(host, ".bilibili.com")) {
		return nil, nil
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	baseClient := s.Client.HTTP
	if baseClient == nil {
		baseClient = http.DefaultClient
	}
	redirectClient := *baseClient
	redirectClient.Jar = jar
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, loginURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.Client.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Referer", "https://www.bilibili.com/")
	resp, err := redirectClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return nil, err
	}
	cookies := append([]*http.Cookie{}, resp.Cookies()...)
	cookies = append(cookies, jar.Cookies(loginURL)...)
	homeURL := &url.URL{Scheme: "https", Host: "www.bilibili.com"}
	cookies = append(cookies, jar.Cookies(homeURL)...)
	return credentialFromResponseCookies(cookies), nil
}

func rawQueryValues(raw string) map[string]string {
	values := make(map[string]string)
	for _, part := range strings.Split(raw, "&") {
		pair := strings.SplitN(part, "=", 2)
		if len(pair) != 2 {
			continue
		}
		values[pair[0]] = pair[1]
	}
	return values
}

func renderQR(content string) (string, error) {
	code, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return "", err
	}
	return renderCompactQR(code.Bitmap()), nil
}

func renderCompactQR(matrix [][]bool) string {
	matrix = addQRQuietZone(matrix)
	if len(matrix) == 0 || len(matrix[0]) == 0 {
		return ""
	}
	var builder strings.Builder
	for row := 0; row < len(matrix); row += 2 {
		top := matrix[row]
		bottom := make([]bool, len(top))
		if row+1 < len(matrix) {
			bottom = matrix[row+1]
		}
		for column, topDark := range top {
			bottomDark := column < len(bottom) && bottom[column]
			switch {
			case topDark && bottomDark:
				builder.WriteByte(' ')
			case topDark:
				builder.WriteRune('▄')
			case bottomDark:
				builder.WriteRune('▀')
			default:
				builder.WriteRune('█')
			}
		}
		if row+2 < len(matrix) {
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

func addQRQuietZone(matrix [][]bool) [][]bool {
	if len(matrix) == 0 || len(matrix[0]) == 0 {
		return nil
	}
	minX, minY := len(matrix[0]), len(matrix)
	maxX, maxY := -1, -1
	for y, row := range matrix {
		for x, dark := range row {
			if !dark {
				continue
			}
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if maxX < minX || maxY < minY {
		return nil
	}
	// 将库自带的静区收窄为一格, 保持紧凑终端布局.
	const quietZone = 1
	width := maxX - minX + 1 + quietZone*2
	height := maxY - minY + 1 + quietZone*2
	padded := make([][]bool, height)
	for y := range padded {
		padded[y] = make([]bool, width)
	}
	for y := minY; y <= maxY; y++ {
		target := padded[y-minY+quietZone]
		row := matrix[y]
		for x := minX; x <= maxX && x < len(row); x++ {
			target[x-minX+quietZone] = row[x]
		}
	}
	return padded
}

func apiString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
