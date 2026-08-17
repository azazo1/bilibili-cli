package media

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func MergeStreams(ctx context.Context, audioPath, videoPath, outputPath string, logger *slog.Logger) error {
	if strings.TrimSpace(audioPath) == "" || strings.TrimSpace(videoPath) == "" || strings.TrimSpace(outputPath) == "" {
		return errors.New("合并输入或输出路径为空")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return errors.New("合并音视频需要 ffmpeg, 未找到可执行文件")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	temporaryPath := outputPath + ".part.mp4"
	defer os.Remove(temporaryPath)
	logger.Info("开始合并音视频", "audio", audioPath, "video", videoPath)
	command := exec.CommandContext(ctx, "ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-i", videoPath, "-i", audioPath, "-map", "0:v:0", "-map", "1:a:0", "-c", "copy", temporaryPath)
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("ffmpeg 合并失败: %s", message)
	}
	if err := os.Rename(temporaryPath, outputPath); err != nil {
		return err
	}
	return nil
}
