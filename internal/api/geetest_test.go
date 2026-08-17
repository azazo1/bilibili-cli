package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetLoginGeetestChallengeUsesRecaptchaURL(t *testing.T) {
	client := NewClient()
	challenge, err := client.GetLoginGeetestChallenge(context.Background(), "https://example.com/captcha?gee_gt=gt&gee_challenge=challenge&recaptcha_token=token")
	if err != nil || challenge.GT != "gt" || challenge.Challenge != "challenge" || challenge.RecaptchaToken != "token" {
		t.Fatalf("unexpected challenge: %#v, %v", challenge, err)
	}
}

func TestGetLoginGeetestChallengeFallsBackToPassportCaptcha(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/passport-login/captcha" || r.URL.Query().Get("source") != "main_web" {
			t.Fatalf("unexpected captcha request: %s", r.URL.String())
		}
		fmt.Fprint(w, `{"code":0,"data":{"token":"token","geetest":{"gt":"gt","challenge":"challenge"}}}`)
	}))
	defer server.Close()
	client := NewClient()
	client.PassportBaseURL = server.URL
	client.HTTP = server.Client()
	challenge, err := client.GetLoginGeetestChallenge(context.Background(), "")
	if err != nil || challenge.GT != "gt" || challenge.Challenge != "challenge" || challenge.RecaptchaToken != "token" {
		t.Fatalf("unexpected fallback challenge: %#v, %v", challenge, err)
	}
}

func TestGetGeetestConfigAddsChallengeValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gettype.php" || r.URL.Query().Get("gt") != "gt" {
			t.Fatalf("unexpected Geetest config request: %s", r.URL.String())
		}
		fmt.Fprint(w, `({"status":"success","data":{"api_server":"api.geetest.com"}})`)
	}))
	defer server.Close()
	client := NewClient()
	client.GeetestBaseURL = server.URL
	client.HTTP = server.Client()
	config, err := client.GetGeetestConfig(context.Background(), GeetestChallenge{GT: "gt", Challenge: "challenge", RecaptchaToken: "token"})
	if err != nil || stringValue(config["gt"]) != "gt" || stringValue(config["challenge"]) != "challenge" || stringValue(config["product"]) != "bind" || boolValue(config["https"]) != true {
		t.Fatalf("unexpected Geetest config: %#v, %v", config, err)
	}
}
