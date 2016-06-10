package peco

func (q *Query) Reset() {
	q.query = []rune(nil)
}

func (q *Query) RestoreSavedQuery() {
	q.query = q.savedQuery
	q.query = []rune(nil)
}

func (q *Query) SaveQuery() {
	q.savedQuery = q.query
	q.savedQuery = []rune(nil)
}

func (q *Query) DeleteRange(start, end int) {
	if start == -1 {
		return
	}

	if end > q.Len() {
		end = q.Len()
	}

	// everything up to "start" is left in tact
	// everything between start <-> end is deleted
	// everything up to "start" is left in tact
	// everything between start <-> end is deleted
	copy(q.query[start:], q.query[end:])
	q.query = q.query[:q.Len()-(end-start)]
}

func (q Query) String() string {
	return string(q.query)
}

func (q Query) Len() int {
	return len(q.query)
}

func (q *Query) Append(r rune) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	q.query = append(q.query, r)
}

func (q Query) Runes() []rune {
	return q.query
}

func (q Query) RuneAt(where int) rune {
	return q.query[where]
}

func (q *Query) InsertAt(ch rune, where int) {
	if where == q.Len() {
		q.Append(ch)
		return
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

	sq := q.query
	buf := make([]rune, len(sq)+1)
	copy(buf, sq[:where])
	buf[where] = ch
	copy(buf[where+1:], sq[where:])
	q.query = buf
}

