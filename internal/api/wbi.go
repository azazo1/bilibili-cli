package api

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var wbiPermutation = []int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34, 44, 52,
}

var wbiForbidden = regexp.MustCompile(`[!'()*]`)

func (c *Client) signWBI(ctx context.Context, query url.Values, credential *Credential) (url.Values, error) {
	key, err := c.wbiMixinKey(ctx, credential)
	if err != nil {
		return nil, err
	}
	signed := make(url.Values, len(query)+2)
	for name, values := range query {
		cleaned := make([]string, 0, len(values))
		for _, value := range values {
			cleaned = append(cleaned, wbiForbidden.ReplaceAllString(value, ""))
		}
		signed[name] = cleaned
	}
	if signed.Get("web_location") == "" {
		signed.Set("web_location", "1550101")
	}
	addWBI2Fields(signed)
	signed.Set("wts", strconv.FormatInt(time.Now().Unix(), 10))
	encoded := signed.Encode()
	sum := md5.Sum([]byte(encoded + key))
	signed.Set("w_rid", hex.EncodeToString(sum[:]))
	return signed, nil
}

func addWBI2Fields(values url.Values) {
	values.Set("dm_img_list", "[]")
	values.Set("dm_img_str", randomWBI2Marker())
	values.Set("dm_cover_img_str", randomWBI2Marker())
	values.Set("dm_img_inter", `{"ds":[],"wh":[0,0,0],"of":[0,0,0]}`)
}

func randomWBI2Marker() string {
	const alphabet = "ABCDEFGHIJK"
	bytes := make([]byte, 2)
	if _, err := cryptorand.Read(bytes); err != nil {
		return "AB"
	}
	first := int(bytes[0]) % len(alphabet)
	second := int(bytes[1]) % (len(alphabet) - 1)
	if second >= first {
		second++
	}
	return string([]byte{alphabet[first], alphabet[second]})
}

func (c *Client) wbiMixinKey(ctx context.Context, credential *Credential) (string, error) {
	c.wbiMu.Lock()
	if c.wbiKey != "" && time.Now().Before(c.wbiExpires) {
		key := c.wbiKey
		c.wbiMu.Unlock()
		return key, nil
	}
	c.wbiMu.Unlock()

	raw, err := c.rawJSON(ctx, "/x/web-interface/nav", nil, credential)
	if err != nil {
		return "", withAction("获取 WBI 密钥", err)
	}
	var payload struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", NewError(CodeUpstream, "获取 WBI 密钥", "WBI 密钥响应不是有效 JSON")
	}
	images := mapValue(mapValue(payload.Data)["wbi_img"])
	imageKey := wbiFilename(stringValue(images["img_url"]))
	subKey := wbiFilename(stringValue(images["sub_url"]))
	combined := imageKey + subKey
	if len(combined) < 64 {
		return "", NewError(CodeUpstream, "获取 WBI 密钥", "WBI 密钥响应异常")
	}
	var builder strings.Builder
	builder.Grow(32)
	for _, index := range wbiPermutation {
		if index < len(combined) {
			builder.WriteByte(combined[index])
		}
	}
	permuted := builder.String()
	if len(permuted) < 32 {
		return "", NewError(CodeUpstream, "获取 WBI 密钥", "WBI mixin key 长度异常")
	}
	key := permuted[:32]
	c.wbiMu.Lock()
	c.wbiKey = key
	c.wbiExpires = time.Now().Add(6 * time.Hour)
	c.wbiMu.Unlock()
	return key, nil
}

func wbiFilename(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	file := path.Base(parsed.Path)
	return strings.TrimSuffix(file, path.Ext(file))
}
