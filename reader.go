package peco

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type BufferReader struct {
	*Ctx
	input io.ReadCloser
}

func (b *BufferReader) Loop() {
	defer b.ReleaseWaitGroup()
	defer func() { recover() }() // ignore errors

	ch := make(chan string, 10)

	// scanner.Scan() blocks until the next read or error. But we want to
	// exit immediately, so we move it out to its own goroutine
	go func() {
		defer func() { recover() }()
		defer func() { close(ch) }()
		scanner := bufio.NewScanner(b.input)
		for scanner.Scan() {
			ch <- scanner.Text()
		}
	}()

	m := &sync.Mutex{}
	var refresh *time.Timer

	loop := true
	for loop {
		select {
		case <-b.LoopCh():
			loop = false
		case line, ok := <-ch:
			if !ok {
				loop = false
				continue
			}

			if line != "" {
				m.Lock()
				b.lines = append(b.lines, NewNoMatch(line, b.enableSep))
				if b.IsBufferOverflowing() {
					b.lines = b.lines[1:]
				}
				m.Unlock()
			}

			m.Lock()
			if refresh == nil {
				refresh = time.AfterFunc(100*time.Millisecond, func() {
					if !b.ExecQuery() {
						b.DrawMatches(b.lines)
					}
					m.Lock()
					refresh = nil
					m.Unlock()
				})
			}
			m.Unlock()
		}
	}

	b.input.Close()

	// Out of the reader loop. If at this point we have no buffer,
	// that means we have no buffer, so we should quit.
	if len(b.lines) == 0 {
		b.ExitWith(1)
		fmt.Fprintf(os.Stderr, "No buffer to work with was available")
	}
}
