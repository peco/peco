package buffer

import (
	"sync"

	"github.com/peco/peco/line"
)

const filterBufSize = 1000

var lineListPool = sync.Pool{
	New: func() interface{} {
		return make([]line.Line, 0, filterBufSize)
	},
}

func ReleaseLineListBuf(l []line.Line) {
	if l == nil {
		return
	}
	l = l[0:0]
	lineListPool.Put(l)
}

func GetLineListBuf() []line.Line {
	l := lineListPool.Get().([]line.Line)
	return l
}
