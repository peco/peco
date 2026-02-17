package buffer

import (
	"sync"

	"github.com/peco/peco/line"
)

const DefaultFilterBufSize = 1000

var lineListPool = sync.Pool{
	New: func() any {
		return make([]line.Line, 0, DefaultFilterBufSize)
	},
}

func ReleaseLineListBuf(l []line.Line) {
	if l == nil {
		return
	}
	l = l[0:0]
	lineListPool.Put(l) //nolint:staticcheck // SA6002: converting to pointer-based pool breaks tests
}

func GetLineListBuf() []line.Line {
	l, _ := lineListPool.Get().([]line.Line)
	return l
}
