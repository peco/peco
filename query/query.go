package query

import "sync"

// Text holds the current query string and an optional saved query
// for restore-after-cancel behavior.
type Text struct {
	query      []rune
	savedQuery []rune
	mutex      sync.Mutex
}

func (q *Text) Set(s string) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.query = []rune(s)
}

func (q *Text) Reset() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.query = []rune(nil)
}

func (q *Text) RestoreSavedQuery() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.query = q.savedQuery
	q.savedQuery = []rune(nil)
}

func (q *Text) SaveQuery() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.savedQuery = q.query
	q.query = []rune(nil)
}

// DeleteRange deletes runes in the range [start, end) from the query with boundary validation.
func (q *Text) DeleteRange(start, end int) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if start < 0 {
		return
	}

	l := len(q.query)
	if start > l {
		return
	}

	if end > l {
		end = l
	}

	if start >= end {
		return
	}

	// everything up to "start" is left intact
	// everything between start <-> end is deleted
	copy(q.query[start:], q.query[end:])
	q.query = q.query[:l-(end-start)]
}

func (q *Text) String() string {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return string(q.query)
}

func (q *Text) Len() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return len(q.query)
}

// RuneSlice returns a copy of the query runes
func (q *Text) RuneSlice() []rune {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	out := make([]rune, len(q.query))
	copy(out, q.query)
	return out
}

func (q *Text) RuneAt(where int) rune {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if where < 0 || where >= len(q.query) {
		return 0
	}
	return q.query[where]
}

// InsertAt inserts a rune at the specified position in the query.
func (q *Text) InsertAt(ch rune, where int) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	l := len(q.query)
	if where < 0 || where > l {
		return
	}

	if where == l {
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

// Caret tracks the cursor position within the query line.
type Caret struct {
	mutex sync.Mutex
	pos   int
}

// Pos returns the current caret position, thread-safe.
func (c *Caret) Pos() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.pos
}

// setPosNL sets the caret position without acquiring the mutex.
// The caller must already hold the lock.
func (c *Caret) setPosNL(p int) {
	c.pos = p
}

// SetPos sets the caret position, thread-safe.
func (c *Caret) SetPos(p int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.setPosNL(p)
}

// Move moves the caret by the given delta, thread-safe.
func (c *Caret) Move(diff int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.setPosNL(c.pos + diff)
}
