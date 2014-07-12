// +build !darwin,!windows,!freebsd

package peco

import (
	"syscall"
	"unsafe"
)

// IsTty checks if the given fd is a tty
func IsTty(fd uintptr) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	return err == 0
}

// TtyReady checks if the tty is ready to go
func TtyReady() error {
	return nil
}

// TtyTerm restores any state, if necessary
func TtyTerm() {
}
