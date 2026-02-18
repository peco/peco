package peco

import "sync"

// PageCrop filters out a new LineBuffer based on entries
// per page and the page number
type PageCrop struct {
	perPage     int
	currentPage int
}

// Location tracks the current viewport position within the line buffer.
type Location struct {
	mutex   sync.RWMutex
	col     int
	lineno  int
	maxPage int
	page    int
	perPage int
	offset  int
	total   int
}

func (l *Location) SetColumn(n int) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.col = n
}

func (l *Location) Column() int {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.col
}

func (l *Location) SetLineNumber(n int) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.lineno = n
}

func (l *Location) LineNumber() int {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.lineno
}

func (l *Location) SetOffset(n int) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.offset = n
}

func (l *Location) Offset() int {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.offset
}

func (l *Location) SetPerPage(n int) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.perPage = n
}

func (l *Location) PerPage() int {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.perPage
}

func (l *Location) SetPage(n int) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.page = n
}

func (l *Location) Page() int {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.page
}

func (l *Location) SetTotal(n int) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.total = n
}

func (l *Location) Total() int {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.total
}

func (l *Location) SetMaxPage(n int) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.maxPage = n
}

func (l *Location) MaxPage() int {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.maxPage
}

func (l *Location) PageCrop() PageCrop {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return PageCrop{
		perPage:     l.perPage,
		currentPage: l.page,
	}
}

// Crop returns a new Buffer whose contents are
// bound within the given range
func (pf PageCrop) Crop(in Buffer) *FilteredBuffer {
	return NewFilteredBuffer(in, pf.currentPage, pf.perPage)
}
