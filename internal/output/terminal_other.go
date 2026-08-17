//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd)

package output

import "os"

func terminalWidthForFile(_ *os.File) int {
	return 0
}
