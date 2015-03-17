// +build !debug

package peco

import "sync"

const debug = true
func newMutex() sync.Locker {
	return &sync.Mutex{}
}

func trace(f string, args ...interface{}) {}