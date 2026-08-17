package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

var bvidPattern = regexp.MustCompile(`\bBV[0-9A-Za-z]{10}\b`)

func ExtractBVID(value string) (string, error) {
	if match := bvidPattern.FindString(value); match != "" {
		return match, nil
	}
	return "", NewError(CodeInvalidInput, "", fmt.Sprintf("无法提取 BV 号: %s", value))
}

func (c *Client) GetVideoInfo(ctx context.Context, bvid string, cred *Credential) (map[string]any, error) {
	var data map[string]any
	err := c.request(ctx, http.MethodGet, "/x/web-interface/view", url.Values{"bvid": []string{bvid}}, nil, cred, &data)
	if err != nil {
		return nil, withAction("获取视频信息", err)
	}
	return mapValue(data), nil
}

func (c *Client) getVideoPages(ctx context.Context, bvid string, cred *Credential) ([]map[string]any, error) {
	var data []map[string]any
	err := c.request(ctx, http.MethodGet, "/x/player/pagelist", url.Values{"bvid": []string{bvid}}, nil, cred, &data)
	if err != nil {
		return nil, withAction("获取视频分P信息", err)
	}
	return data, nil
}

func (c *Client) getPlayerInfo(ctx context.Context, bvid string, cid int64, cred *Credential) (map[string]any, error) {
	query := url.Values{"bvid": []string{bvid}, "cid": []string{fmt.Sprintf("%d", cid)}}
	var data map[string]any
	err := c.request(ctx, http.MethodGet, "/x/player/v2", query, nil, cred, &data)
	if err != nil {
		return nil, withAction("获取播放器信息", err)
	}
	return mapValue(data), nil
}

type SubtitleResult struct {
	Text  string
	Items []map[string]any
}

func (c *Client) GetVideoSubtitle(ctx context.Context, bvid string, cred *Credential) (SubtitleResult, error) {
	pages, err := c.getVideoPages(ctx, bvid, cred)
	if err != nil {
		return SubtitleResult{}, err
	}
	if len(pages) == 0 {
		return SubtitleResult{}, nil
	}
	cid := int64Value(pages[0]["cid"], 0)
	if cid == 0 {
		return SubtitleResult{}, nil
	}
	player, err := c.getPlayerInfo(ctx, bvid, cid, cred)
	if err != nil {
		return SubtitleResult{}, err
	}
	subtitle := mapValue(player["subtitle"])
	subtitles := mapList(subtitle["subtitles"])
	if len(subtitles) == 0 {
		return SubtitleResult{}, nil
	}
	selected := ""
	for _, item := range subtitles {
		lan := strings.ToLower(stringValue(item["lan"]))
		if strings.Contains(lan, "zh") {
			selected = stringValue(item["subtitle_url"])
			break
		}
	}
	if selected == "" {
		selected = stringValue(subtitles[0]["subtitle_url"])
	}
	if strings.HasPrefix(selected, "//") {
		selected = "https:" + selected
	}
	if selected == "" {
		return SubtitleResult{}, nil
	}
	raw, err := c.rawJSON(ctx, selected, nil, cred)
	if err != nil {
		return SubtitleResult{}, &Error{Code: CodeNetwork, Action: "下载字幕", Message: err.Error(), Err: err}
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return SubtitleResult{}, &Error{Code: CodeNetwork, Action: "下载字幕", Message: err.Error(), Err: err}
	}
	items := mapList(payload["body"])
	texts := make([]string, 0, len(items))
	for _, item := range items {
		texts = append(texts, stringValue(item["content"]))
	}
	return SubtitleResult{Text: strings.Join(texts, "\n"), Items: items}, nil
}

func (c *Client) GetVideoAIConclusion(ctx context.Context, bvid string, cred *Credential) (map[string]any, error) {
	pages, err := c.getVideoPages(ctx, bvid, cred)
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return map[string]any{}, nil
	}
	cid := int64Value(pages[0]["cid"], 0)
	if cid == 0 {
		return map[string]any{}, nil
	}
	query := url.Values{"bvid": []string{bvid}, "cid": []string{fmt.Sprintf("%d", cid)}}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/web-interface/view/conclusion/get", query, nil, cred, &data); err != nil {
		return nil, withAction("获取 AI 总结", err)
	}
	return mapValue(data), nil
}

func (c *Client) GetVideoComments(ctx context.Context, bvid string, page int, cred *Credential) (map[string]any, error) {
	info, err := c.GetVideoInfo(ctx, bvid, cred)
	if err != nil {
		return nil, err
	}
	aid := int64Value(info["aid"], 0)
	if aid == 0 {
		return nil, NewError(CodeUpstream, "获取视频评论", "视频信息缺少 aid")
	}
	if page < 1 {
		page = 1
	}
	query := url.Values{
		"oid":  []string{fmt.Sprintf("%d", aid)},
		"type": []string{"1"},
		"pn":   []string{fmt.Sprintf("%d", page)},
		"ps":   []string{"20"},
		"sort": []string{"2"},
	}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/v2/reply", query, nil, cred, &data); err != nil {
		return nil, withAction("获取视频评论", err)
	}
	return mapValue(data), nil
}

func (c *Client) GetRelatedVideos(ctx context.Context, bvid string, cred *Credential) ([]map[string]any, error) {
	var data []map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/web-interface/archive/related", url.Values{"bvid": []string{bvid}}, nil, cred, &data); err != nil {
		return nil, withAction("获取相关推荐", err)
	}
	return data, nil
}

func FormatSubtitleTimeline(items []map[string]any, format string) string {
	if len(items) == 0 {
		return ""
	}
	if format == "srt" {
		var builder strings.Builder
		for index, item := range items {
			builder.WriteString(fmt.Sprintf("%d\n", index+1))
			builder.WriteString(formatSRTTime(float64Value(item["from"], 0)))
			builder.WriteString(" --> ")
			builder.WriteString(formatSRTTime(float64Value(item["to"], 0)))
			builder.WriteString("\n")
			builder.WriteString(stringValue(item["content"]))
			builder.WriteString("\n\n")
		}
		return strings.TrimSuffix(builder.String(), "\n")
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("[%s --> %s] %s", formatSubtitleTime(float64Value(item["from"], 0)), formatSubtitleTime(float64Value(item["to"], 0)), stringValue(item["content"])))
	}
	return strings.Join(lines, "\n")
}

func float64Value(value any, fallback float64) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		var parsed float64
		if _, err := fmt.Sscanf(typed, "%f", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func formatSubtitleTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	minutes := int(seconds / 60)
	remaining := seconds - float64(minutes*60)
	return fmt.Sprintf("%02d:%06.3f", minutes, remaining)
}

func formatSRTTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	hours := int(seconds / 3600)
	minutes := int((seconds - float64(hours*3600)) / 60)
	remaining := seconds - float64(hours*3600+minutes*60)
	return strings.Replace(fmt.Sprintf("%02d:%02d:%06.3f", hours, minutes, remaining), ".", ",", 1)
}

