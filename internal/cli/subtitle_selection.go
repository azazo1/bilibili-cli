package cli

import (
	"context"
	"sort"
	"strings"

	"github.com/azazo1/bilibili-cli/internal/api"
)

func preferredSubtitleTracks(tracks []api.SubtitleTrack) []api.SubtitleTrack {
	ordered := append([]api.SubtitleTrack(nil), tracks...)
	sort.SliceStable(ordered, func(left, right int) bool {
		leftLanguage := subtitleLanguageRank(ordered[left])
		rightLanguage := subtitleLanguageRank(ordered[right])
		if leftLanguage != rightLanguage {
			return leftLanguage < rightLanguage
		}
		if ordered[left].IsAI() != ordered[right].IsAI() {
			return !ordered[left].IsAI()
		}
		return false
	})
	return ordered
}

func subtitleLanguageRank(track api.SubtitleTrack) int {
	language := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(track.Language), "_", "-"))
	name := strings.ToLower(strings.TrimSpace(track.LanguageName))
	if strings.HasPrefix(language, "zh") || strings.Contains(name, "中文") || strings.Contains(name, "chinese") {
		return 0
	}
	if strings.HasPrefix(language, "en") || strings.Contains(name, "英文") || strings.Contains(name, "english") {
		return 1
	}
	return 2
}

func (a *App) downloadPreferredSubtitle(ctx context.Context, bvid string, page int, outputDir, videoTitle string, cred *api.Credential) (string, error) {
	tracks, err := a.API.GetVideoSubtitleTracksForPage(ctx, bvid, page, cred)
	if err != nil {
		return "", err
	}
	if len(tracks) == 0 {
		return "", api.NewError(api.CodeNotFound, "自动下载字幕", "视频没有可用字幕")
	}
	for _, track := range preferredSubtitleTracks(tracks) {
		cues, fetchErr := a.API.DownloadSubtitle(ctx, track, cred)
		if fetchErr != nil {
			a.Logger.Warn("候选字幕下载失败", "bvid", bvid, "page", page, "language", track.Language, "is_ai", track.IsAI(), "error", fetchErr)
			continue
		}
		item := subtitleCommandItem{Track: track, Cues: cues}
		if exportErr := exportSubtitleFiles(outputDir, videoTitle, []subtitleCommandItem{item}); exportErr != nil {
			return "", exportErr
		}
		return item.OutputPath, nil
	}
	return "", api.NewError(api.CodeUpstream, "自动下载字幕", "所有候选字幕均下载失败")
}
