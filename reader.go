package peco

import (
	"bufio"
	"fmt"
	"io"
	"os"
    "regexp"
	"sync"
	"time"
)

var ansiEscape = regexp.MustCompile("\x1B\\[(?:[0-9]{1,2}(?:;[0-9]{1,2})?)?[m|K]");

// BufferReader reads lines from the input, either Stdin or a file.
// If the incoming data is endless, it keeps reading and adding to
// the search buffer, as long as it can.
//
// If you would like to limit the number of lines to keep in the
// buffer, you should set --buffer-size to a number > 0
type BufferReader struct {
	*Ctx
	input        io.ReadCloser
	inputReadyCh chan struct{}
}

// InputReadyCh returns a channel which, when the input starts coming
// in, sends a struct{}{}
func (b *BufferReader) InputReadyCh() <-chan struct{} {
	return b.inputReadyCh
}

// Loop keeps reading from the input
func (b *BufferReader) Loop() {
	defer b.ReleaseWaitGroup()
	defer func() { recover() }()             // ignore errors
	defer func() { close(b.inputReadyCh) }() // Make sure to close notifier

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
	once := &sync.Once{}
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
                line = EscapeAnsiChar(line)
				once.Do(func() { b.inputReadyCh <- struct{}{} })
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

func EscapeAnsiChar(s string) string {
    return ansiEscape.ReplaceAllString(s, "")
}
