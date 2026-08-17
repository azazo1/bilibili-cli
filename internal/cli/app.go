package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/auth"
	"github.com/azazo1/bilibili-cli/internal/output"
)

const Version = "0.6.2"

type App struct {
	API    *api.Client
	Auth   *auth.Store
	Out    *output.Writer
	Logger *slog.Logger
}

func NewApp() *App {
	logger := slog.Default()
	client := api.NewClient()
	store := auth.NewStore(client)
	store.Logger = logger
	return &App{API: client, Auth: store, Out: output.NewWriter(), Logger: logger}
}

type ExitError struct {
	Code     int
	Rendered bool
	Err      error
}

func (e *ExitError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (a *App) ResolveOutput(asJSON, asYAML bool) (output.Mode, error) {
	return a.Out.Resolve(asJSON, asYAML)
}

func (a *App) Fail(err error, action string, mode output.Mode) error {
	if err == nil {
		err = errors.New(action)
	}
	if apiErr, ok := err.(*api.Error); ok && apiErr.Action == "" {
		copy := *apiErr
		copy.Action = action
		err = &copy
	}
	if writeErr := a.Out.EmitError(err, action, mode); writeErr != nil {
		err = fmt.Errorf("%w; output: %v", err, writeErr)
	}
	return &ExitError{Code: 1, Rendered: true, Err: err}
}

func (a *App) Complete(data any, mode output.Mode, render func(io.Writer)) error {
	if mode != output.ModeRich {
		return a.Out.EmitSuccess(data, mode)
	}
	if render != nil {
		render(a.Out.Stdout)
	}
	return nil
}

func (a *App) Execute(ctx context.Context) error {
	root := NewRoot(a)
	return root.ExecuteContext(ctx)
}

func (a *App) RequireCredential(ctx context.Context, write bool, mode output.Mode, message string) (*api.Credential, error) {
	requested := auth.ModeRead
	if write {
		requested = auth.ModeWrite
	}
	credential, err := a.Auth.GetCredential(ctx, requested)
	if err != nil {
		return nil, a.Fail(err, "读取登录凭证失败", mode)
	}
	if credential != nil {
		return credential, nil
	}
	if write {
		if saved, _ := a.Auth.GetCredential(ctx, auth.ModeOptional); saved != nil && saved.BiliJct == "" {
			return nil, a.Fail(api.NewError(api.CodePermissionDenied, "", "当前登录凭证不支持写操作, 缺少 bili_jct. 请执行 bili login 重新登录"), "", mode)
		}
	}
	if message == "" {
		message = "未登录. 使用 bili login 登录"
	}
	return nil, a.Fail(api.NewError(api.CodeNotAuthenticated, "", message), "", mode)
}

func (a *App) setupLogging(verbose bool) {
	level := slog.LevelWarn
	if verbose {
		level = slog.LevelDebug
	}
	options := &slog.HandlerOptions{Level: level}
	a.Logger = slog.New(slog.NewTextHandler(os.Stderr, options))
	slog.SetDefault(a.Logger)
	a.Auth.Logger = a.Logger
	a.API.Logger = a.Logger
}

