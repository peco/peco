// +build linux

package peco

import (
	"os"
	"syscall"
	"unsafe"
)

func IsTty(fd uintptr) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	return err == 0
}

func TtyReady() error {
	return nil
}

func TtyTerm() {
}
