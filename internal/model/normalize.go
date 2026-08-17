package model

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

func ToInt(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case uint:
		return int(typed)
	case uint64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed)
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
			return parsed
		}
	}
	return fallback
}

func ToInt64(value any, fallback int64) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case string:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func ToFloat(value any, fallback float64) float64 {
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
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func String(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func Map(value any) map[string]any {
	if result, ok := value.(map[string]any); ok {
		return result
	}
	return map[string]any{}
}

func List(value any) []any {
	if result, ok := value.([]any); ok {
		return result
	}
	return []any{}
}

func Maps(value any) []map[string]any {
	if direct, ok := value.([]map[string]any); ok {
		return direct
	}
	items := List(value)
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if mapped, ok := item.(map[string]any); ok {
			result = append(result, mapped)
		}
	}
	return result
}

func StripHTML(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(htmlTagPattern.ReplaceAllString(text, "")))
}

func FormatDuration(value any) string {
	total := ToInt(value, 0)
	if total < 0 {
		total = 0
	}
	if total >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", total/3600, (total%3600)/60, total%60)
	}
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

func FormatCount(value any) string {
	count := ToInt(value, 0)
	if count >= 10000 {
		return fmt.Sprintf("%.1f万", float64(count)/10000)
	}
	return strconv.Itoa(count)
}

func NormalizeUser(info map[string]any) map[string]any {
	return map[string]any{
		"id":       String(info["mid"]),
		"name":     firstString(info["name"], info["uname"]),
		"username": firstString(info["name"], info["uname"]),
		"level":    ToInt(info["level"], 0),
		"coins":    ToInt(info["coins"], 0),
		"sign":     String(info["sign"]),
		"vip":      Map(info["vip"]),
	}
}

func NormalizeRelation(info map[string]any) map[string]any {
	return map[string]any{
		"following": ToInt(info["following"], 0),
		"follower":  ToInt(info["follower"], 0),
	}
}

func NormalizeVideoSummary(value map[string]any) map[string]any {
	owner := Map(value["owner"])
	stat := Map(value["stat"])
	duration := ToInt(value["duration"], ToInt(value["length"], 0))
	if duration == 0 {
		if raw := String(value["length"]); strings.Contains(raw, ":") {
			duration = parseClock(raw)
		}
	}
	bvid := String(value["bvid"])
	identifier := bvid
	if identifier == "" {
		identifier = String(value["aid"])
	}
	url := ""
	if bvid != "" {
		url = "https://www.bilibili.com/video/" + bvid
	}
	return map[string]any{
		"id":                identifier,
		"bvid":              bvid,
		"aid":               ToInt(value["aid"], 0),
		"title":             StripHTML(value["title"]),
		"description":       firstString(value["desc"], value["description"]),
		"duration_seconds":  duration,
		"duration":          FormatDuration(duration),
		"url":               url,
		"owner":             map[string]any{"id": firstString(owner["mid"], owner["id"], value["mid"], value["author_mid"]), "name": firstString(owner["name"], owner["uname"], value["author"])},
		"stats": map[string]any{
			"view":     ToInt(firstValue(stat["view"], value["play"]), 0),
			"danmaku":  ToInt(stat["danmaku"], 0),
			"like":     ToInt(stat["like"], 0),
			"coin":     ToInt(stat["coin"], 0),
			"favorite": ToInt(stat["favorite"], 0),
			"share":    ToInt(stat["share"], 0),
		},
	}
}

func NormalizeSubtitleItems(raw []map[string]any) []map[string]any {
	items := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		items = append(items, map[string]any{
			"from":    ToFloat(item["from"], 0),
			"to":      ToFloat(item["to"], 0),
			"content": String(item["content"]),
		})
	}
	return items
}

func NormalizeComment(item map[string]any) map[string]any {
	member := Map(item["member"])
	content := Map(item["content"])
	identifier := firstString(item["rpid_str"], item["rpid"])
	return map[string]any{
		"id": identifier,
		"author": map[string]any{
			"id":   String(member["mid"]),
			"name": String(member["uname"]),
		},
		"message":     String(content["message"]),
		"like":        ToInt(item["like"], 0),
		"reply_count": ToInt(item["rcount"], 0),
	}
}

func NormalizeSearchUser(item map[string]any) map[string]any {
	return map[string]any{
		"id":     String(item["mid"]),
		"name":   String(item["uname"]),
		"sign":   String(item["usign"]),
		"fans":   ToInt(item["fans"], 0),
		"videos": ToInt(item["videos"], 0),
	}
}

func NormalizeSearchVideo(item map[string]any) map[string]any {
	duration := String(item["duration"])
	if duration == "" {
		duration = FormatDuration(item["duration"])
	}
	return map[string]any{
		"id":       String(item["bvid"]),
		"bvid":     String(item["bvid"]),
		"title":    StripHTML(item["title"]),
		"author":   String(item["author"]),
		"play":     ToInt(item["play"], 0),
		"duration": duration,
	}
}

func NormalizeFavoriteFolder(item map[string]any) map[string]any {
	return map[string]any{
		"id":          ToInt(item["id"], 0),
		"title":       String(item["title"]),
		"media_count": ToInt(item["media_count"], 0),
	}
}

func NormalizeFavoriteMedia(item map[string]any) map[string]any {
	upper := Map(item["upper"])
	duration := ToInt(item["duration"], 0)
	return map[string]any{
		"id":               firstString(item["bvid"], item["id"]),
		"bvid":             String(item["bvid"]),
		"title":            String(item["title"]),
		"duration_seconds": duration,
		"duration":         FormatDuration(duration),
		"upper":            map[string]any{"name": String(upper["name"])},
	}
}

func NormalizeFollowingUser(item map[string]any) map[string]any {
	return map[string]any{"id": String(item["mid"]), "name": String(item["uname"]), "sign": String(item["sign"])}
}

func NormalizeHistoryItem(item map[string]any) map[string]any {
	history := Map(item["history"])
	owner := Map(item["owner"])
	viewedAt := ToInt64(firstValue(history["view_at"], item["view_at"]), 0)
	identifier := firstString(history["bvid"], item["bvid"], history["oid"])
	author := firstString(owner["name"], item["author_name"], item["author"])
	return map[string]any{
		"id":         identifier,
		"bvid":       firstString(history["bvid"], item["bvid"]),
		"title":      firstString(item["title"], item["name"]),
		"author":     author,
		"viewed_at":  timestampISO(viewedAt),
	}
}

func NormalizeWatchLaterItem(item map[string]any) map[string]any {
	owner := Map(item["owner"])
	duration := ToInt(item["duration"], 0)
	return map[string]any{
		"id":               String(item["bvid"]),
		"bvid":             String(item["bvid"]),
		"title":            String(item["title"]),
		"author":           String(owner["name"]),
		"duration_seconds": duration,
		"duration":         FormatDuration(duration),
	}
}

func NormalizeDynamicItem(item map[string]any) map[string]any {
	modules := Map(item["modules"])
	author := Map(modules["module_author"])
	dynamic := Map(modules["module_dynamic"])
	stat := Map(modules["module_stat"])
	desc := Map(dynamic["desc"])
	major := Map(dynamic["major"])
	archive := Map(major["archive"])
	article := Map(major["article"])
	card := DecodeJSON(item["card"])
	descInfo := Map(item["desc"])
	identifier := firstString(descInfo["dynamic_id_str"], descInfo["dynamic_id"], item["id_str"], item["id"])
	text := String(desc["text"])
	if text == "" {
		for _, key := range []string{"dynamic", "description", "summary", "title"} {
			if value := String(card[key]); value != "" {
				text = value
				break
			}
		}
		itemInfo := Map(card["item"])
		if text == "" {
			text = firstString(itemInfo["content"], itemInfo["description"], itemInfo["title"])
		}
	}
	commentInfo := Map(stat["comment"])
	likeInfo := Map(stat["like"])
	ts := ToInt64(descInfo["timestamp"], 0)
	return map[string]any{
		"id": identifier,
		"author": map[string]any{"name": String(author["name"])},
		"published_at":    timestampISO(ts),
		"published_label": String(author["pub_time"]),
		"title":           firstString(archive["title"], article["title"]),
		"text":            text,
		"stats": map[string]any{
			"comment": ToInt(commentInfo["count"], 0),
			"like":    ToInt(likeInfo["count"], 0),
		},
	}
}

func NormalizeVideoCommandPayload(info map[string]any, subtitleText string, subtitleItems []map[string]any, subtitleFormat, aiSummary string, comments, related []map[string]any, warnings []map[string]string) map[string]any {
	normalizedComments := make([]map[string]any, 0, len(comments))
	for _, item := range comments {
		normalizedComments = append(normalizedComments, NormalizeComment(item))
	}
	normalizedRelated := make([]map[string]any, 0, len(related))
	for _, item := range related {
		normalizedRelated = append(normalizedRelated, NormalizeVideoSummary(item))
	}
	return map[string]any{
		"video": NormalizeVideoSummary(info),
		"subtitle": map[string]any{
			"available": len(subtitleText) > 0 || len(subtitleItems) > 0,
			"format":    subtitleFormat,
			"text":      subtitleText,
			"items":     NormalizeSubtitleItems(subtitleItems),
		},
		"ai_summary": aiSummary,
		"comments":   normalizedComments,
		"related":    normalizedRelated,
		"warnings":   warnings,
	}
}

func ActionResult(action string, fields map[string]any) map[string]any {
	result := map[string]any{"success": true, "action": action}
	for key, value := range fields {
		result[key] = value
	}
	return result
}

func DecodeJSON(value any) map[string]any {
	if mapped, ok := value.(map[string]any); ok {
		return mapped
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return map[string]any{}
	}
	return result
}

func TimestampISO(value int64) string { return timestampISO(value) }

func timestampISO(value int64) string {
	if value <= 0 {
		return ""
	}
	return time.Unix(value, 0).Local().Format(time.RFC3339)
}

func firstValue(values ...any) any {
	for _, value := range values {
		if value == nil {
			continue
		}
		if text, ok := value.(string); ok && text == "" {
			continue
		}
		return value
	}
	return nil
}

func firstString(values ...any) string { return String(firstValue(values...)) }

func parseClock(value string) int {
	parts := strings.Split(value, ":")
	if len(parts) == 0 {
		return 0
	}
	result := 0
	for _, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil {
			return 0
		}
		result = result*60 + number
	}
	return result
}
