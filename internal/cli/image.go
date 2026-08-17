package cli

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/media"
	"github.com/azazo1/bilibili-cli/internal/output"
)

type imageCommandOptions struct {
	outputDir  string
	withAvatar bool
	asJSON     bool
	asYAML     bool
}

type imageDownloadResult struct {
	Role  api.ImageAssetRole
	URL   string
	Path  string
	Bytes int64
}

func newImageCommand(app *App) *cobra.Command {
	options := &imageCommandOptions{}
	command := &cobra.Command{
		Use:   "image REF",
		Short: "下载 Bilibili 封面或头像",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImageCommand(app, cmd, args[0], "", options)
		},
	}
	command.PersistentFlags().StringVarP(&options.outputDir, "output", "o", "", "输出目录, 默认当前文件夹")
	command.PersistentFlags().BoolVar(&options.withAvatar, "with-avatar", false, "同时下载作者或主播头像")
	command.PersistentFlags().BoolVar(&options.asJSON, "json", false, "输出 JSON")
	command.PersistentFlags().BoolVar(&options.asYAML, "yaml", false, "输出 YAML")
	command.AddCommand(
		newTypedImageCommand(app, options, api.ImageKindUser, "user REF", []string{"up"}, "下载用户头像"),
		newTypedImageCommand(app, options, api.ImageKindVideo, "video REF", nil, "下载视频封面"),
		newTypedImageCommand(app, options, api.ImageKindArticle, "article REF", nil, "下载专栏封面"),
		newTypedImageCommand(app, options, api.ImageKindBangumi, "bangumi REF", nil, "下载番剧封面"),
		newTypedImageCommand(app, options, api.ImageKindMedia, "media REF", nil, "下载影视封面"),
		newTypedImageCommand(app, options, api.ImageKindLive, "live REF", nil, "下载直播间封面"),
	)
	return command
}

func newTypedImageCommand(app *App, options *imageCommandOptions, kind api.ImageKind, use string, aliases []string, short string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Aliases: aliases,
		Short:   short,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImageCommand(app, cmd, args[0], kind, options)
		},
	}
}

func runImageCommand(app *App, cmd *cobra.Command, input string, forcedKind api.ImageKind, options *imageCommandOptions) error {
	mode, err := app.mode(cmd, options.asJSON, options.asYAML)
	if err != nil {
		return err
	}
	ctx := contextOrBackground(cmd.Context())
	var reference api.ImageReference
	if forcedKind == "" {
		reference, err = app.API.ResolveImageReference(ctx, input)
	} else {
		reference, err = app.API.ResolveImageReferenceAs(ctx, forcedKind, input)
	}
	if err != nil {
		return app.invalidInput(cmd, err.Error(), mode)
	}
	if reference.Kind == api.ImageKindUser {
		uid, resolveErr := resolveUID(cmd, app, reference.ID, mode)
		if resolveErr != nil {
			return resolveErr
		}
		reference.ID = fmt.Sprintf("%d", uid)
	}
	credential := app.OptionalCredential(ctx)
	target, err := app.API.GetImageTarget(ctx, reference, credential)
	if err != nil {
		return app.apiFailure(err, "获取图片信息失败", mode)
	}
	outDir := "."
	if options.outputDir != "" {
		outDir = expandHome(options.outputDir)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return app.Fail(err, "创建输出目录失败", mode)
	}
	assets := make([]imageDownloadResult, 0, 2)
	primary, err := downloadImageAsset(ctx, app, mode, outDir, target, target.ImageRole, target.ImageURL)
	if err != nil {
		return app.Fail(err, "下载"+imageRoleLabel(target.ImageRole)+"失败", mode)
	}
	assets = append(assets, primary)
	warnings := make([]string, 0, 1)
	if options.withAvatar && target.Kind != api.ImageKindUser {
		avatarURL, avatarErr := app.API.GetImageAvatar(ctx, target, credential)
		if avatarErr != nil {
			warnings = append(warnings, "作者头像未下载: "+avatarErr.Error())
			app.Logger.Warn("作者头像获取失败, 已保留主图", "kind", target.Kind, "id", target.ID, "error", avatarErr)
		} else {
			avatar, downloadErr := downloadImageAsset(ctx, app, mode, outDir, target, api.ImageAssetAvatar, avatarURL)
			if downloadErr != nil {
				warnings = append(warnings, "作者头像未下载: "+downloadErr.Error())
				app.Logger.Warn("作者头像下载失败, 已保留主图", "kind", target.Kind, "id", target.ID, "error", downloadErr)
			} else {
				assets = append(assets, avatar)
			}
		}
	}
	payload := imageDownloadPayload(target, assets, warnings)
	return app.Complete(payload, mode, func(writer io.Writer) {
		if target.Title != "" {
			fmt.Fprintf(writer, "%s: %s\n", imageKindLabel(target.Kind), target.Title)
		}
		for _, asset := range assets {
			fmt.Fprintf(writer, "%s已保存: %s (%s)\n", imageRoleLabel(asset.Role), asset.Path, formatDownloadSize(asset.Bytes))
		}
		for _, warning := range warnings {
			fmt.Fprintf(writer, "警告: %s\n", warning)
		}
	})
}

func downloadImageAsset(ctx context.Context, app *App, mode output.Mode, outputDir string, target api.ImageTarget, role api.ImageAssetRole, sourceURL string) (imageDownloadResult, error) {
	filePath := filepath.Join(outputDir, imageFileName(target.Kind, target.ID, role, sourceURL))
	if mode == output.ModeRich {
		fmt.Fprintf(app.Out.Stdout, "下载%s中...\n", imageRoleLabel(role))
	}
	var progress *downloadProgressBar
	if mode == output.ModeRich {
		progress = newDownloadProgressBar(app.Out.Stdout, imageRoleLabel(role))
	}
	threads := app.Config.Download.Threads
	if threads < 1 {
		threads = media.DefaultDownloadThreads
	}
	var update media.ProgressFunc
	if progress != nil {
		update = progress.Update
	}
	bytes, err := media.DownloadFileWithProgressAndThreads(ctx, sourceURL, filePath, app.Logger, update, threads)
	if progress != nil {
		progress.Finish()
	}
	if err != nil {
		return imageDownloadResult{}, err
	}
	return imageDownloadResult{Role: role, URL: sourceURL, Path: filePath, Bytes: bytes}, nil
}

func imageDownloadPayload(target api.ImageTarget, assets []imageDownloadResult, warnings []string) map[string]any {
	items := make([]map[string]any, 0, len(assets))
	for _, asset := range assets {
		items = append(items, map[string]any{
			"role":  string(asset.Role),
			"url":   asset.URL,
			"path":  asset.Path,
			"bytes": asset.Bytes,
		})
	}
	return map[string]any{
		"kind":     string(target.Kind),
		"id":       target.ID,
		"title":    target.Title,
		"assets":   items,
		"warnings": warnings,
	}
}

func imageFileName(kind api.ImageKind, id string, role api.ImageAssetRole, sourceURL string) string {
	return string(kind) + "-" + id + "-" + string(role) + imageFileExtension(sourceURL)
}

func imageFileExtension(sourceURL string) string {
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return ".jpg"
	}
	base := path.Base(parsed.Path)
	if index := strings.IndexByte(base, '@'); index >= 0 {
		base = base[:index]
	}
	extension := strings.ToLower(path.Ext(base))
	switch extension {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".avif":
		return extension
	default:
		return ".jpg"
	}
}

func imageRoleLabel(role api.ImageAssetRole) string {
	if role == api.ImageAssetAvatar {
		return "头像"
	}
	return "封面"
}

func imageKindLabel(kind api.ImageKind) string {
	switch kind {
	case api.ImageKindUser:
		return "用户"
	case api.ImageKindVideo:
		return "视频"
	case api.ImageKindArticle:
		return "专栏"
	case api.ImageKindBangumi:
		return "番剧"
	case api.ImageKindMedia:
		return "影视"
	case api.ImageKindLive:
		return "直播间"
	default:
		return string(kind)
	}
}
