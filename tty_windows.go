package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.MustLoadDLL("kernel32.dll")
	procSetStdHandle = kernel32.MustFindProc("SetStdHandle")
)

func getStdHandle(h int) (fd syscall.Handle) {
	r, _ := syscall.GetStdHandle(h)
	syscall.CloseOnExec(r)
	return r
}

func setStdHandle(stdhandle int32, handle syscall.Handle) error {
	r0, _, e1 := syscall.Syscall(procSetStdHandle.Addr(), 2, uintptr(stdhandle), uintptr(handle), 0)
	if r0 == 0 {
		if e1 != 0 {
			return error(e1)
		}
		return syscall.EINVAL
	}
	return nil
}

func isTty() bool {
	f := syscall.MustLoadDLL("kernel32.dll").MustFindProc("GetConsoleMode")
	var st uint32
	r1, _, err := f.Call(uintptr(os.Stdin.Fd()), uintptr(unsafe.Pointer(&st)))
	return r1 != 0 && err != nil
}

var stdout = os.Stdout
var stdin = os.Stdin

func ttyReady() error {
	var err error
	_stdin, err := os.Open("CONIN$")
	if err != nil {
		return err
	}
	_stdout, err := os.Open("CONOUT$")
	if err != nil {
		return err
	}

	stdin = os.Stdin
	stdout = os.Stdout

	os.Stdin = _stdin
	os.Stdout = _stdout

	syscall.Stdin = syscall.Handle(os.Stdin.Fd())
	err = setStdHandle(syscall.STD_INPUT_HANDLE, syscall.Stdin)
	if err != nil {
		return err
	}
	syscall.Stdout = syscall.Handle(os.Stdout.Fd())
	err = setStdHandle(syscall.STD_OUTPUT_HANDLE, syscall.Stdout)
	if err != nil {
		return err
	}

	return nil
}

func ttyTerm() {
	os.Stdin = stdin
	syscall.Stdin = syscall.Handle(os.Stdin.Fd())
	setStdHandle(syscall.STD_INPUT_HANDLE, syscall.Stdin)
	os.Stdout = stdout
	syscall.Stdout = syscall.Handle(os.Stdout.Fd())
	setStdHandle(syscall.STD_OUTPUT_HANDLE, syscall.Stdout)
}
