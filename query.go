package peco

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
	q.query = []rune(nil)
}

func (q *Query) SaveQuery() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.savedQuery = q.query
	q.savedQuery = []rune(nil)
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

	// everything up to "start" is left in tact
	// everything between start <-> end is deleted
	// everything up to "start" is left in tact
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

func (q *Query) Append(r rune) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	q.query = append(q.query, r)
}

// Runes returns a copy of the underlying query as an array of runes.
func (q *Query) Runes() []rune {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	ret := make([]rune, len(q.query))
	copy(ret, q.query)

	// Because this is a copy, the user of this function does not need
	// to know about locking and stuff
	return ret
}

func (q *Query) RuneAt(where int) rune {
	q.mutex.Lock()
	defer q.mutex.Unlock()
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
