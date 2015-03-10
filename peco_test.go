package peco

import "sync"

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

