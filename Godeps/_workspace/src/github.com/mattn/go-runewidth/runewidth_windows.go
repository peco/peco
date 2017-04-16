package runewidth

import (
	"syscall"
)

var (
	kernel32   = syscall.NewLazyDLL("kernel32")
	procGetACP = kernel32.NewProc("GetACP")
)

func IsEastAsian() bool {
	r1, _, _ := procGetACP.Call()
	if r1 == 0 {
		return false
	}

	switch int(r1) {
	case 932, 51932, 936, 949, 950:
		return true
	}

	return false
}
