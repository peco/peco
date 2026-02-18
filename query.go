package peco

import "sync"

// Query holds the current query string and an optional saved query
// for restore-after-cancel behavior.
type Query struct {
	query      []rune
	savedQuery []rune
	mutex      sync.Mutex
}

func (q *Query) Set(s string) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.query = []rune(s)
}

func (q *Query) Reset() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.query = []rune(nil)
}

func (q *Query) RestoreSavedQuery() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.query = q.savedQuery
	q.savedQuery = []rune(nil)
}

func (q *Query) SaveQuery() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.savedQuery = q.query
	q.query = []rune(nil)
}

func (q *Query) DeleteRange(start, end int) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if start == -1 {
		return
	}

	l := len(q.query)
	if end > l {
		end = l
	}

	if start > end {
		return
	}

	// everything up to "start" is left intact
	// everything between start <-> end is deleted
	copy(q.query[start:], q.query[end:])
	q.query = q.query[:l-(end-start)]
}

func (q *Query) String() string {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return string(q.query)
}

func (q *Query) Len() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return len(q.query)
}

// RuneSlice returns a copy of the query runes
func (q *Query) RuneSlice() []rune {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	out := make([]rune, len(q.query))
	copy(out, q.query)
	return out
}

func (q *Query) RuneAt(where int) rune {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if where < 0 || where >= len(q.query) {
		return 0
	}
	return q.query[where]
}

func (q *Query) InsertAt(ch rune, where int) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if where == len(q.query) {
		q.query = append(q.query, ch)
		return
	}

	sq := q.query
	buf := make([]rune, len(sq)+1)
	copy(buf, sq[:where])
	buf[where] = ch
	copy(buf[where+1:], sq[where:])
	q.query = buf
}
