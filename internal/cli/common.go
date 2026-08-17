package cli

import (
	"context"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/model"
	"github.com/azazo1/bilibili-cli/internal/output"
)

func (a *App) mode(command *cobra.Command, asJSON, asYAML bool) (output.Mode, error) {
	mode, err := a.ResolveOutput(asJSON, asYAML)
	if err != nil {
		return mode, a.failUsageWithMode(command, err, "", commandOutputMode(command, a))
	}
	return mode, nil
}

func (a *App) extractBVID(command *cobra.Command, value string, mode output.Mode) (string, error) {
	bvid, err := api.ExtractBVID(value)
	if err != nil {
		return "", a.failUsageWithMode(command, err, "", commandOutputMode(command, a))
	}
	return bvid, nil
}

func (a *App) extractVideoReference(command *cobra.Command, value string, mode output.Mode) (api.VideoReference, error) {
	reference, err := api.ExtractVideoReference(value)
	if err != nil {
		return api.VideoReference{}, a.failUsageWithMode(command, err, "", commandOutputMode(command, a))
	}
	return reference, nil
}

func (a *App) apiFailure(err error, action string, mode output.Mode) error {
	return a.Fail(err, action, mode)
}

func (a *App) renderTable(w io.Writer, title string, headers []string, rows [][]string) {
	if a.Out != nil {
		a.Out.RenderTable(w, title, headers, rows)
		return
	}
	output.RenderTable(w, title, headers, rows, output.TableOptions{})
}

func truncate(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func formatDuration(value any) string { return model.FormatDuration(value) }
func formatCount(value any) string    { return model.FormatCount(value) }

func hasPublishedAt(items []map[string]any) bool {
	for _, item := range items {
		if strings.TrimSpace(stringValue(item["published_at"])) != "" {
			return true
		}
	}
	return false
}

func publishedTime(item map[string]any) string {
	value := strings.TrimSpace(stringValue(item["published_at"]))
	if value == "" {
		return "-"
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.Local().Format("2006-01-02 15:04")
	}
	return value
}

func firstMapList(value any) []map[string]any { return model.Maps(value) }

func mapValue(value any) map[string]any { return model.Map(value) }

func stringValue(value any) string { return model.String(value) }

func intValue(value any, fallback int) int { return model.ToInt(value, fallback) }

func int64Value(value any, fallback int64) int64 { return model.ToInt64(value, fallback) }

func decodeDynamicText(item map[string]any) string {
	parts := make([]string, 0, 4)
	modules := model.Map(item["modules"])
	dynamic := model.Map(modules["module_dynamic"])
	desc := model.Map(dynamic["desc"])
	if value := strings.TrimSpace(model.String(desc["text"])); value != "" {
		parts = append(parts, value)
	}
	card := model.DecodeJSON(item["card"])
	for _, key := range []string{"title", "description", "dynamic", "summary"} {
		if value := strings.TrimSpace(model.String(card[key])); value != "" {
			parts = append(parts, value)
		}
	}
	cardItem := model.Map(card["item"])
	for _, key := range []string{"title", "description", "content"} {
		if value := strings.TrimSpace(model.String(cardItem[key])); value != "" {
			parts = append(parts, value)
		}
	}
	if len(parts) == 0 {
		fallback := model.Map(item["desc"])
		for _, key := range []string{"description", "dynamic_id_str"} {
			if value := strings.TrimSpace(model.String(fallback[key])); value != "" {
				parts = append(parts, value)
			}
		}
	}
	return strings.Join(parts, " ")
}

func dynamicID(item map[string]any) int64 {
	desc := model.Map(item["desc"])
	for _, value := range []any{desc["dynamic_id"], desc["dynamic_id_str"], item["id_str"], item["id"]} {
		if parsed := model.ToInt64(value, 0); parsed != 0 {
			return parsed
		}
	}
	return 0
}

func dynamicTimestamp(item map[string]any) int64 {
	return model.ToInt64(model.Map(item["desc"])["timestamp"], 0)
}

func timestampDisplay(value int64) string {
	if value <= 0 {
		return "-"
	}
	return model.TimestampISO(value)
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

func stripHTML(value string) string { return strings.TrimSpace(htmlTagPattern.ReplaceAllString(value, "")) }
