package util

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.MustLoadDLL("kernel32.dll")
	procSetStdHandle   = kernel32.MustFindProc("SetStdHandle")
	procGetConsoleMode = kernel32.MustFindProc("GetConsoleMode")
)

func getStdHandle(h int) (fd syscall.Handle) {
	r, _ := syscall.GetStdHandle(h)
	syscall.CloseOnExec(r)
	return r
}

func setStdHandle(stdhandle int32, handle syscall.Handle) error {
	r1, _, err := procSetStdHandle.Call(uintptr(stdhandle), uintptr(handle))
	if r1 == 0 && err != nil {
		return fmt.Errorf("failed to call SetStdHandle: %w", err)
	}
	return nil
}

// IsTty checks if the given fd is a tty
func IsTty(arg any) bool {
	fdsrc, ok := arg.(fder)
	if !ok {
		return false
	}
	fd := fdsrc.Fd()

	var st uint32
	r1, _, err := procGetConsoleMode.Call(fd, uintptr(unsafe.Pointer(&st)))
	return r1 != 0 && err != nil
}

var stdin = os.Stdin

// TtyReady checks if the tty is ready to go
func TtyReady() error {
	var err error

	_stdin, err := os.Open("CONIN$")
	if err != nil {
		return err
	}
	stdin = os.Stdin
	os.Stdin = _stdin
	syscall.Stdin = syscall.Handle(os.Stdin.Fd())

	if err = setStdHandle(syscall.STD_INPUT_HANDLE, syscall.Stdin); err != nil {
		return fmt.Errorf("failed to check for TtyReady: %w", err)
	}
	return nil
}

// TtyTerm restores any state, if necessary
func TtyTerm() {
	os.Stdin = stdin
	syscall.Stdin = syscall.Handle(os.Stdin.Fd())
	setStdHandle(syscall.STD_INPUT_HANDLE, syscall.Stdin)
}
