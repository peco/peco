package query

func New() *Query {
	return &Query{}
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

// Runes returns a channel that gives you the list of runes in the query
func (q *Query) Runes() <-chan rune {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	c := make(chan rune, len(q.query))

	go func() {
		defer close(c)
		q.mutex.Lock()
		defer q.mutex.Unlock()

		for _, r := range q.query {
			c <- r
		}
	}()

	return c
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
