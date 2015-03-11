package peco

import (
	"bufio"
	"errors"
	"io"
	"sync"
	"time"
)

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

	m := newMutex()
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
				// Notify once that we have received something from the file/stdin
				once.Do(func() { b.inputReadyCh <- struct{}{} })

				// Make sure we lock access to b.lines
				m.Lock()
				b.AddRawLine(NewRawLine(line, b.enableSep))
				m.Unlock()
			}

			m.Lock()
			if refresh == nil {
				refresh = time.AfterFunc(100*time.Millisecond, func() {
					if !b.ExecQuery() {
						b.SendDraw()
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
	if b.GetRawLineBufferSize() == 0 {
		b.ExitWith(errors.New("no buffer to work with was available"))
	}
}
