package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/output"
)

func configureUsageErrors(root *cobra.Command, app *App) {
	root.SetFlagErrorFunc(func(command *cobra.Command, err error) error {
		return app.failUsage(command, err)
	})
	configureArgumentErrors(root, app)
}

func configureArgumentErrors(command *cobra.Command, app *App) {
	if validate := command.Args; validate != nil {
		command.Args = func(cmd *cobra.Command, args []string) error {
			if err := validate(cmd, args); err != nil {
				return app.failUsage(cmd, err)
			}
			return nil
		}
	}
	for _, child := range command.Commands() {
		configureArgumentErrors(child, app)
	}
}

func (a *App) invalidInput(command *cobra.Command, message string, mode output.Mode) error {
	return a.failUsageWithMode(command, api.NewError(api.CodeInvalidInput, "", message), "", commandOutputMode(command, a))
}

func (a *App) failUsage(command *cobra.Command, err error) error {
	if api.CodeOf(err) != api.CodeInvalidInput {
		err = api.NewError(api.CodeInvalidInput, "", err.Error())
	}
	return a.failUsageWithMode(command, err, "", commandOutputMode(command, a))
}

func (a *App) failUsageWithMode(command *cobra.Command, err error, action string, mode output.Mode) error {
	usage := strings.TrimSpace(command.UsageString())
	if mode != output.ModeRich {
		return a.fail(err, action, mode, map[string]any{"usage": usage})
	}
	result := a.Fail(err, action, mode)
	if usage != "" {
		fmt.Fprintln(a.Out.Stderr)
		fmt.Fprintln(a.Out.Stderr, usage)
	}
	return result
}

func commandOutputMode(command *cobra.Command, app *App) output.Mode {
	asJSON, _ := command.Flags().GetBool("json")
	asYAML, _ := command.Flags().GetBool("yaml")
	if !asJSON && !asYAML && app.Out.DefaultMode == "auto" {
		if _, ok := os.LookupEnv("OUTPUT"); !ok {
			return output.ModeRich
		}
	}
	mode, err := app.ResolveOutput(asJSON, asYAML)
	if err != nil {
		return output.ModeRich
	}
	return mode
}
