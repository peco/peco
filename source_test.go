package peco

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// delay reading so we can see that
type delayedReader struct {
	io.Reader
	delay time.Duration
	once  sync.Once
}

func addReadDelay(r io.Reader, delay time.Duration) io.Reader {
	return &delayedReader{
		Reader: r,
		delay:  delay,
	}
}

func (r *delayedReader) Read(b []byte) (int, error) {
	r.once.Do(func() { time.Sleep(r.delay) })
	return r.Reader.Read(b)
}

func TestSource(t *testing.T) {
	lines := []string{
		"foo",
		"bar",
		"baz",
	}
	ctx := t.Context()

	ig := newIDGen()
	go ig.Run(ctx)

	r := addReadDelay(strings.NewReader(strings.Join(lines, "\n")), 2*time.Second)
	s := NewSource("-", r, false, ig, 0, false, false)
	p := New()
	p.hub = nullHub{}
	go s.Setup(ctx, p)

	timeout := time.After(5 * time.Second)
	waitout := time.After(1 * time.Second)
	select {
	case <-waitout:
		_, ok := <-s.Ready()
		require.False(t, ok, "s.Ready should be false at this point")
	case <-timeout:
		t.Fatal("timed out waiting for source")
	case <-s.Ready():
	}

	// Even if s.Ready() returns, we may still be reading.
	// Wait for the buffer to fill up to the expected number of lines.
	require.Eventually(t, func() bool {
		return s.Size() == len(lines)
	}, 5*time.Second, 10*time.Millisecond, "buffer should fill up to %d lines", len(lines))

	for i := range lines {
		line, err := s.LineAt(i)
		require.NoError(t, err, "s.LineAt(%d) should succeed", i)
		require.Equal(t, line.DisplayString(), lines[i], "expected line found")
	}
}

// errorAfterReader returns data from the underlying reader, then once
// the underlying reader is exhausted it returns a specified error.
type errorAfterReader struct {
	io.Reader
	err    error
	hitEOF bool
}

func (r *errorAfterReader) Read(p []byte) (int, error) {
	if r.hitEOF {
		return 0, r.err
	}
	n, err := r.Reader.Read(p)
	if err == io.EOF {
		r.hitEOF = true
		if n > 0 {
			return n, nil
		}
		return 0, r.err
	}
	return n, err
}

func TestSourceScannerErr(t *testing.T) {
	ctx := t.Context()

	ig := newIDGen()
	go ig.Run(ctx)

	simulatedErr := errors.New("simulated I/O error")
	r := &errorAfterReader{
		Reader: strings.NewReader("line1\nline2\n"),
		err:    simulatedErr,
	}

	s := NewSource("-", r, false, ig, 0, false, false)
	p := New()
	rh := &recordingHub{}
	p.hub = rh
	s.Setup(ctx, p)

	// Verify lines read before the error are present
	require.Equal(t, 2, s.Size(), "should have read 2 lines before the error")
	l0, err := s.LineAt(0)
	require.NoError(t, err)
	require.Equal(t, "line1", l0.DisplayString())
	l1, err := s.LineAt(1)
	require.NoError(t, err)
	require.Equal(t, "line2", l1.DisplayString())

	// Verify the scanner error was reported via SendStatusMsg
	msgs := rh.getStatusMsgs()
	found := false
	for _, msg := range msgs {
		if strings.Contains(msg, "simulated I/O error") {
			found = true
			break
		}
	}
	require.True(t, found, "expected scanner error in status messages, got: %v", msgs)
}
