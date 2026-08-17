package output

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

type TableOptions struct {
	Interactive bool
	Width       int
	NoTruncate  bool
}

func RenderTable(writer io.Writer, title string, headers []string, rows [][]string, options TableOptions) {
	columnCount := len(headers)
	for _, row := range rows {
		if len(row) > columnCount {
			columnCount = len(row)
		}
	}
	if columnCount == 0 {
		if options.Interactive && title != "" {
			fmt.Fprintln(writer, title)
		}
		return
	}
	headers = normalizeTableRow(headers, columnCount)
	normalizedRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		normalizedRows = append(normalizedRows, normalizeTableRow(row, columnCount))
	}
	if !options.Interactive {
		renderTSV(writer, headers, normalizedRows)
		return
	}
	renderAlignedTable(writer, title, headers, normalizedRows, options)
}

func renderTSV(writer io.Writer, headers []string, rows [][]string) {
	if len(headers) > 0 {
		fmt.Fprintln(writer, strings.Join(headers, "\t"))
	}
	for _, row := range rows {
		fmt.Fprintln(writer, strings.Join(row, "\t"))
	}
}

func renderAlignedTable(writer io.Writer, title string, headers []string, rows [][]string, options TableOptions) {
	widths := tableColumnWidths(headers, rows)
	gap := 2
	if !options.NoTruncate && options.Width > 0 {
		widths, gap = fitTableWidths(headers, widths, options.Width, gap)
		title = truncateDisplay(title, options.Width)
	}
	if title != "" {
		fmt.Fprintln(writer, title)
	}
	writeTableRow(writer, headers, widths, gap)
	writeTableDivider(writer, widths, gap)
	for _, row := range rows {
		writeTableRow(writer, row, widths, gap)
	}
}

func tableColumnWidths(headers []string, rows [][]string) []int {
	widths := make([]int, len(headers))
	for index, header := range headers {
		widths[index] = displayWidth(header)
	}
	for _, row := range rows {
		for index, cell := range row {
			if width := displayWidth(cell); width > widths[index] {
				widths[index] = width
			}
		}
	}
	return widths
}

func fitTableWidths(headers []string, widths []int, limit, gap int) ([]int, int) {
	if tableWidth(widths, gap) <= limit {
		return widths, gap
	}
	minimums := make([]int, len(widths))
	for index, width := range widths {
		minimums[index] = displayWidth(headers[index])
		if isFlexibleColumn(headers[index]) && minimums[index] < 8 {
			minimums[index] = 8
		}
		if minimums[index] > width {
			minimums[index] = width
		}
	}
	shrinkTableWidths(widths, minimums, flexibleColumnIndexes(headers), limit, gap)
	if tableWidth(widths, gap) <= limit {
		return widths, gap
	}
	if gap > 1 {
		gap = 1
	}
	shrinkTableWidths(widths, minimums, allColumnIndexes(widths), limit, gap)
	if tableWidth(widths, gap) <= limit {
		return widths, gap
	}
	for index := range minimums {
		minimums[index] = 1
	}
	shrinkTableWidths(widths, minimums, allColumnIndexes(widths), limit, gap)
	return widths, gap
}

func shrinkTableWidths(widths, minimums, candidates []int, limit, gap int) {
	for tableWidth(widths, gap) > limit {
		selected := -1
		for _, index := range candidates {
			if widths[index] <= minimums[index] {
				continue
			}
			if selected < 0 || widths[index]-minimums[index] > widths[selected]-minimums[selected] {
				selected = index
			}
		}
		if selected < 0 {
			return
		}
		widths[selected]--
	}
}

func flexibleColumnIndexes(headers []string) []int {
	indexes := make([]int, 0, len(headers))
	for index, header := range headers {
		if isFlexibleColumn(header) {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func allColumnIndexes(widths []int) []int {
	indexes := make([]int, len(widths))
	for index := range widths {
		indexes[index] = index
	}
	return indexes
}

func isFlexibleColumn(header string) bool {
	switch header {
	case "标题", "内容", "简介", "签名", "名称", "用户名", "UP主", "作者", "描述", "分区", "地区", "发布时间", "观看时间", "开播时间":
		return true
	default:
		return false
	}
}

func tableWidth(widths []int, gap int) int {
	total := 0
	for _, width := range widths {
		total += width
	}
	if len(widths) > 1 {
		total += gap * (len(widths) - 1)
	}
	return total
}

func writeTableRow(writer io.Writer, row []string, widths []int, gap int) {
	parts := make([]string, len(widths))
	for index, width := range widths {
		cell := truncateDisplay(row[index], width)
		parts[index] = cell + strings.Repeat(" ", max(0, width-displayWidth(cell)))
	}
	fmt.Fprintln(writer, strings.Join(parts, strings.Repeat(" ", gap)))
}

func writeTableDivider(writer io.Writer, widths []int, gap int) {
	parts := make([]string, len(widths))
	for index, width := range widths {
		parts[index] = strings.Repeat("-", width)
	}
	fmt.Fprintln(writer, strings.Join(parts, strings.Repeat(" ", gap)))
}

func normalizeTableRow(row []string, columnCount int) []string {
	result := make([]string, columnCount)
	for index := 0; index < columnCount && index < len(row); index++ {
		result[index] = cleanTableCell(row[index])
	}
	return result
}

func cleanTableCell(value string) string {
	replacer := strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ")
	return replacer.Replace(value)
}

func truncateDisplay(value string, maximum int) string {
	if maximum <= 0 || displayWidth(value) <= maximum {
		return value
	}
	suffix := ""
	available := maximum
	if maximum > 3 {
		suffix = "..."
		available -= displayWidth(suffix)
	}
	var builder strings.Builder
	used := 0
	for _, valueRune := range value {
		width := runeDisplayWidth(valueRune)
		if used+width > available {
			break
		}
		builder.WriteRune(valueRune)
		used += width
	}
	return builder.String() + suffix
}

func displayWidth(value string) int {
	width := 0
	for _, valueRune := range value {
		width += runeDisplayWidth(valueRune)
	}
	return width
}

func runeDisplayWidth(value rune) int {
	if value == 0 || unicode.IsControl(value) || unicode.Is(unicode.Mn, value) || unicode.Is(unicode.Me, value) || unicode.Is(unicode.Cf, value) {
		return 0
	}
	if (value >= 0x1100 && value <= 0x115F) ||
		(value >= 0x2329 && value <= 0x232A) ||
		(value >= 0x2E80 && value <= 0xA4CF) ||
		(value >= 0xAC00 && value <= 0xD7A3) ||
		(value >= 0xF900 && value <= 0xFAFF) ||
		(value >= 0xFE10 && value <= 0xFE19) ||
		(value >= 0xFE30 && value <= 0xFE6F) ||
		(value >= 0xFF00 && value <= 0xFF60) ||
		(value >= 0xFFE0 && value <= 0xFFE6) ||
		(value >= 0x1F300 && value <= 0x1FAFF) ||
		(value >= 0x20000 && value <= 0x3FFFD) {
		return 2
	}
	return 1
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
