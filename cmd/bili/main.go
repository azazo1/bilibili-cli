package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/azazo1/bilibili-cli/internal/cli"
)

func main() {
	app := cli.NewApp()
	err := app.Execute(context.Background())
	if err == nil {
		return
	}
	var exitErr *cli.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.Code)
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

