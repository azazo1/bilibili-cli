package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
	projectName   = "bilibili-cli"
	mainPackage   = "./cmd/bili"
	versionSymbol = "github.com/azazo1/bilibili-cli/internal/cli.Version"
	distDirectory = "dist"
)

var (
	versionTagPattern     = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+)(\.[0-9A-Za-z-]+)*)?(\+([0-9A-Za-z-]+)(\.[0-9A-Za-z-]+)*)?$`)
	artifactVersionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+-]*$`)
)

type target struct {
	GOOS             string
	GOARCH           string
	Platform         string
	Architecture     string
	ArchiveExtension string
	BinaryName       string
}

type buildMetadata struct {
	DisplayVersion string
	ArchiveVersion string
	BinaryPath     string
	ArchivePath    string
}

func main() {
	mode := flag.String("mode", "build", "执行模式: metadata, build 或 dist")
	targetOS := flag.String("target-os", runtime.GOOS, "目标 Go OS")
	targetArch := flag.String("target-arch", runtime.GOARCH, "目标 Go 架构")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	target, err := resolveTarget(*targetOS, *targetArch)
	if err != nil {
		logger.Error("解析发布目标失败", "error", err)
		os.Exit(1)
	}
	metadata, err := resolveMetadata(target)
	if err != nil {
		logger.Error("解析构建版本失败", "error", err)
		os.Exit(1)
	}

	switch *mode {
	case "metadata":
		emitMetadata(metadata)
	case "build":
		if err := buildBinary(metadata, target, logger); err != nil {
			logger.Error("构建 CLI 失败", "error", err)
			os.Exit(1)
		}
	case "dist":
		if err := buildBinary(metadata, target, logger); err != nil {
			logger.Error("构建 CLI 失败", "error", err)
			os.Exit(1)
		}
		if err := createArchive(metadata, target, logger); err != nil {
			logger.Error("打包发布归档失败", "error", err)
			os.Exit(1)
		}
	default:
		logger.Error("不支持的执行模式", "mode", *mode)
		os.Exit(1)
	}
}

func resolveTarget(goos, goarch string) (target, error) {
	target := target{GOOS: goos, GOARCH: goarch}
	switch goos {
	case "linux":
		target.Platform = "linux"
		target.ArchiveExtension = "tar.gz"
	case "windows":
		target.Platform = "windows"
		target.ArchiveExtension = "zip"
	case "darwin":
		target.Platform = "macos"
		target.ArchiveExtension = "tar.gz"
	default:
		return target, fmt.Errorf("不支持的目标系统: %s", goos)
	}

	switch goarch {
	case "amd64":
		target.Architecture = "x86_64"
	case "arm64":
		target.Architecture = "aarch64"
	default:
		return target, fmt.Errorf("不支持的目标架构: %s", goarch)
	}

	target.BinaryName = "bili"
	if goos == "windows" {
		target.BinaryName += ".exe"
	}
	return target, nil
}

func resolveMetadata(target target) (buildMetadata, error) {
	displayVersion, err := resolveDisplayVersion()
	if err != nil {
		return buildMetadata{}, err
	}
	configuredVersion := strings.TrimSpace(os.Getenv("BILI_RELEASE_VERSION"))
	return buildMetadataForVersion(displayVersion, configuredVersion, target)
}

func buildMetadataForVersion(displayVersion, configuredVersion string, target target) (buildMetadata, error) {
	archiveVersion := strings.TrimPrefix(displayVersion, "v")
	archiveVersion = strings.Replace(archiveVersion, "^", "-dirty-", 1)
	if configuredVersion != "" {
		archiveVersion = configuredVersion
	}
	if !artifactVersionPattern.MatchString(archiveVersion) {
		return buildMetadata{}, fmt.Errorf("归档版本格式无效: %s", archiveVersion)
	}

	binaryPath := filepath.Join("bin", target.BinaryName)
	archiveName := fmt.Sprintf(
		"%s-%s-%s-%s.%s",
		projectName,
		archiveVersion,
		target.Platform,
		target.Architecture,
		target.ArchiveExtension,
	)
	return buildMetadata{
		DisplayVersion: displayVersion,
		ArchiveVersion: archiveVersion,
		BinaryPath:     binaryPath,
		ArchivePath:    filepath.Join(distDirectory, archiveName),
	}, nil
}

func resolveDisplayVersion() (string, error) {
	explicitVersion := strings.TrimSpace(os.Getenv("BILI_BUILD_VERSION"))
	if explicitVersion != "" && !versionTagPattern.MatchString(explicitVersion) {
		return "", fmt.Errorf("构建版本必须是 vX.Y.Z tag: %s", explicitVersion)
	}

	commit, commitErr := gitOutput("rev-parse", "--short=7", "HEAD")
	if commitErr != nil || commit == "" {
		commit = "unknown"
	}
	status, statusErr := gitOutput("status", "--porcelain", "--untracked-files=normal")
	dirty := statusErr == nil && status != ""
	exactTag := releaseTag(gitOutput("describe", "--tags", "--exact-match", "--match", "v[0-9]*"))
	nearestTag := releaseTag(gitOutput("describe", "--tags", "--abbrev=0", "--match", "v[0-9]*"))
	return formatBuildVersion(explicitVersion, exactTag, nearestTag, commit, dirty), nil
}

func releaseTag(tag string, err error) string {
	if err != nil || !versionTagPattern.MatchString(tag) {
		return ""
	}
	return tag
}

func formatBuildVersion(explicitVersion, exactTag, nearestTag, commit string, dirty bool) string {
	baseVersion := explicitVersion
	if baseVersion == "" {
		if exactTag != "" {
			baseVersion = exactTag
		} else {
			baseVersion = nearestTag
		}
	}
	if baseVersion == "" {
		baseVersion = "devel"
	}
	if commit == "" {
		commit = "unknown"
	}
	if dirty {
		return baseVersion + "^" + commit
	}
	if explicitVersion != "" || exactTag != "" {
		return baseVersion
	}
	return baseVersion + "-" + commit
}

func gitOutput(arguments ...string) (string, error) {
	command := exec.Command("git", arguments...)
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func emitMetadata(metadata buildMetadata) {
	fmt.Printf("build_version=%s\n", metadata.DisplayVersion)
	fmt.Printf("archive_version=%s\n", metadata.ArchiveVersion)
	fmt.Printf("binary_path=%s\n", metadata.BinaryPath)
	fmt.Printf("archive_path=%s\n", metadata.ArchivePath)
}

func buildBinary(metadata buildMetadata, target target, logger *slog.Logger) error {
	if err := os.MkdirAll(filepath.Dir(metadata.BinaryPath), 0o755); err != nil {
		return fmt.Errorf("创建二进制目录: %w", err)
	}
	logger.Info(
		"构建 CLI",
		"target", target.GOOS+"/"+target.GOARCH,
		"version", metadata.DisplayVersion,
		"output", metadata.BinaryPath,
	)
	command := exec.Command(
		"go",
		"build",
		"-trimpath",
		"-ldflags",
		"-X "+versionSymbol+"="+metadata.DisplayVersion,
		"-o",
		metadata.BinaryPath,
		mainPackage,
	)
	command.Env = buildEnvironment(target)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return err
	}
	return nil
}

func buildEnvironment(target target) []string {
	environment := make([]string, 0, len(os.Environ())+3)
	for _, value := range os.Environ() {
		name, _, found := strings.Cut(value, "=")
		if found && (name == "GOOS" || name == "GOARCH" || name == "CGO_ENABLED") {
			continue
		}
		environment = append(environment, value)
	}
	return append(
		environment,
		"GOOS="+target.GOOS,
		"GOARCH="+target.GOARCH,
		"CGO_ENABLED=0",
	)
}

func createArchive(metadata buildMetadata, target target, logger *slog.Logger) error {
	if err := os.MkdirAll(filepath.Dir(metadata.ArchivePath), 0o755); err != nil {
		return fmt.Errorf("创建归档目录: %w", err)
	}
	source, err := os.Open(metadata.BinaryPath)
	if err != nil {
		return fmt.Errorf("打开二进制文件: %w", err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("读取二进制信息: %w", err)
	}

	archive, err := os.Create(metadata.ArchivePath)
	if err != nil {
		return fmt.Errorf("创建归档文件: %w", err)
	}
	defer archive.Close()

	switch target.ArchiveExtension {
	case "zip":
		err = writeZip(archive, source, info, target.BinaryName)
	case "tar.gz":
		err = writeTarGz(archive, source, info, target.BinaryName)
	default:
		err = errors.New("未知归档格式")
	}
	if err != nil {
		return err
	}
	logger.Info("生成发布归档", "archive", metadata.ArchivePath)
	return nil
}

func writeZip(destination io.Writer, source io.Reader, info os.FileInfo, name string) error {
	writer := zip.NewWriter(destination)
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o755)
	header.SetModTime(info.ModTime())
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("创建 zip 条目: %w", err)
	}
	if _, err := io.Copy(entry, source); err != nil {
		return fmt.Errorf("写入 zip 条目: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("关闭 zip 归档: %w", err)
	}
	return nil
}

func writeTarGz(destination io.Writer, source io.Reader, info os.FileInfo, name string) error {
	gzipWriter := gzip.NewWriter(destination)
	tarWriter := tar.NewWriter(gzipWriter)
	header := &tar.Header{
		Name:    name,
		Mode:    0o755,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("创建 tar 条目: %w", err)
	}
	if _, err := io.Copy(tarWriter, source); err != nil {
		return fmt.Errorf("写入 tar 条目: %w", err)
	}
	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("关闭 tar 归档: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("关闭 gzip 归档: %w", err)
	}
	return nil
}
