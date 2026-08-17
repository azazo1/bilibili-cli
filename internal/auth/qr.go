package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/azazo1/bilibili-cli/internal/api"
)

type tvQRGenerateResponse struct {
	URL      string `json:"url"`
	AuthCode string `json:"auth_code"`
}

func (s *Store) QRLogin(ctx context.Context, out io.Writer) (*api.Credential, error) {
	if s.Client == nil {
		return nil, api.NewError(api.CodeInternal, "登录", "登录客户端未初始化")
	}
	var generated tvQRGenerateResponse
	query := url.Values{
		"local_id": []string{"0"},
		"mobi_app": []string{"android_hd"},
		"platform": []string{"android"},
	}
	if err := s.Client.RequestPassportApp(ctx, http.MethodPost, "/x/passport-tv-login/qrcode/auth_code", query, nil, nil, &generated); err != nil {
		return nil, api.NewError(api.CodeNetwork, "登录", err.Error())
	}
	if generated.URL == "" || generated.AuthCode == "" {
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
		credential, state, err := s.pollTVQRCode(ctx, generated.AuthCode)
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

func (s *Store) pollTVQRCode(ctx context.Context, authCode string) (*api.Credential, string, error) {
	query := url.Values{
		"auth_code": []string{authCode},
		"local_id":  []string{"0"},
	}
	var data map[string]any
	err := s.Client.RequestPassportApp(ctx, http.MethodPost, "/x/passport-tv-login/qrcode/poll", query, nil, nil, &data)
	if err != nil {
		var apiErr *api.Error
		if errors.As(err, &apiErr) {
			switch apiErr.APIStatus {
			case 86039:
				return nil, "waiting", nil
			case 86038:
				return nil, "timeout", nil
			case 86090:
				return nil, "confirmed", nil
			}
		}
		return nil, "", api.NewError(api.CodeNetwork, "登录", err.Error())
	}
	credential := credentialFromTVLoginData(data)
	if credential == nil {
		return nil, "", api.NewError(api.CodeUpstream, "登录", "登录响应缺少 App 凭证")
	}
	return credential, "done", nil
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

func credentialFromTVLoginData(data map[string]any) *api.Credential {
	credential := credentialFromCookieInfo(data["cookie_info"])
	if credential == nil {
		return nil
	}
	token, _ := data["token_info"].(map[string]any)
	credential.AccessKey = apiString(token["access_token"])
	credential.RefreshToken = apiString(token["refresh_token"])
	if credential.RefreshToken == "" {
		credential.RefreshToken = apiString(data["refresh_token"])
	}
	credential.AcTimeValue = credential.RefreshToken
	if !credential.ValidForApp() {
		return nil
	}
	return credential
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
