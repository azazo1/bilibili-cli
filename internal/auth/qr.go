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
	if err := s.Client.Request(ctx, http.MethodGet, "/x/passport-login/web/qrcode/generate", nil, nil, nil, &generated); err != nil {
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
	requestURL := s.Client.URL("/x/passport-login/web/qrcode/poll") + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", s.Client.UserAgent)
	client := s.Client.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", api.NewError(api.CodeNetwork, "登录", err.Error())
	}
	defer resp.Body.Close()
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
	return strings.TrimRight(code.ToString(false), "\n"), nil
}

func apiString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
