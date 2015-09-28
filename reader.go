package peco

import (
	"bufio"
	"errors"
	"io"
	"sync"
	"time"
)

// BufferReader reads from either stdin or a file. In case of stdin,
// it also handles possible infinite source.
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
	defer b.input.Close()

	ch := make(chan string, 10)

	// scanner.Scan() blocks until the next read or error. But we want our
	// main loop to be able to exit without blocking, so we move this out
	// to its own goroutine
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

	doDelayedDraw := func() {
		m.Lock()
		defer m.Unlock()

		trace("doDelayedDraw")

		if refresh != nil {
			return
		}

		refresh = time.AfterFunc(100*time.Millisecond, func() {
			if !b.ExecQuery() {
				b.SendDraw(false)
			}
			m.Lock()
			defer m.Unlock()
			refresh = nil
		})
	}

	for loop := true; loop; {
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
				// This is the cue to start initializing the terminal
				once.Do(func() { b.inputReadyCh <- struct{}{} })

				// Make sure we lock access to b.lines
				m.Lock()
				b.AddRawLine(NewRawLine(line, b.enableSep))
				m.Unlock()
			}

			doDelayedDraw()
		}
	}

	// Out of the reader loop. If at this point we have no buffer,
	// that means we have no buffer, so we should quit.
	if b.GetRawLineBufferSize() == 0 {
		b.ExitWith(errors.New("no buffer to work with was available"))
	}
}
