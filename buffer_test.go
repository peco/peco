package peco

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func ExampleBufferChain() {
	rawbuf := NewRawLineBuffer()
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		rawbuf.AppendLine(NewRawLine(scanner.Text(), false))
	}

	pc := PageCrop{perPage: 10, currentPage: 1}
	rf := RegexpFilter{
		flags: regexpFlagList(defaultFlags),
		query: `mattn is da king`,
	}

	result := pc.Crop(rf.Filter(rawbuf))
	for i := 0; i < result.Size(); i++ {
		l, err := result.LineAt(i)
		if err != nil {
			panic(err)
		}

		fmt.Printf("line = %s\n", l.DisplayString())
	}
}

func TestInputReaderToRawLineBuffer(t *testing.T) {
	buf := strings.NewReader(`
1. Foo
2. Bar
3. Baz
`)
	rdr := NewInputReader(ioutil.NopCloser(buf))
	rawbuf := NewRawLineBuffer()

	go rdr.Loop()

	<-rdr.ReadyCh()

	rawbuf.Accept(rdr)

	for l := range rawbuf.OutputCh() {
		t.Logf("Received new line %#v", l)
	}

	if rawbuf.Size() != 3 {
		t.Errorf("Expected 3 entries in RawLineBuffer, got %d", rawbuf.Size())
	}
}

func TestInputReaderToRawLineBufferToRegexpFilter(t *testing.T) {
	buf := strings.NewReader(`
1. Foo
2. Bar
3. Baz
`)
	rdr := NewInputReader(ioutil.NopCloser(buf))
	rawbuf := NewRawLineBuffer()

	go rdr.Loop()

	<-rdr.ReadyCh()

	rawbuf.Accept(rdr)
	rf := RegexpFilter{
		flags: regexpFlagList(ignoreCaseFlags),
		query: `\d\. b`,
	}
	rf.Accept(rawbuf)

	flb := NewRawLineBuffer()

	flb.Accept(rf)

	for l := range flb.OutputCh() {
		t.Logf("Received new line %#v", l)
	}

	if flb.Size() != 2 {
		t.Errorf("Expected 2 entries in RawLineBuffer, got %d", flb.Size())
	}
}

func TestBuffer(t *testing.T) {
	rawbuf := NewRawLineBuffer()
	for _, l := range []string{"Alice", "Bob", "Charlie"} {
		rawbuf.AppendLine(NewRawLine(l, false))
	}

	if rawbuf.Size() != 3 {
		t.Errorf("Expected to read 3 lines, got %d", rawbuf.Size())
	}

	f := RegexpFilter{
		flags: regexpFlagList(ignoreCaseFlags),
		query: `c`,
	}
	buf := f.Filter(rawbuf)

	if buf.Size() != 2 {
		t.Errorf("Expected to match 2 lines, got %d", buf.Size())
	}

	for i, v := range []string{"Alice", "Charlie"} {
		l, err := buf.LineAt(i)
		if err != nil {
			t.Errorf("Failed to get line at %d: %s", i, err)
			continue
		}

		if l.DisplayString() != v {
			t.Errorf("Expected filtered output at %d to be '%s', got '%s'", i, v, l.DisplayString())
		}
	}
}

func TestBufferPaging(t *testing.T) {
	rawbuf := NewRawLineBuffer()
	for _, l := range []string{"Alice", "Bob", "Charlie", "David", "Eve", "Frank", "George", "Hugh"} {
		rawbuf.AppendLine(NewRawLine(l, false))
	}

	pc := PageCrop{perPage: 4, currentPage: 2}
	pagebuf := pc.Crop(rawbuf)

	for i, v := range []string{"Eve", "Frank", "George", "Hugh"} {
		l, err := pagebuf.LineAt(i)
		if err != nil {
			t.Errorf("Failed to get line at %d: %s", i, err)
			continue
		}

		if l.DisplayString() != v {
			t.Errorf("Expected filtered output at %d to be '%s', got '%s'", i, v, l.DisplayString())
		}
	}

	// Also test regexp filter + paging
	rf := RegexpFilter{
		flags: regexpFlagList(ignoreCaseFlags),
		query: `a`,
	}
	pc.perPage = 2
	pagebuf = pc.Crop(rf.Filter(rawbuf))

	for i, v := range []string{"David", "Frank"} {
		l, err := pagebuf.LineAt(i)
		if err != nil {
			t.Errorf("Failed to get line at %d: %s", i, err)
			continue
		}

		if l.DisplayString() != v {
			t.Errorf("Expected filtered output at %d to be '%s', got '%s'", i, v, l.DisplayString())
		}
	}
}
