package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/auth"
)

type subtitleCommandItem struct {
	Track      api.SubtitleTrack
	Cues       []api.SubtitleCue
	FetchErr   error
	OutputPath string
}

func newSubtitleCommand(app *App) *cobra.Command {
	var outputPath string
	var asJSON, asYAML bool
	command := &cobra.Command{
		Use:     "subtitle BV_OR_URL",
		Aliases: []string{"st"},
		Short:   "列出并导出视频字幕",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := app.mode(asJSON, asYAML)
			if err != nil {
				return err
			}
			bvid, err := app.extractBVID(args[0], mode)
			if err != nil {
				return err
			}
			ctx := contextOrBackground(cmd.Context())
			credential, _ := app.Auth.GetCredential(ctx, auth.ModeOptional)
			tracks, err := app.API.GetVideoSubtitleTracks(ctx, bvid, credential)
			if err != nil {
				return app.apiFailure(err, "获取字幕列表", mode)
			}
			app.Logger.Info("获取字幕轨道", "bvid", bvid, "count", len(tracks))
			items := make([]subtitleCommandItem, len(tracks))
			warnings := make([]map[string]string, 0)
			for index, track := range tracks {
				app.Logger.Info("下载字幕内容", "bvid", bvid, "index", index+1, "language", track.Language)
				cues, fetchErr := app.API.DownloadSubtitle(ctx, track, credential)
				items[index] = subtitleCommandItem{Track: track, Cues: cues, FetchErr: fetchErr}
				if fetchErr != nil {
					warnings = append(warnings, map[string]string{
						"code":    "subtitle_download_failed",
						"message": fmt.Sprintf("字幕 %d 获取失败: %s", index+1, fetchErr),
					})
					app.Logger.Warn("获取字幕内容失败", "bvid", bvid, "index", index+1, "error", fetchErr)
				}
			}
			if outputPath != "" {
				outputPath = expandHome(outputPath)
				if downloadableSubtitleCount(items) == 0 {
					return app.Fail(api.NewError(api.CodeNotFound, "", "没有可导出的字幕"), "", mode)
				}
				if err := exportSubtitleFiles(outputPath, bvid, items); err != nil {
					return app.Fail(err, "导出字幕文件失败", mode)
				}
			}
			payload := subtitleCommandPayload(bvid, items, warnings, outputPath)
			return app.Complete(payload, mode, func(w io.Writer) {
				renderSubtitleList(w, bvid, items, outputPath)
			})
		},
	}
	command.Flags().StringVarP(&outputPath, "output", "o", "", "字幕输出路径, 目录导出全部字幕, 单条字幕可指定 SRT 文件")
	addStructuredFlags(command, &asJSON, &asYAML)
	return command
}

func exportSubtitleFiles(outputPath, bvid string, items []subtitleCommandItem) error {
	if strings.EqualFold(filepath.Ext(outputPath), ".srt") {
		return exportSingleSubtitleFile(outputPath, items)
	}
	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		return err
	}
	for index := range items {
		if items[index].FetchErr != nil {
			continue
		}
		language := sanitizeFileName(items[index].Track.Language)
		if language == "" {
			language = "subtitle"
		}
		fileName := fmt.Sprintf("%s.%02d.%s.srt", bvid, index+1, language)
		path := filepath.Join(outputPath, fileName)
		if err := os.WriteFile(path, []byte(api.FormatSubtitleSRT(items[index].Cues)), 0o644); err != nil {
			return err
		}
		items[index].OutputPath = path
	}
	return nil
}

func exportSingleSubtitleFile(outputPath string, items []subtitleCommandItem) error {
	index := -1
	for itemIndex, item := range items {
		if item.FetchErr != nil {
			continue
		}
		if index != -1 {
			return api.NewError(api.CodeInvalidInput, "", "多个字幕轨道时 -o 需要目录路径")
		}
		index = itemIndex
	}
	if index == -1 {
		return api.NewError(api.CodeNotFound, "", "没有可导出的字幕")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, []byte(api.FormatSubtitleSRT(items[index].Cues)), 0o644); err != nil {
		return err
	}
	items[index].OutputPath = outputPath
	return nil
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

func subtitleCommandPayload(bvid string, items []subtitleCommandItem, warnings []map[string]string, outputPath string) map[string]any {
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
		"bvid":           bvid,
		"subtitle_count": len(items),
		"subtitles":      subtitles,
		"warnings":       warnings,
	}
	if outputPath != "" {
		payload["output"] = outputPath
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

func renderSubtitleList(w io.Writer, bvid string, items []subtitleCommandItem, outputPath string) {
	if len(items) == 0 {
		fmt.Fprintln(w, "无可用字幕")
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
			track.Language,
			name,
			kind,
			author,
			lineCount,
			status,
		})
	}
	renderTable(w, "字幕列表: "+bvid, []string{"#", "语言", "名称", "类型", "作者", "行数", "状态"}, rows)
	if outputPath == "" {
		return
	}
	fmt.Fprintf(w, "\n已导出 %d 个字幕文件: %s\n", exportedSubtitleCount(items), outputPath)
	for _, item := range items {
		if item.OutputPath != "" {
			fmt.Fprintln(w, item.OutputPath)
		}
	}
}
