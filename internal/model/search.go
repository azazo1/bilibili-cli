package model

import "strings"

func NormalizeSearchResult(searchType string, item map[string]any) map[string]any {
	switch searchType {
	case "all":
		resultType := String(item["result_type"])
		if resultType != "" && resultType != "all" {
			normalized := NormalizeSearchResult(resultType, item)
			normalized["result_type"] = resultType
			return normalized
		}
		return normalizeSearchFallback(item)
	case "article":
		return NormalizeSearchArticle(item)
	case "video":
		return NormalizeSearchVideo(item)
	case "user", "bili_user":
		return NormalizeSearchUser(item)
	case "bangumi", "media_bangumi", "media", "media_ft":
		return NormalizeSearchMedia(item)
	case "live", "live_room":
		return NormalizeSearchLive(item)
	default:
		return normalizeSearchFallback(item)
	}
}

func normalizeSearchFallback(item map[string]any) map[string]any {
	return map[string]any{
		"id":           firstString(item["id"], item["aid"]),
		"title":        StripHTML(item["title"]),
		"author":       firstString(item["author"], item["uname"]),
		"published_at": searchPublishedAt(item),
	}
}

func NormalizeSearchUser(item map[string]any) map[string]any {
	return map[string]any{
		"id":           String(item["mid"]),
		"uid":          String(item["mid"]),
		"name":         String(item["uname"]),
		"sign":         String(item["usign"]),
		"fans":         ToInt(item["fans"], 0),
		"videos":       ToInt(item["videos"], 0),
		"published_at": searchPublishedAt(item),
	}
}

func NormalizeSearchVideo(item map[string]any) map[string]any {
	duration := String(item["duration"])
	if duration == "" {
		duration = FormatDuration(item["duration"])
	}
	bvid := String(item["bvid"])
	url := firstString(item["arcurl"], item["url"])
	if url == "" && bvid != "" {
		url = "https://www.bilibili.com/video/" + bvid
	}
	return map[string]any{
		"id":           bvid,
		"bvid":         bvid,
		"title":        StripHTML(item["title"]),
		"author":       String(item["author"]),
		"play":         ToInt(item["play"], 0),
		"danmaku":      ToInt(firstValue(item["danmaku"], item["video_review"]), 0),
		"favorite":     ToInt(firstValue(item["favorite"], item["favorites"]), 0),
		"duration":     duration,
		"published_at": searchPublishedAt(item),
		"url":          normalizeSearchURL(url),
	}
}

func NormalizeSearchArticle(item map[string]any) map[string]any {
	id := firstString(item["id"], item["article_id"])
	url := firstString(item["url"], item["arcurl"])
	if url == "" && id != "" {
		url = "https://www.bilibili.com/read/cv" + id
	}
	return map[string]any{
		"id":           id,
		"title":        StripHTML(item["title"]),
		"author":       firstString(item["author"], item["uname"], item["name"]),
		"author_id":    String(item["mid"]),
		"view":         ToInt(item["view"], 0),
		"like":         ToInt(item["like"], 0),
		"reply":        ToInt(item["reply"], 0),
		"published_at": searchPublishedAt(item),
		"url":          normalizeSearchURL(url),
	}
}

func NormalizeSearchMedia(item map[string]any) map[string]any {
	score := Map(item["media_score"])
	return map[string]any{
		"id":           firstString(item["season_id"], item["media_id"]),
		"media_id":     String(item["media_id"]),
		"season_id":    String(item["season_id"]),
		"title":        StripHTML(item["title"]),
		"description":  String(item["desc"]),
		"areas":        String(item["areas"]),
		"styles":       String(item["styles"]),
		"score":        firstString(score["score"], item["score"]),
		"index_show":   String(item["index_show"]),
		"published_at": searchPublishedAt(item),
		"url":          normalizeSearchURL(firstString(item["url"], item["goto_url"])),
	}
}

func NormalizeSearchLive(item map[string]any) map[string]any {
	roomID := firstString(item["roomid"], item["room_id"])
	url := firstString(item["url"], item["jump_url"])
	if url == "" && roomID != "" {
		url = "https://live.bilibili.com/" + roomID
	}
	return map[string]any{
		"id":           roomID,
		"room_id":      roomID,
		"title":        StripHTML(item["title"]),
		"author":       firstString(item["uname"], item["author"]),
		"author_id":    String(item["uid"]),
		"online":       ToInt(item["online"], 0),
		"category":     StripHTML(item["cate_name"]),
		"published_at": searchPublishedAt(item),
		"url":          normalizeSearchURL(url),
	}
}

func searchPublishedAt(item map[string]any) string { return PublishedAt(item) }

func normalizeSearchURL(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "//") {
		return "https:" + value
	}
	if strings.HasPrefix(value, "/") {
		return "https://www.bilibili.com" + value
	}
	return value
}
