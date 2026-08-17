package api

import (
	"net/http"
	"strings"
)

type Credential struct {
	Sessdata    string  `json:"sessdata"`
	BiliJct     string  `json:"bili_jct"`
	AcTimeValue string  `json:"ac_time_value"`
	Buvid3      string  `json:"buvid3"`
	Buvid4      string  `json:"buvid4"`
	DedeUserID  string  `json:"dedeuserid"`
	AccessKey   string  `json:"access_key,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	SavedAt     float64 `json:"saved_at,omitempty"`
}

func (c *Credential) ValidForRead() bool {
	return c != nil && (strings.TrimSpace(c.Sessdata) != "" || strings.TrimSpace(c.AccessKey) != "")
}

func (c *Credential) ValidForWrite() bool {
	return c != nil && ((strings.TrimSpace(c.Sessdata) != "" && strings.TrimSpace(c.BiliJct) != "") || strings.TrimSpace(c.AccessKey) != "")
}

func (c *Credential) ValidForApp() bool {
	return c != nil && strings.TrimSpace(c.AccessKey) != ""
}

func (c *Credential) CookieHeader() string {
	if c == nil {
		return ""
	}
	parts := make([]string, 0, 6)
	add := func(name, value string) {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, name+"="+value)
		}
	}
	add("SESSDATA", c.Sessdata)
	add("bili_jct", c.BiliJct)
	add("ac_time_value", c.AcTimeValue)
	add("buvid3", c.Buvid3)
	add("buvid4", c.Buvid4)
	add("DedeUserID", c.DedeUserID)
	add("opus-goback", "1")
	return strings.Join(parts, "; ")
}

func (c *Credential) Apply(req *http.Request) {
	if c == nil {
		return
	}
	if cookie := c.CookieHeader(); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if c.BiliJct != "" {
		req.Header.Set("X-CSRFToken", c.BiliJct)
	}
}
