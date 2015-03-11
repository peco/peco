package peco

import (
	"bufio"
	"errors"
	"io"
	"sync"
	"time"
)

// BufferReader reads lines from the input, either Stdin or a file.
// If the incoming data is endless, it keeps reading and adding to
// the search buffer, as long as it can.
//
// If you would like to limit the number of lines to keep in the
// buffer, you should set --buffer-size to a number > 0
type InputReader struct {
	simplePipeline
	// Read from this channel to figure out when we started receiving data.
	// It will be fired ONLY ONCE. Only perform terminal configuration and/or
	// screen resets after you receive from this channel
	readyCh chan struct{}

	// This is where you receive incoming data. It could be os.Stdin, or
	// a stream from a file
	src io.Reader
}

func NewInputReader(src io.Reader) *InputReader {
	return &InputReader{
		simplePipeline: simplePipeline{
			cancelCh: make(chan struct{}),
			outputCh: make(chan Line),
		},
		readyCh: make(chan struct{}),
		src:     src,
	}
}

func (ir InputReader) ReadyCh() chan struct{} { return ir.readyCh }

func (ir *InputReader) notifyReady() {
	tracer.Printf("InputReader: Notifying arrival of data")
	ir.readyCh <- struct{}{}
}

func (ir *InputReader) Loop() {
	tracer.Printf("InputReader.Loop: START")
	defer tracer.Printf("InputReader.Loop: END")

	defer close(ir.outputCh)
	// This is the actual reader. It's isolated in its own goroutine
	// because scanner.Scan() may block. The lines that were read
	// are fed into readCh
	readCh := make(chan string, 10)
	go func(input io.Reader, readCh chan string) {
		// Ignore errors. We don't care.
		defer func() { recover() }()
		defer close(readCh)
		scanner := bufio.NewScanner(input)
		for scanner.Scan() {
			t := scanner.Text()
			tracer.Printf("InputReader: scanner read '%s'", t)
			readCh <- t
		}
	}(ir.src, readCh)

	// This loop waits for input (or end of it), or cancellation from the
	// caller.
	once := &sync.Once{}
	for loop := true; loop; {
		select {
		case <-ir.cancelCh:
			// Canceled
			loop = false
		case line, ok := <-readCh:
			// End of input
			if line == "" && !ok {
				tracer.Printf("InputReader: End of incoming input. Bailing out")
				loop = false
				continue
			}

			if line == "" {
				tracer.Printf("InputReader: Received empty line")
				// Received an empty line...? Sorry, we don't work with empty
				// lines, maybe the next line will be useful...
				continue
			}

			// Notify once that we have received something from the file/stdin
			once.Do(ir.notifyReady)

			tracer.Printf("InputReader: Sending '%s' to output", line)
			ir.outputCh <- NewRawLine(line, false)
		}
	}
}

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
