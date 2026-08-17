package output

import (
	"io"
	"os"
)

func terminalWidth(writer io.Writer) int {
	file, ok := writer.(*os.File)
	if !ok {
		return 0
	}
	return terminalWidthForFile(file)
}
