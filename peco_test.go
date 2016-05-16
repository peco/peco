package peco

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
)

type interceptorArgs []interface{}
type interceptor struct {
	m      sync.Locker
	events map[string][]interceptorArgs
}

func newInterceptor() *interceptor {
	return &interceptor{
		newMutex(),
		make(map[string][]interceptorArgs),
	}
}

func (i *interceptor) reset() {
	i.m.Lock()
	defer i.m.Unlock()

	i.events = make(map[string][]interceptorArgs)
}

func (i *interceptor) record(name string, args []interface{}) {
	i.m.Lock()
	defer i.m.Unlock()

	events := i.events
	v, ok := events[name]
	if !ok {
		v = []interceptorArgs{}
	}

	events[name] = append(v, interceptorArgs(args))
}

func TestIDGen(t *testing.T) {
	lines := []*RawLine{}
	for i := 0; i < 1000000; i++ {
		lines = append(lines, NewRawLine(fmt.Sprintf("%d", i), false))
	}

	sel := NewSelection()
	for _, l := range lines {
		if sel.Has(l) {
			t.Errorf("Collision detected %d", l.ID())
		}
		sel.Add(l)
	}
}

func TestPeco(t *testing.T) {
	p := Peco{
		Args: []string{"peco_test.go"},
	}

	time.AfterFunc(time.Second, func() {
		p.Exit(nil)
	})
	if !assert.NoError(t, p.Run(), "p.Run() succeeds") {
		return
	}

	spew.Dump(p)
}
