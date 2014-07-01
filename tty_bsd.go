// +build darwin freebsd

package peco

import (
	"syscall"
	"unsafe"
)

func IsTty(fd uintptr) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCGETA), uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	return err == 0
}

func TtyReady() error {
	return nil
}

func TtyTerm() {
}
