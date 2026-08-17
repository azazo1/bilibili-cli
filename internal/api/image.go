package api

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type ImageKind string

const (
	ImageKindUser    ImageKind = "user"
	ImageKindVideo   ImageKind = "video"
	ImageKindArticle ImageKind = "article"
	ImageKindBangumi ImageKind = "bangumi"
	ImageKindMedia   ImageKind = "media"
	ImageKindLive    ImageKind = "live"
)

type ImageAssetRole string

const (
	ImageAssetCover  ImageAssetRole = "cover"
	ImageAssetAvatar ImageAssetRole = "avatar"
)

type ImageReference struct {
	Kind ImageKind
	ID   string
}

type ImageTarget struct {
	Kind      ImageKind
	ID        string
	Title     string
	ImageURL  string
	ImageRole ImageAssetRole

	avatarURL      string
	avatarUserID   int64
	avatarSeasonID int64
}

var imageBVIDPattern = regexp.MustCompile(`(?i)(?:^|[^0-9a-z])(bv[0-9a-z]{10})(?:$|[^0-9a-z])`)
var imageArticleIDPattern = regexp.MustCompile(`(?i)(?:^|[^0-9a-z])cv([0-9]+)(?:$|[^0-9a-z])`)
var imageSeasonIDPattern = regexp.MustCompile(`(?i)(?:^|[^0-9a-z])ss([0-9]+)(?:$|[^0-9a-z])`)
var imageEpisodeIDPattern = regexp.MustCompile(`(?i)(?:^|[^0-9a-z])ep([0-9]+)(?:$|[^0-9a-z])`)
var imageMediaIDPattern = regexp.MustCompile(`(?i)(?:^|[^0-9a-z])md([0-9]+)(?:$|[^0-9a-z])`)

func (k ImageKind) Valid() bool {
	switch k {
	case ImageKindUser, ImageKindVideo, ImageKindArticle, ImageKindBangumi, ImageKindMedia, ImageKindLive:
		return true
	default:
		return false
	}
}

func (c *Client) ResolveImageReference(ctx context.Context, value string) (ImageReference, error) {
	resolved, isURL, err := c.resolveImageInput(ctx, value)
	if err != nil {
		return ImageReference{}, err
	}
	if isURL {
		if reference, ok := imageReferenceFromURL(resolved); ok {
			return reference, nil
		}
		return ImageReference{}, NewError(CodeInvalidInput, "", "不支持的 Bilibili 图片资源 URL")
	}
	if reference, ok := imageReferenceFromIdentifier(resolved); ok {
		return reference, nil
	}
	if strings.TrimSpace(resolved) == "" {
		return ImageReference{}, NewError(CodeInvalidInput, "", "图片资源引用不能为空")
	}
	return ImageReference{Kind: ImageKindUser, ID: strings.TrimSpace(resolved)}, nil
}

func (c *Client) ResolveImageReferenceAs(ctx context.Context, kind ImageKind, value string) (ImageReference, error) {
	if !kind.Valid() {
		return ImageReference{}, NewError(CodeInvalidInput, "", "不支持的图片资源类型: "+string(kind))
	}
	resolved, isURL, err := c.resolveImageInput(ctx, value)
	if err != nil {
		return ImageReference{}, err
	}
	if isURL {
		reference, ok := imageReferenceFromURL(resolved)
		if !ok || reference.Kind != kind {
			return ImageReference{}, NewError(CodeInvalidInput, "", "URL 与指定的图片资源类型不匹配")
		}
		return reference, nil
	}
	return imageReferenceForKind(kind, resolved)
}

func (c *Client) resolveImageInput(ctx context.Context, value string) (string, bool, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "b23.tv/") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", false, NewError(CodeInvalidInput, "", "图片资源 URL 无法解析")
	}
	if parsed.Scheme == "" && parsed.Host == "" {
		if strings.Contains(value, "://") || strings.HasPrefix(value, "//") {
			return "", false, NewError(CodeInvalidInput, "", "图片资源 URL 无法解析")
		}
		return value, false, nil
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false, NewError(CodeInvalidInput, "", "图片资源 URL 仅支持 HTTP 或 HTTPS")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "b23.tv" || host == "www.b23.tv" {
		return c.resolveB23URL(ctx, parsed.String())
	}
	if !isBilibiliImagePageHost(host) {
		return "", false, NewError(CodeInvalidInput, "", "仅支持 Bilibili 资源页面 URL")
	}
	return parsed.String(), true, nil
}

func (c *Client) resolveB23URL(ctx context.Context, value string) (string, bool, error) {
	finalURL, err := c.resolveB23Target(ctx, value)
	if err != nil {
		return "", false, err
	}
	if !isBilibiliImagePageHost(strings.ToLower(finalURL.Hostname())) {
		return "", false, NewError(CodeInvalidInput, "", "b23 短链未跳转到受支持的 Bilibili 资源页面")
	}
	return finalURL.String(), true, nil
}

func isBilibiliImagePageHost(host string) bool {
	switch strings.ToLower(host) {
	case "bilibili.com", "www.bilibili.com", "m.bilibili.com", "space.bilibili.com", "live.bilibili.com":
		return true
	default:
		return false
	}
}

func imageReferenceFromURL(value string) (ImageReference, bool) {
	parsed, err := url.Parse(value)
	if err != nil || !isBilibiliImagePageHost(strings.ToLower(parsed.Hostname())) {
		return ImageReference{}, false
	}
	pathValue := strings.Trim(parsed.Path, "/")
	parts := strings.Split(pathValue, "/")
	host := strings.ToLower(parsed.Hostname())
	if (host == "space.bilibili.com" && len(parts) > 0) || (len(parts) > 1 && parts[0] == "space") {
		candidate := parts[0]
		if len(parts) > 1 && parts[0] == "space" {
			candidate = parts[1]
		}
		if id, ok := positiveImageNumber(candidate); ok {
			return ImageReference{Kind: ImageKindUser, ID: id}, true
		}
	}
	if host == "live.bilibili.com" && len(parts) > 0 {
		if id, ok := positiveImageNumber(parts[0]); ok {
			return ImageReference{Kind: ImageKindLive, ID: id}, true
		}
	}
	return imageReferenceFromIdentifier(pathValue)
}

func imageReferenceFromIdentifier(value string) (ImageReference, bool) {
	if match := imageBVIDPattern.FindStringSubmatch(value); len(match) == 2 {
		return ImageReference{Kind: ImageKindVideo, ID: normalizeImageBVID(match[1])}, true
	}
	if match := imageArticleIDPattern.FindStringSubmatch(value); len(match) == 2 {
		if id, ok := positiveImageNumber(match[1]); ok {
			return ImageReference{Kind: ImageKindArticle, ID: "cv" + id}, true
		}
	}
	if match := imageSeasonIDPattern.FindStringSubmatch(value); len(match) == 2 {
		if id, ok := positiveImageNumber(match[1]); ok {
			return ImageReference{Kind: ImageKindBangumi, ID: "ss" + id}, true
		}
	}
	if match := imageEpisodeIDPattern.FindStringSubmatch(value); len(match) == 2 {
		if id, ok := positiveImageNumber(match[1]); ok {
			return ImageReference{Kind: ImageKindBangumi, ID: "ep" + id}, true
		}
	}
	if match := imageMediaIDPattern.FindStringSubmatch(value); len(match) == 2 {
		if id, ok := positiveImageNumber(match[1]); ok {
			return ImageReference{Kind: ImageKindMedia, ID: "md" + id}, true
		}
	}
	return ImageReference{}, false
}

func normalizeImageBVID(value string) string {
	if len(value) < 2 {
		return value
	}
	return "BV" + value[2:]
}

func imageReferenceForKind(kind ImageKind, value string) (ImageReference, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return ImageReference{}, NewError(CodeInvalidInput, "", "图片资源引用不能为空")
	}
	switch kind {
	case ImageKindUser:
		return ImageReference{Kind: kind, ID: value}, nil
	case ImageKindVideo:
		if reference, ok := imageReferenceFromIdentifier(value); ok && reference.Kind == ImageKindVideo {
			return reference, nil
		}
		bvid, err := ExtractBVID(value)
		if err != nil {
			return ImageReference{}, err
		}
		return ImageReference{Kind: kind, ID: bvid}, nil
	case ImageKindArticle:
		id, ok := prefixedImageNumber(value, "cv")
		if !ok {
			return ImageReference{}, NewError(CodeInvalidInput, "", "专栏引用必须是 cv 号或正整数 ID")
		}
		return ImageReference{Kind: kind, ID: "cv" + id}, nil
	case ImageKindBangumi:
		if reference, ok := imageReferenceFromIdentifier(value); ok && reference.Kind == ImageKindBangumi {
			return reference, nil
		}
		id, ok := positiveImageNumber(value)
		if !ok {
			return ImageReference{}, NewError(CodeInvalidInput, "", "番剧引用必须是 ss, ep 或正整数 ID")
		}
		return ImageReference{Kind: kind, ID: "ss" + id}, nil
	case ImageKindMedia:
		id, ok := prefixedImageNumber(value, "md")
		if !ok {
			return ImageReference{}, NewError(CodeInvalidInput, "", "影视引用必须是 md 号或正整数 ID")
		}
		return ImageReference{Kind: kind, ID: "md" + id}, nil
	case ImageKindLive:
		id, ok := positiveImageNumber(value)
		if !ok {
			return ImageReference{}, NewError(CodeInvalidInput, "", "直播间引用必须是正整数 ID")
		}
		return ImageReference{Kind: kind, ID: id}, nil
	default:
		return ImageReference{}, NewError(CodeInvalidInput, "", "不支持的图片资源类型: "+string(kind))
	}
}

func prefixedImageNumber(value, prefix string) (string, bool) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), prefix) {
		value = value[len(prefix):]
	}
	return positiveImageNumber(value)
}

func positiveImageNumber(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		return "", false
	}
	return strconv.FormatInt(parsed, 10), true
}

func (c *Client) GetImageTarget(ctx context.Context, reference ImageReference, cred *Credential) (ImageTarget, error) {
	switch reference.Kind {
	case ImageKindUser:
		return c.getUserImageTarget(ctx, reference, cred)
	case ImageKindVideo:
		return c.getVideoImageTarget(ctx, reference, cred)
	case ImageKindArticle:
		return c.getArticleImageTarget(ctx, reference, cred)
	case ImageKindBangumi:
		return c.getBangumiImageTarget(ctx, reference, cred)
	case ImageKindMedia:
		return c.getMediaImageTarget(ctx, reference, cred)
	case ImageKindLive:
		return c.getLiveImageTarget(ctx, reference, cred)
	default:
		return ImageTarget{}, NewError(CodeInvalidInput, "", "不支持的图片资源类型: "+string(reference.Kind))
	}
}

func (c *Client) GetImageAvatar(ctx context.Context, target ImageTarget, cred *Credential) (string, error) {
	if target.Kind == ImageKindUser {
		return target.ImageURL, nil
	}
	if target.avatarURL != "" {
		return target.avatarURL, nil
	}
	if target.avatarUserID > 0 {
		info, err := c.GetUserInfo(ctx, target.avatarUserID, cred)
		if err != nil {
			return "", withAction("获取作者头像", err)
		}
		if avatar := imageURL(info["face"]); avatar != "" {
			return avatar, nil
		}
	}
	if target.avatarSeasonID > 0 {
		seasonTarget, err := c.getBangumiImageTarget(ctx, ImageReference{Kind: ImageKindBangumi, ID: "ss" + strconv.FormatInt(target.avatarSeasonID, 10)}, cred)
		if err != nil {
			return "", withAction("获取作者头像", err)
		}
		if seasonTarget.avatarURL != "" {
			return seasonTarget.avatarURL, nil
		}
	}
	return "", NewError(CodeNotFound, "获取作者头像", "对象没有可下载的作者头像")
}

func (c *Client) getUserImageTarget(ctx context.Context, reference ImageReference, cred *Credential) (ImageTarget, error) {
	uid, ok := positiveImageNumber(reference.ID)
	if !ok {
		return ImageTarget{}, NewError(CodeInvalidInput, "", "用户图片引用必须是正整数 UID")
	}
	info, err := c.GetUserInfo(ctx, mustImageNumber(uid), cred)
	if err != nil {
		return ImageTarget{}, withAction("获取用户图片", err)
	}
	return newImageTarget(ImageKindUser, uid, stringValue(info["name"]), ImageAssetAvatar, info["face"])
}

func (c *Client) getVideoImageTarget(ctx context.Context, reference ImageReference, cred *Credential) (ImageTarget, error) {
	bvid, err := ExtractBVID(reference.ID)
	if err != nil {
		return ImageTarget{}, err
	}
	info, err := c.GetVideoInfo(ctx, bvid, cred)
	if err != nil {
		return ImageTarget{}, withAction("获取视频图片", err)
	}
	target, targetErr := newImageTarget(ImageKindVideo, bvid, stringValue(info["title"]), ImageAssetCover, info["pic"])
	if targetErr != nil {
		return ImageTarget{}, targetErr
	}
	target.avatarURL = imageURL(mapValue(info["owner"])["face"])
	return target, nil
}

func (c *Client) getArticleImageTarget(ctx context.Context, reference ImageReference, cred *Credential) (ImageTarget, error) {
	id, ok := prefixedImageNumber(reference.ID, "cv")
	if !ok {
		return ImageTarget{}, NewError(CodeInvalidInput, "", "专栏图片引用必须是 cv 号")
	}
	var data map[string]any
	query := url.Values{
		"id":       []string{id},
		"mobi_app": []string{"pc"},
		"from":     []string{"web"},
	}
	if err := c.request(ctx, http.MethodGet, "/x/article/viewinfo", query, nil, cred, &data); err != nil {
		return ImageTarget{}, withAction("获取专栏图片", err)
	}
	target, targetErr := newImageTarget(ImageKindArticle, "cv"+id, stringValue(data["title"]), ImageAssetCover, data["banner_url"], data["image_urls"])
	if targetErr != nil {
		return ImageTarget{}, targetErr
	}
	author := mapValue(data["author"])
	target.avatarURL = imageURL(author["face"])
	target.avatarUserID = int64Value(author["mid"], int64Value(data["mid"], 0))
	return target, nil
}

func (c *Client) getBangumiImageTarget(ctx context.Context, reference ImageReference, cred *Credential) (ImageTarget, error) {
	query := make(url.Values)
	id := strings.ToLower(strings.TrimSpace(reference.ID))
	switch {
	case strings.HasPrefix(id, "ss"):
		seasonID, ok := positiveImageNumber(id[2:])
		if !ok {
			return ImageTarget{}, NewError(CodeInvalidInput, "", "番剧引用必须是 ss 或 ep 号")
		}
		query.Set("season_id", seasonID)
	case strings.HasPrefix(id, "ep"):
		episodeID, ok := positiveImageNumber(id[2:])
		if !ok {
			return ImageTarget{}, NewError(CodeInvalidInput, "", "番剧引用必须是 ss 或 ep 号")
		}
		query.Set("ep_id", episodeID)
	default:
		return ImageTarget{}, NewError(CodeInvalidInput, "", "番剧引用必须是 ss 或 ep 号")
	}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/pgc/view/web/season", query, nil, cred, &data); err != nil {
		return ImageTarget{}, withAction("获取番剧图片", err)
	}
	seasonID := int64Value(data["season_id"], 0)
	if seasonID <= 0 {
		return ImageTarget{}, NewError(CodeUpstream, "获取番剧图片", "番剧响应缺少 season_id")
	}
	target, targetErr := newImageTarget(ImageKindBangumi, "ss"+strconv.FormatInt(seasonID, 10), firstImageText(data["title"], data["season_title"]), ImageAssetCover, data["cover"], data["square_cover"])
	if targetErr != nil {
		return ImageTarget{}, targetErr
	}
	target.avatarURL = imageURL(mapValue(data["up_info"])["avatar"])
	return target, nil
}

func (c *Client) getMediaImageTarget(ctx context.Context, reference ImageReference, cred *Credential) (ImageTarget, error) {
	id, ok := prefixedImageNumber(reference.ID, "md")
	if !ok {
		return ImageTarget{}, NewError(CodeInvalidInput, "", "影视图片引用必须是 md 号")
	}
	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/pgc/review/user", url.Values{"media_id": []string{id}}, nil, cred, &data); err != nil {
		return ImageTarget{}, withAction("获取影视图片", err)
	}
	media := mapValue(data["media"])
	target, targetErr := newImageTarget(ImageKindMedia, "md"+id, stringValue(media["title"]), ImageAssetCover, media["cover"], media["horizontal_picture"])
	if targetErr != nil {
		return ImageTarget{}, targetErr
	}
	target.avatarSeasonID = int64Value(media["season_id"], 0)
	return target, nil
}

func (c *Client) getLiveImageTarget(ctx context.Context, reference ImageReference, cred *Credential) (ImageTarget, error) {
	roomID, ok := positiveImageNumber(reference.ID)
	if !ok {
		return ImageTarget{}, NewError(CodeInvalidInput, "", "直播间图片引用必须是正整数 ID")
	}
	var data map[string]any
	if err := c.requestAtBase(ctx, http.MethodGet, c.LiveBaseURL, defaultLiveBaseURL, "/room/v1/Room/get_info", url.Values{"room_id": []string{roomID}}, nil, cred, &data); err != nil {
		return ImageTarget{}, withAction("获取直播间图片", err)
	}
	canonicalRoomID := int64Value(data["room_id"], mustImageNumber(roomID))
	target, targetErr := newImageTarget(ImageKindLive, strconv.FormatInt(canonicalRoomID, 10), stringValue(data["title"]), ImageAssetCover, data["user_cover"], data["keyframe"], data["background"])
	if targetErr != nil {
		return ImageTarget{}, targetErr
	}
	target.avatarUserID = int64Value(data["uid"], 0)
	return target, nil
}

func newImageTarget(kind ImageKind, id, title string, role ImageAssetRole, values ...any) (ImageTarget, error) {
	imageURL := firstImageURL(values...)
	if imageURL == "" {
		return ImageTarget{}, NewError(CodeNotFound, "获取图片信息", "对象没有可下载的主图")
	}
	return ImageTarget{Kind: kind, ID: id, Title: strings.TrimSpace(title), ImageURL: imageURL, ImageRole: role}, nil
}

func firstImageURL(values ...any) string {
	for _, value := range values {
		if result := imageURL(value); result != "" {
			return result
		}
		for _, item := range listValue(value) {
			if result := imageURL(item); result != "" {
				return result
			}
		}
	}
	return ""
}

func imageURL(value any) string {
	raw := strings.TrimSpace(stringValue(value))
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return parsed.String()
}

func firstImageText(values ...any) string {
	for _, value := range values {
		if result := strings.TrimSpace(stringValue(value)); result != "" {
			return result
		}
	}
	return ""
}

func mustImageNumber(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}
