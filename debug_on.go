// +build debug

package peco

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"sync"
)

const debug = true
var tracer *log.Logger
func init() {
	if v, err := strconv.ParseBool(os.Getenv("PECO_TRACE")); err == nil && v {
		tracer = log.New(os.Stderr, "peco: ", log.LstdFlags)
		tracer.Printf("==== INITIALIZED tracer ====")
	}
}

func trace(f string, args ...interface{}) {
	if tracer == nil {
		return
	}
	tracer.Printf(f, args...)
}

func newMutex() sync.Locker {
	return &loggingMutex{&sync.Mutex{}}
}

type loggingMutex struct {
	*sync.Mutex
}

func (m *loggingMutex) Lock() {
	buf := make([]byte, 8092)
	l := runtime.Stack(buf, false)
	fmt.Printf("LOCK %s\n", buf[:l])
	m.Mutex.Lock()
}

func (m *loggingMutex) Unlock() {
	buf := make([]byte, 8092)
	l := runtime.Stack(buf, false)
	fmt.Printf("UNLOCK %s\n", buf[:l])
	m.Mutex.Unlock()
}
