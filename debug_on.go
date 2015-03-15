// +build debug

package peco

import (
	"log"
	"os"
	"runtime"
	"strconv"
	"sync"
)

const debug = true

var tracer *log.Logger
var mutexTracer *log.Logger

func init() {
	if v, err := strconv.ParseBool(os.Getenv("PECO_TRACE")); err == nil && v {
		tracer = log.New(os.Stderr, "peco: ", log.LstdFlags)
		tracer.Printf("==== INITIALIZED tracer ====")
	}
	if v, err := strconv.ParseBool(os.Getenv("PECO_LOCK_TRACE")); err == nil && v {
		mutexTracer = log.New(os.Stderr, "mutex: ", log.LstdFlags)
		mutexTracer.Printf("==== INITIALIZED mutext tracer ====")
	}
}

func trace(f string, args ...interface{}) {
	if tracer == nil {
		return
	}
	tracer.Printf(f, args...)
}

func mutexTrace(f string, args ...interface{}) {
	if mutexTracer == nil {
		return
	}
	mutexTracer.Printf(f, args...)
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
	mutexTrace("LOCK %s\n", buf[:l])
	m.Mutex.Lock()
}

func (m *loggingMutex) Unlock() {
	buf := make([]byte, 8092)
	l := runtime.Stack(buf, false)
	mutexTrace("UNLOCK %s\n", buf[:l])
	m.Mutex.Unlock()
}
