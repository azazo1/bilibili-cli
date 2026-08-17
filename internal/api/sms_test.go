package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const testSMSPublicKey = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDjb4V7EidX/ym28t2ybo0U6t0n
6p4ej8VjqKHg100va6jkNbNTrLQqMCQCAYtXMXXp2Fwkk6WR+12N9zknLjf+C9sx
/+l48mjUU8RqahiFD1XT/u2e0m2EN029OhCgkHx3Fc/KlFSIbak93EH/XlYis0w+
Xl69GV6klzgxW6d2xQIDAQAB
-----END PUBLIC KEY-----`

func TestSMSLoginUsesSignedPassportAppRequests(t *testing.T) {
	var sendForm, loginForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/passport-login/web/key":
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected web key method: %s", r.Method)
			}
			fmt.Fprintf(w, `{"code":0,"data":{"hash":"hash","key":%q}}`, testSMSPublicKey)
		case "/x/passport-login/sms/send":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected SMS send method: %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			sendForm = r.Form
			fmt.Fprint(w, `{"code":0,"data":{"captcha_key":"captcha-key","recaptcha_url":""}}`)
		case "/x/passport-login/login/sms":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected SMS login method: %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			loginForm = r.Form
			fmt.Fprint(w, `{"code":0,"data":{"token_info":{"access_token":"access-token","refresh_token":"refresh-token"},"cookie_info":{"cookies":[{"name":"SESSDATA","value":"session"},{"name":"bili_jct","value":"csrf"},{"name":"DedeUserID","value":"42"}]}}}`)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient()
	client.PassportBaseURL = server.URL
	client.HTTP = server.Client()
	result, err := client.SendSMSCode(context.Background(), "13800138000", 86)
	if err != nil || result.CaptchaKey != "captcha-key" {
		t.Fatalf("SendSMSCode() = %#v, %v", result, err)
	}
	if sendForm.Get("tel") != "13800138000" || sendForm.Get("cid") != "86" || sendForm.Get("buvid") == "" || sendForm.Get("login_session_id") == "" || sendForm.Get("appkey") == "" || sendForm.Get("sign") == "" {
		t.Fatalf("unexpected SMS send form: %v", sendForm)
	}
	credential, err := client.LoginBySMS(context.Background(), "13800138000", "123456", result.CaptchaKey, 86)
	if err != nil {
		t.Fatal(err)
	}
	if credential.AccessKey != "access-token" || credential.RefreshToken != "refresh-token" || credential.Sessdata != "session" || credential.BiliJct != "csrf" || credential.DedeUserID != "42" {
		t.Fatalf("unexpected SMS credential: %#v", credential)
	}
	if loginForm.Get("tel") != "13800138000" || loginForm.Get("code") != "123456" || loginForm.Get("captcha_key") != "captcha-key" || loginForm.Get("dt") == "" || loginForm.Get("device_id") == "" || loginForm.Get("appkey") == "" || loginForm.Get("sign") == "" {
		t.Fatalf("unexpected SMS login form: %v", loginForm)
	}
	if strings.Contains(loginForm.Get("dt"), "hash") {
		t.Fatalf("dt unexpectedly contains web key hash: %q", loginForm.Get("dt"))
	}
}

func TestSendSMSCodeReportsRecaptchaURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/passport-login/sms/send" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"code":0,"data":{"captcha_key":"","recaptcha_url":"https://example.com/captcha"}}`)
	}))
	defer server.Close()
	client := NewClient()
	client.PassportBaseURL = server.URL
	client.HTTP = server.Client()
	result, err := client.SendSMSCode(context.Background(), "13800138000", 86)
	if err != nil || result.RecaptchaURL != "https://example.com/captcha" {
		t.Fatalf("unexpected recaptcha result: %#v, %v", result, err)
	}
}

func TestSendSMSCodeReportsRecaptchaURLFromChallengeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/passport-login/sms/send" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"code":-105,"message":"captcha required","data":{"url":"https://example.com/challenge"}}`)
	}))
	defer server.Close()
	client := NewClient()
	client.PassportBaseURL = server.URL
	client.HTTP = server.Client()
	result, err := client.SendSMSCode(context.Background(), "13800138000", 86)
	if err != nil || result.RecaptchaURL != "https://example.com/challenge" {
		t.Fatalf("unexpected challenge result: %#v, %v", result, err)
	}
}

func TestSendSMSCodeRejectsInvalidPhone(t *testing.T) {
	client := NewClient()
	if _, err := client.SendSMSCode(context.Background(), "not-a-phone", 86); CodeOf(err) != CodeInvalidInput {
		t.Fatalf("unexpected invalid phone error: %v", err)
	}
}
