package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func (s *Store) QRLogin(ctx context.Context, out io.Writer) (*api.Credential, error) {
	var generated qrGenerateResponse
	if err := s.Client.RequestPassport(ctx, http.MethodGet, "/x/passport-login/web/qrcode/generate", nil, nil, nil, &generated); err != nil {
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
	query := url.Values{"qrcode_key": []string{key}}
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
	switch envelope.Code {
	case 0:
		credential := credentialFromLoginURL(apiString(envelope.Data["url"]))
		if credential == nil {
			return nil, "", api.NewError(api.CodeUpstream, "登录", "登录响应缺少凭证")
		}
		return credential, "done", nil
	case 86038:
		return nil, "timeout", nil
	case 86090:
		return nil, "confirmed", nil
	case 86101:
		return nil, "waiting", nil
	default:
		return nil, "", api.NewError(api.CodeUpstream, "登录", fmt.Sprintf("[%d] %s", envelope.Code, envelope.Message))
	}
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
