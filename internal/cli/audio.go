package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/auth"
	"github.com/azazo1/bilibili-cli/internal/media"
	"github.com/azazo1/bilibili-cli/internal/output"
)

var unsafeFileName = regexp.MustCompile(`[<>:"/\\|?*]`)

func newAudioCommand(app *App) *cobra.Command {
	var segment int
	var noSplit bool
	var outputDir string
	command := &cobra.Command{
		Use:   "audio BV_OR_URL",
		Short: "下载视频音频并切分为 WAV 片段",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := output.ModeRich
			if segment < 5 || segment > 300 {
				return app.Fail(api.NewError(api.CodeInvalidInput, "", "--segment 范围为 5-300"), "", mode)
			}
			bvid, err := app.extractBVID(args[0], mode)
			if err != nil {
				return err
			}
			ctx := contextOrBackground(cmd.Context())
			credential, _ := app.Auth.GetCredential(ctx, auth.ModeOptional)
			info, fetchErr := app.API.GetVideoInfo(ctx, bvid, credential)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "获取视频信息", mode)
			}
			title := stringValue(info["title"])
			if title == "" {
				title = bvid
			}
			fmt.Fprintf(app.Out.Stdout, "%s (%s)\n", title, formatDuration(info["duration"]))
			fmt.Fprintln(app.Out.Stdout, "获取音频流地址...")
			audioURL, fetchErr := app.API.GetAudioURL(ctx, bvid, credential)
			if fetchErr != nil {
				return app.apiFailure(fetchErr, "获取音频流", mode)
			}
			safeTitle := sanitizeFileName(title)
			outDir := outputDir
			if outDir == "" {
				outDir = filepath.Join(os.TempDir(), "bilibili-cli", safeTitle)
			} else {
				outDir = expandHome(outDir)
			}
			if noSplit {
				outFile := filepath.Join(outDir, safeTitle+".m4a")
				fmt.Fprintln(app.Out.Stdout, "下载音频中...")
				bytes, downloadErr := media.Download(ctx, audioURL, outFile, app.Logger)
				if downloadErr != nil {
					return app.Fail(downloadErr, "下载音频", mode)
				}
				fmt.Fprintf(app.Out.Stdout, "音频已保存: %s (%.1f MB)\n", outFile, float64(bytes)/(1024*1024))
				return nil
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return app.Fail(err, "创建输出目录失败", mode)
			}
			rawPath := filepath.Join(outDir, "_raw.m4s")
			fmt.Fprintln(app.Out.Stdout, "下载音频中...")
			bytes, downloadErr := media.Download(ctx, audioURL, rawPath, app.Logger)
			if downloadErr != nil {
				return app.Fail(downloadErr, "下载音频", mode)
			}
			fmt.Fprintf(app.Out.Stdout, "下载完成 (%.1f MB), 切分中...\n", float64(bytes)/(1024*1024))
			segments, splitErr := media.SplitAudio(ctx, rawPath, outDir, segment, app.Logger)
			_ = os.Remove(rawPath)
			if splitErr != nil {
				return app.Fail(splitErr, "音频切分失败", mode)
			}
			fmt.Fprintf(app.Out.Stdout, "切分完成: %d 段 (每段约 %ds)\n", len(segments), segment)
			fmt.Fprintf(app.Out.Stdout, "输出目录: %s\n", outDir)
			for _, segmentPath := range segments {
				if info, statErr := os.Stat(segmentPath); statErr == nil {
					fmt.Fprintf(app.Out.Stdout, "  %s (%.0f KB)\n", filepath.Base(segmentPath), float64(info.Size())/1024)
				}
			}
			return nil
		},
	}
	command.Flags().IntVarP(&segment, "segment", "s", 25, "每段时长, 单位秒")
	command.Flags().BoolVar(&noSplit, "no-split", false, "直接保存完整音频文件")
	command.Flags().StringVarP(&outputDir, "output", "o", "", "输出目录")
	return command
}

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

