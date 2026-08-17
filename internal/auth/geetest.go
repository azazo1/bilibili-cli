package auth

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/azazo1/bilibili-cli/internal/api"
)

const geetestVerificationTimeout = 5 * time.Minute

type geetestValidation struct {
	Challenge string `json:"geetest_challenge"`
	Validate  string `json:"geetest_validate"`
	Seccode   string `json:"geetest_seccode"`
}

func (s *Store) completeGeetest(ctx context.Context, recaptchaURL string, out io.Writer) (api.SMSCodeRequest, error) {
	challenge, err := s.Client.GetLoginGeetestChallenge(ctx, recaptchaURL)
	if err != nil {
		return api.SMSCodeRequest{}, err
	}
	config, err := s.Client.GetGeetestConfig(ctx, challenge)
	if err != nil {
		return api.SMSCodeRequest{}, err
	}
	validation, err := waitForGeetestValidation(ctx, config, out)
	if err != nil {
		return api.SMSCodeRequest{}, err
	}
	return api.SMSCodeRequest{
		CaptchaToken:     challenge.RecaptchaToken,
		CaptchaValidate:  validation.Validate,
		CaptchaSeccode:   validation.Seccode,
		CaptchaChallenge: validation.Challenge,
	}, nil
}

func waitForGeetestValidation(ctx context.Context, config map[string]any, out io.Writer) (geetestValidation, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	nonce, err := geetestNonce()
	if err != nil {
		return geetestValidation{}, api.NewError(api.CodeInternal, "人机验证", err.Error())
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return geetestValidation{}, api.NewError(api.CodeNetwork, "人机验证", "无法启动本地验证页面: "+err.Error())
	}
	result := make(chan geetestValidation, 1)
	mux := http.NewServeMux()
	resultPath := "/result?token=" + url.QueryEscape(nonce)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, geetestPage(config, resultPath))
	})
	mux.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Query().Get("token") != nonce {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		validation, decodeErr := decodeGeetestValidation(r.Body)
		if decodeErr != nil {
			http.Error(w, decodeErr.Error(), http.StatusBadRequest)
			return
		}
		select {
		case result <- validation:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(listener)
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	verificationURL := "http://" + listener.Addr().String()
	if out != nil {
		fmt.Fprintln(out, "需要完成图形验证, 请在浏览器打开以下本地地址:")
		fmt.Fprintln(out, verificationURL)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, geetestVerificationTimeout)
	defer cancel()
	select {
	case validation := <-result:
		if out != nil {
			fmt.Fprintln(out, "图形验证已完成, 正在重新请求短信验证码")
		}
		return validation, nil
	case <-timeoutCtx.Done():
		if timeoutCtx.Err() == context.DeadlineExceeded {
			return geetestValidation{}, api.NewError(api.CodeInvalidInput, "人机验证", "验证超时, 请重新登录")
		}
		return geetestValidation{}, timeoutCtx.Err()
	}
}

func geetestNonce() (string, error) {
	buffer := make([]byte, 24)
	if _, err := cryptorand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func decodeGeetestValidation(source io.Reader) (geetestValidation, error) {
	var validation geetestValidation
	decoder := json.NewDecoder(io.LimitReader(source, 64<<10))
	if err := decoder.Decode(&validation); err != nil {
		return geetestValidation{}, fmt.Errorf("验证结果格式错误")
	}
	validation.Challenge = strings.TrimSpace(validation.Challenge)
	validation.Validate = strings.TrimSpace(validation.Validate)
	validation.Seccode = strings.TrimSpace(validation.Seccode)
	if validation.Challenge == "" || validation.Validate == "" || validation.Seccode == "" {
		return geetestValidation{}, fmt.Errorf("验证结果不完整")
	}
	return validation, nil
}

func geetestPage(config map[string]any, resultPath string) string {
	encodedConfig, _ := json.Marshal(config)
	encodedResultPath, _ := json.Marshal(resultPath)
	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>验证</title>
</head>
<body>
  <script src="https://static.geetest.com/static/js/fullpage.0.0.0.js"></script>
  <main id="status">正在加载验证...</main>
  <script>
    const config = %s;
    const resultPath = %s;
    const status = document.getElementById("status");
    try {
      const captcha = Geetest(config)
        .onSuccess(() => {
          const result = captcha.getValidate();
          fetch(resultPath, {
            method: "POST",
            headers: {"Content-Type": "application/json"},
            body: JSON.stringify(result)
          }).then(() => {
            status.textContent = "验证成功, 可以返回终端";
          }).catch(() => {
            status.textContent = "验证结果提交失败";
          });
        })
        .onError(() => {
          status.textContent = "验证加载失败";
        })
        .onClose(() => {
          status.textContent = "验证已关闭";
        });
      captcha.onReady(() => captcha.verify());
    } catch (_) {
      status.textContent = "验证初始化失败";
    }
  </script>
</body>
</html>`, encodedConfig, encodedResultPath)
}
