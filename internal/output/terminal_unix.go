//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package output

import (
	"os"
	"syscall"
	"unsafe"
)

type terminalDimensions struct {
	Rows    uint16
	Columns uint16
	XPixel  uint16
	YPixel  uint16
}

func terminalWidthForFile(file *os.File) int {
	var dimensions terminalDimensions
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&dimensions)))
	if errno != 0 {
		return 0
	}
	return int(dimensions.Columns)
}
