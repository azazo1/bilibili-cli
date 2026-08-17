package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type webIDEntry struct {
	Value  string
	Expiry time.Time
}

var renderDataPattern = regexp.MustCompile(`<script[^>]+id="__RENDER_DATA__"[^>]*>(.*?)</script>`)
var nextDataPattern = regexp.MustCompile(`<script[^>]+id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

func (c *Client) userWebID(ctx context.Context, uid int64, credential *Credential) string {
	c.webIDMu.Lock()
	if entry, ok := c.webIDs[uid]; ok && entry.Value != "" && time.Now().Before(entry.Expiry) {
		c.webIDMu.Unlock()
		return entry.Value
	}
	c.webIDMu.Unlock()

	base := strings.TrimRight(strings.TrimSpace(os.Getenv("BILI_SPACE_BASE_URL")), "/")
	if base == "" {
		base = "https://space.bilibili.com"
	}
	pageURL := fmt.Sprintf("%s/%d/dynamic", base, uid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Referer", "https://space.bilibili.com/")
	if credential != nil {
		credential.Apply(req)
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return ""
	}
	value := findAccessID(string(body))
	if value == "" {
		return ""
	}
	c.webIDMu.Lock()
	if c.webIDs == nil {
		c.webIDs = make(map[int64]webIDEntry)
	}
	c.webIDs[uid] = webIDEntry{Value: value, Expiry: time.Now().Add(30 * time.Minute)}
	c.webIDMu.Unlock()
	return value
}

func findAccessID(body string) string {
	for _, pattern := range []*regexp.Regexp{renderDataPattern, nextDataPattern} {
		match := pattern.FindStringSubmatch(body)
		if len(match) < 2 {
			continue
		}
		content := html.UnescapeString(match[1])
		var value any
		if json.Unmarshal([]byte(content), &value) == nil {
			if accessID := accessIDFromValue(value); accessID != "" {
				return accessID
			}
		}
	}
	return ""
}

func accessIDFromValue(value any) string {
	if mapped, ok := value.(map[string]any); ok {
		if accessID, ok := mapped["access_id"].(string); ok && accessID != "" {
			return accessID
		}
		for _, child := range mapped {
			if accessID := accessIDFromValue(child); accessID != "" {
				return accessID
			}
		}
	}
	if list, ok := value.([]any); ok {
		for _, child := range list {
			if accessID := accessIDFromValue(child); accessID != "" {
				return accessID
			}
		}
	}
	return ""
}

