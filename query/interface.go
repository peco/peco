package query

import "sync"

type Query struct {
	query      []rune
	savedQuery []rune
	mutex      sync.Mutex
}
