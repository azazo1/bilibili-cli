package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type SubtitleTrack struct {
	ID           string
	Language     string
	LanguageName string
	URL          string
	AuthorID     string
	AuthorName   string
	Type         int
	AIType       int
	AIStatus     int
}

func (track SubtitleTrack) IsAI() bool {
	return track.Type == 1
}

type SubtitleCue struct {
	From    float64
	To      float64
	Content string
}

func (c *Client) GetVideoSubtitleTracks(ctx context.Context, bvid string, cred *Credential) ([]SubtitleTrack, error) {
	return c.GetVideoSubtitleTracksForPage(ctx, bvid, 1, cred)
}

func (c *Client) GetVideoSubtitleTracksForPage(ctx context.Context, bvid string, page int, cred *Credential) ([]SubtitleTrack, error) {
	if page < 1 {
		return nil, NewError(CodeInvalidInput, "获取字幕列表", "分P序号必须大于 0")
	}
	pages, err := c.GetVideoPages(ctx, bvid, cred)
	if err != nil {
		if CodeOf(err) == CodeNotFound {
			return []SubtitleTrack{}, nil
		}
		return nil, err
	}
	if page > len(pages) {
		return nil, NewError(CodeNotFound, "获取字幕列表", "分P序号超出范围")
	}
	cid := pages[page-1].CID
	if cid == 0 {
		return []SubtitleTrack{}, nil
	}
	player, err := c.getPlayerInfo(ctx, bvid, cid, cred)
	if err != nil {
		return nil, err
	}
	subtitle := mapValue(player["subtitle"])
	items := mapList(subtitle["subtitles"])
	tracks := make([]SubtitleTrack, 0, len(items))
	for _, item := range items {
		author := mapValue(item["author"])
		languageName := stringValue(item["lan_doc"])
		if languageName == "" {
			languageName = stringValue(item["lan_doc_brief"])
		}
		trackURL := stringValue(item["subtitle_url"])
		if trackURL == "" {
			trackURL = stringValue(item["subtitle_url_v2"])
		}
		if strings.HasPrefix(trackURL, "//") {
			trackURL = "https:" + trackURL
		}
		tracks = append(tracks, SubtitleTrack{
			ID:           stringValue(item["id"]),
			Language:     stringValue(item["lan"]),
			LanguageName: languageName,
			URL:          trackURL,
			AuthorID:     stringValue(author["mid"]),
			AuthorName:   stringValue(author["name"]),
			Type:         intValue(item["type"], 0),
			AIType:       intValue(item["ai_type"], 0),
			AIStatus:     intValue(item["ai_status"], 0),
		})
	}
	return tracks, nil
}

func (c *Client) DownloadSubtitle(ctx context.Context, track SubtitleTrack, cred *Credential) ([]SubtitleCue, error) {
	if track.URL == "" {
		return nil, NewError(CodeUpstream, "下载字幕", "字幕缺少下载地址")
	}
	raw, err := c.rawJSON(ctx, track.URL, nil, cred)
	if err != nil {
		return nil, withAction("下载字幕", err)
	}
	var payload map[string]any
	if err := decodeJSON(raw, &payload); err != nil {
		return nil, &Error{Code: CodeUpstream, Action: "下载字幕", Message: err.Error(), Err: err}
	}
	items := mapList(payload["body"])
	cues := make([]SubtitleCue, 0, len(items))
	for _, item := range items {
		cues = append(cues, SubtitleCue{
			From:    float64Value(item["from"], 0),
			To:      float64Value(item["to"], 0),
			Content: stringValue(item["content"]),
		})
	}
	return cues, nil
}

func FormatSubtitleSRT(cues []SubtitleCue) string {
	if len(cues) == 0 {
		return ""
	}
	var builder strings.Builder
	for index, cue := range cues {
		builder.WriteString(fmt.Sprintf("%d\n", index+1))
		builder.WriteString(formatSRTTime(cue.From))
		builder.WriteString(" --> ")
		builder.WriteString(formatSRTTime(cue.To))
		builder.WriteString("\n")
		builder.WriteString(cue.Content)
		builder.WriteString("\n\n")
	}
	return strings.TrimSuffix(builder.String(), "\n")
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
	case json.Number:
		if parsed, err := typed.Float64(); err == nil {
			return parsed
		}
	case string:
		var parsed float64
		if _, err := fmt.Sscanf(typed, "%f", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
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
