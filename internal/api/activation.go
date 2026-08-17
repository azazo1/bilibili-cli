package api

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/bits"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (c *Client) activateDevice(ctx context.Context, device *Credential) error {
	if device == nil || device.Buvid3 == "" || device.Buvid4 == "" {
		return NewError(CodeInvalidInput, "激活匿名设备", "设备标识为空")
	}
	uuid := infocUUID()
	payload := activationPayload(uuid)
	buvidFP := murmurFingerprint(payload, 31)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL("/x/internal/gaia-gateway/ExClimbWuzhi"), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Referer", "https://www.bilibili.com")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", strings.Join([]string{
		"buvid3=" + device.Buvid3,
		"buvid4=" + device.Buvid4,
		"buvid_fp=" + buvidFP,
		"_uuid=" + uuid,
	}, "; "))
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return &Error{Code: CodeNetwork, Message: err.Error(), Err: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &Error{Code: CodeNetwork, Message: err.Error(), Err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &Error{Code: CodeNetwork, HTTPStatus: resp.StatusCode, Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return &Error{Code: CodeUpstream, Message: "设备激活响应不是有效 JSON", Err: err}
	}
	if result.Code != 0 {
		message := result.Message
		if message == "" {
			message = result.Msg
		}
		return &Error{Code: CodeUpstream, APIStatus: result.Code, Message: message}
	}
	return nil
}

func infocUUID() string {
	const alphabet = "123456789ABCDEF"
	part := func(length int) string {
		result := make([]byte, length)
		if _, err := cryptorand.Read(result); err != nil {
			for index := range result {
				result[index] = alphabet[index%len(alphabet)]
			}
			return string(result)
		}
		for index := range result {
			result[index] = alphabet[int(result[index])%len(alphabet)]
		}
		return string(result)
	}
	tail := strconv.FormatInt(time.Now().UnixMilli()%100000, 10)
	for len(tail) < 5 {
		tail += "0"
	}
	return strings.Join([]string{part(8), part(4), part(4), part(4), part(12)}, "-") + tail + "infoc"
}

func activationPayload(uuid string) []byte {
	content := map[string]any{
		"3064": 1,
		"5062": time.Now().UnixMilli(),
		"03bf": "https%3A%2F%2Fwww.bilibili.com%2F",
		"39c8": "333.788.fp.risk",
		"34f1": "",
		"d402": "",
		"654a": "",
		"6e7c": "839x959",
		"3c43": map[string]any{
			"2673": 0,
			"5766": 24,
			"6527": 0,
			"7003": 1,
			"807e": 1,
			"b8ce": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
			"641c": 0,
			"07a4": "en-US",
			"1c57": "not available",
			"0bd0": 8,
			"748e": []int{900, 1440},
			"d61f": []int{875, 1440},
			"fc9d": -480,
			"6aa9": "Asia/Shanghai",
			"75b8": 1,
			"3b21": 1,
			"8a1c": 0,
			"d52f": "not available",
			"adca": "MacIntel",
			"13ab": "0dAAAAAASUVORK5CYII=",
			"bfe9": "QgAAEIQAACEIAABCCQN4FXANGq7S8KTZayAAAAAElFTkSuQmCC",
			"a3c1": []string{"extensions:ANGLE_instanced_arrays;EXT_blend_minmax;EXT_color_buffer_half_float;EXT_float_blend;EXT_frag_depth;EXT_shader_texture_lod;EXT_texture_compression_bptc;EXT_texture_compression_rgtc;EXT_texture_compression_s3tc;EXT_texture_filter_anisotropic;EXT_sRGB;OES_element_index_uint;OES_standard_derivatives;OES_texture_float;OES_texture_float_linear;OES_vertex_array_object"},
			"6bc5": "Apple Inc.~Apple GPU",
			"ed31": 0,
			"72bd": 0,
			"097b": 0,
			"52cd": []int{0, 0, 0},
			"a658": []string{"Arial", "Arial Black", "Courier New", "Helvetica", "Menlo", "Monaco", "Times New Roman"},
			"d02f": "124.04345259929687",
		},
		"54ef": "{\"in_new_ab\":true,\"pageVersion\":\"new_video\"}",
		"8b94": "https%3A%2F%2Fwww.bilibili.com%2F",
		"df35": uuid,
		"07a4": "en-US",
		"5f45": nil,
		"db46": 0,
	}
	inner, _ := json.Marshal(content)
	outer, _ := json.Marshal(map[string]string{"payload": string(inner)})
	return outer
}

func murmurFingerprint(data []byte, seed uint64) string {
	h1, h2 := murmur3X64(data, seed)
	return strconv.FormatUint(h1, 16) + strconv.FormatUint(h2, 16)
}

func murmur3X64(data []byte, seed uint64) (uint64, uint64) {
	const (
		c1 = uint64(0x87c37b91114253d5)
		c2 = uint64(0x4cf5ad432745937f)
	)
	h1, h2 := seed, seed
	totalLength := uint64(len(data))
	for len(data) >= 16 {
		k1 := binary.LittleEndian.Uint64(data[:8])
		k2 := binary.LittleEndian.Uint64(data[8:16])
		k1 *= c1
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= c2
		h1 ^= k1
		h1 = bits.RotateLeft64(h1, 27)
		h1 += h2
		h1 = h1*5 + 0x52dce729
		k2 *= c2
		k2 = bits.RotateLeft64(k2, 33)
		k2 *= c1
		h2 ^= k2
		h2 = bits.RotateLeft64(h2, 31)
		h2 += h1
		h2 = h2*5 + 0x38495ab5
		data = data[16:]
	}
	var k1, k2 uint64
	for index := len(data) - 1; index >= 0; index-- {
		if index >= 8 {
			k2 ^= uint64(data[index]) << uint((index-8)*8)
		} else {
			k1 ^= uint64(data[index]) << uint(index*8)
		}
	}
	if len(data) > 8 {
		k2 *= c2
		k2 = bits.RotateLeft64(k2, 33)
		k2 *= c1
		h2 ^= k2
	}
	if len(data) > 0 {
		k1 *= c1
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= c2
		h1 ^= k1
	}
	h1 ^= totalLength
	h2 ^= totalLength
	h1 += h2
	h2 += h1
	h1 = fmix64(h1)
	h2 = fmix64(h2)
	h1 += h2
	h2 += h1
	return h1, h2
}

func fmix64(value uint64) uint64 {
	value ^= value >> 33
	value *= 0xff51afd7ed558ccd
	value ^= value >> 33
	value *= 0xc4ceb9fe1a85ec53
	value ^= value >> 33
	return value
}
