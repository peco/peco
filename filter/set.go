package filter

import (
	"errors"
	"sync"

	pdebug "github.com/lestrrat-go/pdebug"
)

// ErrFilterNotFound is returned when a filter name does not match any
// registered filter in the Set.
var ErrFilterNotFound = errors.New("specified filter was not found")

// Set holds the collection of available filters and tracks which one
// is currently active.
type Set struct {
	current int
	filters []Filter
	mutex   sync.Mutex
}

func (fs *Set) Reset() {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	fs.current = 0
}

func (fs *Set) Size() int {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	return len(fs.filters)
}

func (fs *Set) Add(lf Filter) {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	fs.filters = append(fs.filters, lf)
}

func (fs *Set) Rotate() {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	fs.current++
	if fs.current >= len(fs.filters) {
		fs.current = 0
	}
	if pdebug.Enabled {
		pdebug.Printf("Set.Rotate: now filter in effect is %s", fs.filters[fs.current])
	}
}

func (fs *Set) SetCurrentByName(name string) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	for i, f := range fs.filters {
		if f.String() == name {
			fs.current = i
			return nil
		}
	}
	return ErrFilterNotFound
}

func (fs *Set) Index() int {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	return fs.current
}

func (fs *Set) Current() Filter {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	return fs.filters[fs.current]
}
