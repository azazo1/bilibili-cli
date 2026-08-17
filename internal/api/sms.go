package api

import (
	"context"
	"crypto/md5"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultSMSCountryCode = 86
	smsBuild              = "2001100"
	smsChannel            = "master"
	smsMobiApp            = "android_hd"
	smsPlatform           = "android"
	smsLocale             = "zh_CN"
	smsStatistics         = `{"appId":5,"platform":3,"version":"2.0.1","abtest":""}`
)

type WebKey struct {
	Hash string
	Key  string
}

type SMSCodeRequest struct {
	Phone            string
	CountryCode      int
	CaptchaToken     string
	CaptchaValidate  string
	CaptchaSeccode   string
	CaptchaChallenge string
	Captcha          string
}

type SMSCodeResult struct {
	CaptchaKey   string
	RecaptchaURL string
}

type SMSLoginRequest struct {
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

func (c *Client) GetWebKey(ctx context.Context) (WebKey, error) {
	var data map[string]any
	if err := c.RequestPassport(ctx, http.MethodGet, "/x/passport-login/web/key", nil, nil, nil, &data); err != nil {
		return WebKey{}, withAction("获取登录密钥", err)
	}
	key := WebKey{Hash: stringValue(data["hash"]), Key: stringValue(data["key"])}
	if strings.TrimSpace(key.Key) == "" {
		return WebKey{}, NewError(CodeUpstream, "获取登录密钥", "响应缺少公钥")
	}
	return key, nil
}

func (c *Client) SendSMSCode(ctx context.Context, phone string, countryCode int) (SMSCodeResult, error) {
	return c.SendSMSCodeRequest(ctx, SMSCodeRequest{
		Phone:       phone,
		CountryCode: countryCode,
	})
}

func (c *Client) SendSMSCodeRequest(ctx context.Context, request SMSCodeRequest) (SMSCodeResult, error) {
	phone, countryCode, err := normalizeSMSIdentity(request.Phone, request.CountryCode)
	if err != nil {
		return SMSCodeResult{}, err
	}
	buvid, _ := c.passportDevice()
	stamp := time.Now().UnixMilli()
	loginSession := md5Hex([]byte(buvid + strconv.FormatInt(stamp, 10)))
	form := url.Values{
		"build":            []string{smsBuild},
		"buvid":            []string{buvid},
		"c_locale":         []string{smsLocale},
		"channel":          []string{smsChannel},
		"cid":              []string{strconv.Itoa(countryCode)},
		"disable_rcmd":     []string{"0"},
		"local_id":         []string{buvid},
		"login_session_id": []string{loginSession},
		"mobi_app":         []string{smsMobiApp},
		"platform":         []string{smsPlatform},
		"s_locale":         []string{smsLocale},
		"statistics":       []string{smsStatistics},
		"tel":              []string{phone},
	}
	setFormValue(form, "gee_challenge", request.CaptchaChallenge)
	setFormValue(form, "gee_seccode", request.CaptchaSeccode)
	setFormValue(form, "gee_validate", request.CaptchaValidate)
	setFormValue(form, "recaptcha_token", request.CaptchaToken)
	setFormValue(form, "captcha", request.Captcha)
	var data map[string]any
	if err := c.RequestPassportApp(ctx, http.MethodPost, "/x/passport-login/sms/send", nil, form, nil, &data); err != nil {
		if url := recaptchaURLFromError(err); url != "" {
			return SMSCodeResult{RecaptchaURL: url}, nil
		}
		return SMSCodeResult{}, withAction("发送短信验证码", err)
	}
	result := SMSCodeResult{
		CaptchaKey:   stringValue(data["captcha_key"]),
		RecaptchaURL: stringValue(data["recaptcha_url"]),
	}
	if result.CaptchaKey == "" && result.RecaptchaURL == "" {
		return SMSCodeResult{}, NewError(CodeUpstream, "发送短信验证码", "响应缺少 captcha_key")
	}
	return result, nil
}

func (c *Client) LoginBySMS(ctx context.Context, phone, code, captchaKey string, countryCode int) (*Credential, error) {
	return c.LoginBySMSRequest(ctx, SMSLoginRequest{
		Phone:       phone,
		Code:        code,
		CaptchaKey:  captchaKey,
		CountryCode: countryCode,
	})
}

func (c *Client) LoginBySMSRequest(ctx context.Context, request SMSLoginRequest) (*Credential, error) {
	phone, countryCode, err := normalizeSMSIdentity(request.Phone, request.CountryCode)
	if err != nil {
		return nil, err
	}
	code := strings.TrimSpace(request.Code)
	if code == "" {
		return nil, NewError(CodeInvalidInput, "短信登录", "短信验证码不能为空")
	}
	captchaKey := strings.TrimSpace(request.CaptchaKey)
	if captchaKey == "" {
		return nil, NewError(CodeInvalidInput, "短信登录", "captcha_key 不能为空")
	}
	webKey, err := c.GetWebKey(ctx)
	if err != nil {
		return nil, err
	}
	buvid, deviceID := c.passportDevice()
	dt, err := encryptLoginDevice(webKey.Key)
	if err != nil {
		return nil, NewError(CodeUpstream, "短信登录", "登录设备信息加密失败: "+err.Error())
	}
	form := url.Values{
		"bili_local_id":    []string{deviceID},
		"build":            []string{smsBuild},
		"buvid":            []string{buvid},
		"c_locale":         []string{smsLocale},
		"captcha_key":      []string{captchaKey},
		"channel":          []string{smsChannel},
		"cid":              []string{strconv.Itoa(countryCode)},
		"code":              []string{code},
		"device":            []string{"phone"},
		"device_id":         []string{deviceID},
		"device_name":       []string{"vivo"},
		"device_platform":   []string{"Android14vivo"},
		"disable_rcmd":      []string{"0"},
		"dt":                []string{dt},
		"from_pv":           []string{"main.my-information.my-login.0.click"},
		"from_url":          []string{"bilibili://user_center/mine"},
		"local_id":          []string{buvid},
		"mobi_app":          []string{smsMobiApp},
		"platform":          []string{smsPlatform},
		"s_locale":          []string{smsLocale},
		"statistics":        []string{smsStatistics},
		"tel":               []string{phone},
	}
	setFormValue(form, "gee_challenge", request.CaptchaChallenge)
	setFormValue(form, "gee_seccode", request.CaptchaSeccode)
	setFormValue(form, "gee_validate", request.CaptchaValidate)
	setFormValue(form, "recaptcha_token", request.CaptchaToken)
	setFormValue(form, "captcha", request.Captcha)
	var data map[string]any
	if err := c.RequestPassportApp(ctx, http.MethodPost, "/x/passport-login/login/sms", nil, form, nil, &data); err != nil {
		return nil, withAction("短信登录", err)
	}
	credential := credentialFromPassportLoginData(data)
	if credential == nil {
		return nil, NewError(CodeUpstream, "短信登录", "登录响应缺少身份凭证")
	}
	return credential, nil
}

func (c *Client) passportDevice() (string, string) {
	c.passportMu.Lock()
	defer c.passportMu.Unlock()
	if c.passportBuvid == "" {
		c.passportBuvid = generateBuvid()
	}
	if c.passportDeviceID == "" {
		c.passportDeviceID = generateDeviceID()
	}
	return c.passportBuvid, c.passportDeviceID
}

func normalizeSMSIdentity(phone string, countryCode int) (string, int, error) {
	phone = strings.TrimSpace(phone)
	phone = strings.TrimPrefix(phone, "+")
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	if phone == "" {
		return "", 0, NewError(CodeInvalidInput, "短信登录", "手机号不能为空")
	}
	for _, char := range phone {
		if char < '0' || char > '9' {
			return "", 0, NewError(CodeInvalidInput, "短信登录", "手机号只能包含数字")
		}
	}
	if countryCode == 0 {
		countryCode = defaultSMSCountryCode
	}
	if countryCode < 1 || countryCode > 999 {
		return "", 0, NewError(CodeInvalidInput, "短信登录", "国家区号必须在 1 到 999 之间")
	}
	return phone, countryCode, nil
}

func setFormValue(form url.Values, key, value string) {
	if value != "" {
		form.Set(key, value)
	}
}

func recaptchaURLFromError(err error) string {
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.APIStatus != -105 || len(apiErr.Data) == 0 {
		return ""
	}
	var data map[string]any
	if json.Unmarshal(apiErr.Data, &data) != nil {
		return ""
	}
	return firstStringValue(data["recaptcha_url"], data["url"])
}

func md5Hex(data []byte) string {
	digest := md5.Sum(data)
	return hex.EncodeToString(digest[:])
}

func randomToken(length int) string {
	if length <= 0 {
		return ""
	}
	buffer := make([]byte, length)
	if _, err := cryptorand.Read(buffer); err != nil {
		fallback := md5Hex([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
		for len(fallback) < length {
			fallback += md5Hex([]byte(fallback))
		}
		return fallback[:length]
	}
	return hex.EncodeToString(buffer)[:length]
}

func generateBuvid() string {
	digest := md5Hex([]byte(randomToken(32) + strconv.FormatInt(time.Now().UnixNano(), 10)))
	return "XY" + digest[2:3] + digest[12:13] + digest[22:23] + digest
}

func generateDeviceID() string {
	return md5Hex([]byte(randomToken(32) + strconv.FormatInt(time.Now().UnixNano(), 10))) + randomToken(2)
}

func encryptLoginDevice(publicKey string) (string, error) {
	block, _ := pem.Decode([]byte(publicKey))
	if block == nil {
		return "", fmt.Errorf("公钥 PEM 格式无效")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	var rsaKey *rsa.PublicKey
	if err == nil {
		rsaKey, _ = parsed.(*rsa.PublicKey)
	} else if parsedPKCS1, pkcs1Err := x509.ParsePKCS1PublicKey(block.Bytes); pkcs1Err == nil {
		rsaKey = parsedPKCS1
	}
	if rsaKey == nil {
		return "", fmt.Errorf("公钥类型不是 RSA")
	}
	plaintext := []byte(randomToken(16))
	ciphertext, err := rsa.EncryptPKCS1v15(cryptorand.Reader, rsaKey, plaintext)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func credentialFromPassportLoginData(data map[string]any) *Credential {
	token := mapValue(data["token_info"])
	cookieInfo := mapValue(data["cookie_info"])
	credential := &Credential{
		AccessKey:    firstStringValue(token["access_token"], data["access_token"]),
		RefreshToken: firstStringValue(token["refresh_token"], data["refresh_token"], data["token"]),
	}
	for _, item := range listValue(cookieInfo["cookies"]) {
		cookie := mapValue(item)
		name := stringValue(cookie["name"])
		value := stringValue(cookie["value"])
		switch name {
		case "SESSDATA":
			credential.Sessdata = value
		case "bili_jct":
			credential.BiliJct = value
		case "ac_time_value":
			credential.AcTimeValue = value
		case "buvid3":
			credential.Buvid3 = value
		case "buvid4":
			credential.Buvid4 = value
		case "DedeUserID", "dedeuserid":
			credential.DedeUserID = value
		}
	}
	if credential.AcTimeValue == "" {
		credential.AcTimeValue = credential.RefreshToken
	}
	if !credential.ValidForRead() {
		return nil
	}
	return credential
}

func firstStringValue(values ...any) string {
	for _, value := range values {
		if text := stringValue(value); text != "" {
			return text
		}
	}
	return ""
}
