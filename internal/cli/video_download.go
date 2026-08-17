package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/media"
	"github.com/azazo1/bilibili-cli/internal/output"
)

type videoDownloadItem struct {
	label string
	url   string
	path  string
}

func newVideoDownloadCommand(app *App) *cobra.Command {
	var outputDir string
	var audioOnly, videoOnly bool
	command := &cobra.Command{
		Use:   "download BV_OR_URL",
		Short: "下载视频音频和视频",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := output.ModeRich
			if audioOnly && videoOnly {
				return app.invalidInput(cmd, "--audio-only 和 --video-only 不能同时使用", mode)
			}
			bvid, err := app.extractBVID(cmd, args[0], mode)
			if err != nil {
				return err
			}
			ctx := contextOrBackground(cmd.Context())
			credential := app.OptionalCredential(ctx)
			info, fetchErr := app.API.GetVideoInfo(ctx, bvid, credential)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "获取视频信息", mode)
			}
			title := stringValue(info["title"])
			if title == "" {
				title = bvid
			}
			fmt.Fprintf(app.Out.Stdout, "%s (%s)\n", title, formatDuration(info["duration"]))
			fmt.Fprintln(app.Out.Stdout, "获取下载地址...")
			urls, fetchErr := app.API.GetVideoDownloadURLs(ctx, bvid, credential)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "获取下载地址", mode)
			}
			outDir := "."
			if outputDir != "" {
				outDir = expandHome(outputDir)
			}
			items, planErr := makeVideoDownloadItems(urls, title, outDir, audioOnly, videoOnly)
			if planErr != nil {
				return app.Fail(planErr, "准备下载", mode)
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return app.Fail(err, "创建输出目录失败", mode)
			}
			fmt.Fprintf(app.Out.Stdout, "输出目录: %s\n", outDir)
			for _, item := range items {
				fmt.Fprintf(app.Out.Stdout, "下载%s中...\n", item.label)
				bytes, downloadErr := media.DownloadFile(ctx, item.url, item.path, app.Logger)
				if downloadErr != nil {
					return app.Fail(downloadErr, "下载"+item.label, mode)
				}
				fmt.Fprintf(app.Out.Stdout, "%s已保存: %s (%.1f MB)\n", item.label, item.path, float64(bytes)/(1024*1024))
			}
			return nil
		},
	}
	command.Flags().StringVarP(&outputDir, "output", "o", "", "输出目录, 默认当前文件夹")
	command.Flags().BoolVar(&audioOnly, "audio-only", false, "仅下载音频")
	command.Flags().BoolVar(&videoOnly, "video-only", false, "仅下载视频")
	return command
}

func makeVideoDownloadItems(urls api.VideoDownloadURLs, title, outputDir string, audioOnly, videoOnly bool) ([]videoDownloadItem, error) {
	if audioOnly && videoOnly {
		return nil, api.NewError(api.CodeInvalidInput, "", "--audio-only 和 --video-only 不能同时使用")
	}
	safeTitle := sanitizeFileName(title)
	audioPath := filepath.Join(outputDir, safeTitle+".m4a")
	videoPath := filepath.Join(outputDir, safeTitle+".mp4")
	if audioOnly {
		if urls.AudioURL == "" {
			return nil, api.NewError(api.CodeNotFound, "", "无法获取音频流")
		}
		return []videoDownloadItem{{label: "音频", url: urls.AudioURL, path: audioPath}}, nil
	}
	if videoOnly {
		videoURL := urls.VideoURL
		if videoURL == "" {
			videoURL = urls.CombinedURL
		}
		if videoURL == "" {
			return nil, api.NewError(api.CodeNotFound, "", "无法获取视频流")
		}
		return []videoDownloadItem{{label: "视频", url: videoURL, path: videoPath}}, nil
	}
	if urls.AudioURL != "" && urls.VideoURL != "" {
		return []videoDownloadItem{
			{label: "音频", url: urls.AudioURL, path: audioPath},
			{label: "视频", url: urls.VideoURL, path: videoPath},
		}, nil
	}
	if urls.CombinedURL != "" {
		return []videoDownloadItem{{label: "视频", url: urls.CombinedURL, path: videoPath}}, nil
	}
	if urls.AudioURL != "" {
		return []videoDownloadItem{{label: "音频", url: urls.AudioURL, path: audioPath}}, nil
	}
	if urls.VideoURL != "" {
		return []videoDownloadItem{{label: "视频", url: urls.VideoURL, path: videoPath}}, nil
	}
	return nil, api.NewError(api.CodeNotFound, "", "无法获取音视频流")
}

var unsafeFileName = regexp.MustCompile(`[<>:"/\\|?*]`)

func sanitizeFileName(title string) string {
	result := unsafeFileName.ReplaceAllString(title, "_")
	result = strings.Trim(result, ". ")
	runes := []rune(result)
	if len(runes) > 120 {
		result = string(runes[:120])
	}
	if result == "" {
		return "audio"
	}
	return result
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
