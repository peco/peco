package mock

import "sync"

type Interceptor struct {
	m      sync.Mutex
	Events map[string][]interface{}
}

func NewInterceptor() *Interceptor {
	return &Interceptor{
		Events: make(map[string][]interface{}),
	}
}

func (i *Interceptor) Reset() {
	i.m.Lock()
	defer i.m.Unlock()

	i.Events = make(map[string][]interface{})
}

func (i *Interceptor) Record(name string, args []interface{}) {
	i.m.Lock()
	defer i.m.Unlock()

	events := i.Events
	v, ok := events[name]
	if !ok {
		v = []interface{}{}
	}

	events[name] = append(v, interface{}(args))
}
