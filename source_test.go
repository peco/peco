package peco

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"context"
	"github.com/stretchr/testify/assert"
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ig := newIDGen()
	go ig.Run(ctx)

	r := addReadDelay(strings.NewReader(strings.Join(lines, "\n")), 2*time.Second)
	s := NewSource("-", r, ig, 0, false)
	p := New()
	p.hub = nullHub{}
	go s.Setup(ctx, p)

	timeout := time.After(5 * time.Second)
	waitout := time.After(1 * time.Second)
	select {
	case <-waitout:
		_, ok := <-s.Ready()
		if !assert.False(t, ok, "s.Ready should be false at this point") {
			return
		}
	case <-timeout:
		assert.Fail(t, "timed out waiting for source")
		return
	case <-s.Ready():
	}

	// Even if s.Ready() returns, we may still be reading.
	// Wait for another few seconds for the buffer to fill up to
	// the expected number of lines
	timeout = time.After(5 * time.Second)
	for s.Size() != len(lines) {
		select {
		case <-timeout:
			assert.Fail(t, "timed out waiting for the buffer to fill")
			return
		default:
			time.Sleep(time.Second)
		}
	}

	for i := 0; i < len(lines); i++ {
		line, err := s.LineAt(i)
		if !assert.NoError(t, err, "s.LineAt(%d) should succeed", i) {
			return
		}
		if !assert.Equal(t, line.DisplayString(), lines[i], "expected lien found") {
			return
		}
	}
}
