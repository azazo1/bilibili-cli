package auth

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/azazo1/bilibili-cli/internal/api"
)

type SMSLoginOptions struct {
	Phone            string
	CountryCode      int
	Code             string
	CaptchaKey       string
	CaptchaToken     string
	CaptchaValidate  string
	CaptchaSeccode   string
	CaptchaChallenge string
	Captcha          string
}

func (s *Store) SMSLogin(ctx context.Context, options SMSLoginOptions, in io.Reader, out io.Writer) (*api.Credential, error) {
	s.ensureDefaults()
	if s.Client == nil {
		return nil, api.NewError(api.CodeInternal, "短信登录", "登录客户端未初始化")
	}
	if in == nil {
		in = os.Stdin
	}
	reader := bufio.NewReader(in)
	phone := strings.TrimSpace(options.Phone)
	if phone == "" {
		value, err := readSMSInput(reader, out, "请输入手机号: ")
		if err != nil {
			return nil, api.NewError(api.CodeInvalidInput, "短信登录", err.Error())
		}
		phone = value
	}
	captchaKey := strings.TrimSpace(options.CaptchaKey)
	if captchaKey == "" {
		result, err := s.Client.SendSMSCodeRequest(ctx, api.SMSCodeRequest{
			Phone:            phone,
			CountryCode:      options.CountryCode,
			CaptchaToken:     options.CaptchaToken,
			CaptchaValidate:  options.CaptchaValidate,
			CaptchaSeccode:   options.CaptchaSeccode,
			CaptchaChallenge: options.CaptchaChallenge,
			Captcha:          options.Captcha,
		})
		if err != nil {
			return nil, err
		}
		if result.RecaptchaURL != "" {
			if out != nil {
				fmt.Fprintf(out, "需要完成图形验证, 请打开: %s\n", result.RecaptchaURL)
			}
			return nil, api.NewError(api.CodePermissionDenied, "短信登录", "Bilibili 要求完成图形验证, 请提供验证码参数后重试")
		}
		captchaKey = result.CaptchaKey
		if out != nil {
			fmt.Fprintln(out, "短信验证码已发送, 有效期 5 分钟")
		}
	} else if out != nil {
		fmt.Fprintln(out, "使用已有短信验证码会话")
	}
	code := strings.TrimSpace(options.Code)
	if code == "" {
		value, readErr := readSMSInput(reader, out, "请输入短信验证码: ")
		if readErr != nil {
			return nil, api.NewError(api.CodeInvalidInput, "短信登录", readErr.Error())
		}
		code = value
	}
	credential, err := s.Client.LoginBySMSRequest(ctx, api.SMSLoginRequest{
		Phone:            phone,
		CountryCode:      options.CountryCode,
		Code:             code,
		CaptchaKey:       captchaKey,
		CaptchaToken:     options.CaptchaToken,
		CaptchaValidate:  options.CaptchaValidate,
		CaptchaSeccode:   options.CaptchaSeccode,
		CaptchaChallenge: options.CaptchaChallenge,
		Captcha:          options.Captcha,
	})
	if err != nil {
		return nil, err
	}
	if err := s.Save(credential); err != nil {
		return nil, api.NewError(api.CodeInternal, "短信登录", err.Error())
	}
	if out != nil {
		fmt.Fprintln(out, "登录成功, 凭证已保存")
	}
	return credential, nil
}

func readSMSInput(reader *bufio.Reader, out io.Writer, prompt string) (string, error) {
	if out != nil && prompt != "" {
		fmt.Fprint(out, prompt)
	}
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		if err != io.EOF {
			return "", err
		}
		return "", fmt.Errorf("未读取到输入")
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return "", fmt.Errorf("输入不能为空")
	}
	return value, nil
}
