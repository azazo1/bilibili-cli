package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/azazo1/bilibili-cli/internal/api"
	"gopkg.in/yaml.v3"
)

type Mode string

const (
	ModeRich Mode = "rich"
	ModeJSON Mode = "json"
	ModeYAML Mode = "yaml"
)

const schemaVersion = "1"

type Writer struct {
	Stdout io.Writer
	Stderr io.Writer
}

func NewWriter() *Writer {
	return &Writer{Stdout: os.Stdout, Stderr: os.Stderr}
}

func (w *Writer) Resolve(asJSON, asYAML bool) (Mode, error) {
	if asJSON && asYAML {
		return ModeRich, api.NewError(api.CodeInvalidInput, "", "不能同时使用 --json 和 --yaml")
	}
	if asJSON {
		return ModeJSON, nil
	}
	if asYAML {
		return ModeYAML, nil
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("OUTPUT"))) {
	case "json":
		return ModeJSON, nil
	case "yaml":
		return ModeYAML, nil
	case "rich":
		return ModeRich, nil
	}
	if !isTTY(w.Stdout) {
		return ModeYAML, nil
	}
	return ModeRich, nil
}

func Success(data any) map[string]any {
	return map[string]any{"ok": true, "schema_version": schemaVersion, "data": data}
}

func ErrorPayload(code api.ErrorCode, message string, details any) map[string]any {
	errorData := map[string]any{"code": string(code), "message": message}
	if details != nil {
		errorData["details"] = details
	}
	return map[string]any{"ok": false, "schema_version": schemaVersion, "error": errorData}
}

func (w *Writer) Emit(data any, mode Mode) error {
	if mode == ModeRich {
		return nil
	}
	var encoded []byte
	var err error
	if mode == ModeJSON {
		encoded, err = json.MarshalIndent(data, "", "  ")
	} else {
		encoded, err = yaml.Marshal(data)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w.Stdout, string(encoded))
	return err
}

func (w *Writer) EmitSuccess(data any, mode Mode) error {
	return w.Emit(Success(data), mode)
}

func (w *Writer) EmitError(err error, action string, mode Mode) error {
	code := api.CodeOf(err)
	message := err.Error()
	if action != "" {
		if apiErr, ok := err.(*api.Error); !ok || apiErr.Action == "" {
			message = action + ": " + message
		}
	}
	if mode != ModeRich {
		return w.Emit(ErrorPayload(code, message, nil), mode)
	}
	_, writeErr := fmt.Fprintln(w.Stderr, message)
	return writeErr
}

func isTTY(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

