package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/azazo1/bilibili-cli/internal/api"
)

type subtitleCommandItem struct {
	Track      api.SubtitleTrack
	Cues       []api.SubtitleCue
	FetchErr   error
	OutputPath string
}

func newSubtitleCommand(app *App) *cobra.Command {
	var outputDir string
	var subtitleIDs, languages []string
	var subtitleType string
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:     "subtitle BV_OR_URL",
		Aliases: []string{"st"},
		Short:   "列出并导出视频字幕",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := app.mode(cmd, asJSON, asYAML)
			if err != nil {
				return err
			}
			subtitleType = strings.ToLower(strings.TrimSpace(subtitleType))
			if subtitleType != "all" && subtitleType != "ai" && subtitleType != "non-ai" {
				return app.invalidInput(cmd, "--type 仅支持 all, ai 或 non-ai", mode)
			}
			reference, err := app.extractVideoReference(cmd, args[0], mode)
			if err != nil {
				return err
			}
			ctx := contextOrBackground(cmd.Context())
			credential := app.OptionalCredential(ctx)
			pages, err := app.API.GetVideoPages(ctx, reference.BVID, credential)
			if err != nil {
				return app.apiFailure(err, "获取视频分P", mode)
			}
			if !reference.PageSpecified && len(pages) > 1 {
				return reportVideoPages(app, reference.BVID, pages, "bili video st", mode)
			}
			if reference.Page > len(pages) {
				return app.Fail(api.NewError(api.CodeNotFound, "", "分P序号超出范围"), "获取字幕列表", mode)
			}
			selectedPage := pages[reference.Page-1]
			tracks, err := app.API.GetVideoSubtitleTracksForPage(ctx, reference.BVID, reference.Page, credential)
			if err != nil {
				return app.apiFailure(err, "获取字幕列表", mode)
			}
			availableTrackCount := len(tracks)
			tracks = filterSubtitleTracks(tracks, subtitleIDs, languages, subtitleType)
			app.Logger.Info("获取字幕轨道", "bvid", reference.BVID, "page", reference.Page, "available_count", availableTrackCount, "selected_count", len(tracks))
			items := make([]subtitleCommandItem, len(tracks))
			warnings := make([]map[string]string, 0)
			for index, track := range tracks {
				app.Logger.Info("下载字幕内容", "bvid", reference.BVID, "page", reference.Page, "index", index+1, "language", track.Language)
				cues, fetchErr := app.API.DownloadSubtitle(ctx, track, credential)
				items[index] = subtitleCommandItem{Track: track, Cues: cues, FetchErr: fetchErr}
				if fetchErr != nil {
					warnings = append(warnings, map[string]string{
						"code":    "subtitle_download_failed",
						"message": fmt.Sprintf("字幕 %d 获取失败: %s", index+1, fetchErr),
					})
					app.Logger.Warn("获取字幕内容失败", "bvid", reference.BVID, "page", reference.Page, "index", index+1, "error", fetchErr)
				}
			}
			if outputDir != "" {
				outputDir = expandHome(outputDir)
				if downloadableSubtitleCount(items) == 0 {
					return app.Fail(api.NewError(api.CodeNotFound, "", "没有可导出的字幕"), "", mode)
				}
				videoTitle := reference.BVID
				if info, infoErr := app.API.GetVideoInfo(ctx, reference.BVID, credential); infoErr == nil {
					if title := stringValue(info["title"]); title != "" {
						videoTitle = title
					}
				} else {
					app.Logger.Warn("获取视频标题失败, 使用 BV 号作为字幕文件名", "bvid", reference.BVID, "error", infoErr)
				}
				fileTitle := videoDownloadFileTitle(videoTitle, reference.Page, len(pages), selectedPage.Title)
				if err := exportSubtitleFiles(outputDir, fileTitle, items); err != nil {
					return app.Fail(err, "导出字幕文件失败", mode)
				}
			}
			payload := subtitleCommandPayload(reference.BVID, reference.Page, selectedPage.Title, availableTrackCount, items, warnings, outputDir)
			return app.CompleteTable(payload, mode, asJSON, asYAML, func(w io.Writer) {
				renderSubtitleList(app, w, reference.BVID, availableTrackCount, items, outputDir)
			})
		},
	}
	command.Flags().StringSliceVar(&subtitleIDs, "id", nil, "按字幕 ID 筛选, 可重复或逗号分隔")
	command.Flags().StringSliceVarP(&languages, "language", "l", nil, "按 API lan 筛选, 可重复或逗号分隔")
	command.Flags().StringVar(&subtitleType, "type", "all", "字幕类型: all, ai 或 non-ai")
	command.Flags().StringVarP(&outputDir, "output", "o", "", "字幕输出目录")
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func filterSubtitleTracks(tracks []api.SubtitleTrack, subtitleIDs, languages []string, subtitleType string) []api.SubtitleTrack {
	ids := make(map[string]struct{}, len(subtitleIDs))
	for _, id := range subtitleIDs {
		if id = strings.TrimSpace(id); id != "" {
			ids[id] = struct{}{}
		}
	}
	selectedLanguages := make(map[string]struct{}, len(languages))
	for _, language := range languages {
		if language = normalizeSubtitleLanguage(language); language != "" {
			selectedLanguages[language] = struct{}{}
		}
	}
	filtered := make([]api.SubtitleTrack, 0, len(tracks))
	for _, track := range tracks {
		if len(ids) > 0 {
			if _, ok := ids[track.ID]; !ok {
				continue
			}
		}
		if len(selectedLanguages) > 0 {
			if _, ok := selectedLanguages[normalizeSubtitleLanguage(track.Language)]; !ok {
				continue
			}
		}
		if subtitleType == "ai" && !track.IsAI() {
			continue
		}
		if subtitleType == "non-ai" && track.IsAI() {
			continue
		}
		filtered = append(filtered, track)
	}
	return filtered
}

func normalizeSubtitleLanguage(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	return strings.ReplaceAll(value, "_", "-")
}

func exportSubtitleFiles(outputDir, videoTitle string, items []subtitleCommandItem) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	videoTitle = sanitizeFileName(videoTitle)
	usedNames := map[string]int{}
	for index := range items {
		if items[index].FetchErr != nil {
			continue
		}
		name := videoTitle + "." + subtitleFileSuffix(items[index].Track)
		key := strings.ToLower(name)
		usedNames[key]++
		if usedNames[key] > 1 {
			name = fmt.Sprintf("%s-%d", name, usedNames[key])
		}
		path := filepath.Join(outputDir, name+".srt")
		if err := os.WriteFile(path, []byte(api.FormatSubtitleSRT(items[index].Cues)), 0o644); err != nil {
			return err
		}
		items[index].OutputPath = path
	}
	return nil
}

func subtitleFileSuffix(track api.SubtitleTrack) string {
	language := strings.TrimSpace(strings.ReplaceAll(track.Language, " ", "_"))
	if language == "" {
		language = "subtitle"
	} else {
		language = sanitizeFileName(language)
	}
	if track.IsAI() {
		return language + "-ai"
	}
	return language
}

func exportedSubtitleCount(items []subtitleCommandItem) int {
	count := 0
	for _, item := range items {
		if item.OutputPath != "" {
			count++
		}
	}
	return count
}

func downloadableSubtitleCount(items []subtitleCommandItem) int {
	count := 0
	for _, item := range items {
		if item.FetchErr == nil {
			count++
		}
	}
	return count
}

func subtitleCommandPayload(bvid string, page int, partTitle string, availableTrackCount int, items []subtitleCommandItem, warnings []map[string]string, outputDir string) map[string]any {
	subtitles := make([]map[string]any, 0, len(items))
	for index, item := range items {
		track := item.Track
		entry := map[string]any{
			"index":         index + 1,
			"id":            track.ID,
			"language":      track.Language,
			"language_name": track.LanguageName,
			"is_ai":         track.IsAI(),
			"type":          track.Type,
			"ai_type":       track.AIType,
			"ai_status":     track.AIStatus,
			"author": map[string]any{
				"id":   track.AuthorID,
				"name": track.AuthorName,
			},
			"url":         track.URL,
			"downloadable": item.FetchErr == nil,
		}
		if item.FetchErr == nil {
			entry["line_count"] = len(item.Cues)
			entry["character_count"] = subtitleCharacterCount(item.Cues)
		} else {
			entry["error"] = item.FetchErr.Error()
		}
		if item.OutputPath != "" {
			entry["output_path"] = item.OutputPath
		}
		subtitles = append(subtitles, entry)
	}
	payload := map[string]any{
		"bvid":                     bvid,
		"page":                     page,
		"part_title":               partTitle,
		"available_subtitle_count": availableTrackCount,
		"subtitle_count":           len(items),
		"subtitles":                subtitles,
		"warnings":                 warnings,
	}
	if outputDir != "" {
		payload["output"] = outputDir
	}
	return payload
}

func subtitleCharacterCount(cues []api.SubtitleCue) int {
	count := 0
	for _, cue := range cues {
		count += len([]rune(cue.Content))
	}
	return count
}

func renderSubtitleList(app *App, w io.Writer, bvid string, availableTrackCount int, items []subtitleCommandItem, outputDir string) {
	if len(items) == 0 {
		if availableTrackCount == 0 {
			fmt.Fprintln(w, "无可用字幕")
		} else {
			fmt.Fprintln(w, "没有匹配的字幕")
		}
		return
	}
	rows := make([][]string, 0, len(items))
	for index, item := range items {
		track := item.Track
		name := track.LanguageName
		if name == "" {
			name = track.Language
		}
		kind := "普通"
		if track.IsAI() {
			kind = "AI"
		}
		author := track.AuthorName
		if author == "" {
			author = "-"
		}
		lineCount := "-"
		status := "可用"
		if item.FetchErr != nil {
			status = "获取失败"
		} else {
			lineCount = fmt.Sprintf("%d", len(item.Cues))
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", index+1),
			track.ID,
			track.Language,
			name,
			kind,
			author,
			lineCount,
			status,
		})
	}
	app.renderTable(w, "字幕列表: "+bvid, []string{"#", "ID", "语言", "名称", "类型", "作者", "行数", "状态"}, rows)
	if outputDir == "" {
		return
	}
	fmt.Fprintf(w, "\n已导出 %d 个字幕文件: %s\n", exportedSubtitleCount(items), outputDir)
	for _, item := range items {
		if item.OutputPath != "" {
			fmt.Fprintln(w, item.OutputPath)
		}
	}
}
