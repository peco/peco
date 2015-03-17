package peco

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReader(t *testing.T) {
	ctx := NewCtx(nil)
	buf := strings.NewReader(`
1. Foo
2. Bar
3. Baz
`)
	rdr := ctx.NewBufferReader(ioutil.NopCloser(buf))

	inputReady := false
	time.AfterFunc(time.Second, func() {
		fmt.Fprintf(os.Stderr, "afterfunc fired\n")
		if !inputReady {
			t.Errorf("inputReady not receieved even after 1 second")
			close(rdr.inputReadyCh)
			ctx.Stop()
		}
	})

	go func() {
		<-rdr.inputReadyCh
		inputReady = true
	}()
	ctx.AddWaitGroup(1)
	rdr.Loop()

	if ctx.GetRawLineBufferSize() != 3 {
		t.Errorf("Expected 3 lines from input, only got %d", ctx.GetRawLineBufferSize())
	}
}
