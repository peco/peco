package peco

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"runtime"
	"testing"
	"time"

	"context"

	"github.com/lestrrat-go/pdebug/v2"
	"github.com/nsf/termbox-go"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/mock"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
	"github.com/peco/peco/ui"
	"github.com/stretchr/testify/assert"
)

type nullHub struct{}

func (h nullHub) Batch(_ context.Context, _ func(context.Context), _ bool)           {}
func (h nullHub) DrawCh() chan hub.Payload                                           { return nil }
func (h nullHub) PagingCh() chan hub.Payload                                         { return nil }
func (h nullHub) QueryCh() chan hub.Payload                                          { return nil }
func (h nullHub) SendDraw(_ context.Context, _ interface{})                          {}
func (h nullHub) SendDrawPrompt(context.Context)                                     {}
func (h nullHub) SendPaging(_ context.Context, _ interface{})                        {}
func (h nullHub) SendQuery(_ context.Context, _ string)                              {}
func (h nullHub) SendStatusMsg(_ context.Context, _ string)                          {}
func (h nullHub) SendStatusMsgAndClear(_ context.Context, _ string, _ time.Duration) {}
func (h nullHub) StatusMsgCh() chan hub.Payload                                      { return nil }

func newConfig(s string) (string, error) {
	f, err := ioutil.TempFile("", "peco-test-config-")
	if err != nil {
		return "", err
	}

	_, _ = io.WriteString(f, s)
	f.Close()
	return f.Name(), nil
}

func newPeco() *Peco {
	_, file, _, _ := runtime.Caller(0)
	state := New()
	state.Argv = []string{"peco", file}
	state.screen = mock.NewScreen()
	state.skipReadConfig = true
	return state
}

func TestIDGen(t *testing.T) {
	idgen := newIDGen()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go idgen.Run(ctx)

	lines := []*line.Raw{}
	for i := 0; i < 1000000; i++ {
		lines = append(lines, line.NewRaw(idgen.Next(), fmt.Sprintf("%d", i), false))
	}

	sel := ui.NewSelection()
	for _, l := range lines {
		if sel.Has(l) {
			t.Errorf("Collision detected %d", l.ID())
		}
		sel.Add(l)
	}
}

func TestPeco(t *testing.T) {
	p := newPeco()
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Second, cancel)
	if !assert.NoError(t, p.Run(ctx), "p.Run() succeeds") {
		return
	}
}

func TestPecoCancel(t *testing.T) {
	done := make(chan struct{})

	// This whole operation may block, so run the test in the background
	go func() {
		defer close(done)
		p := newPeco()

		p.Argv = []string{"peco"}

		buf := &bytes.Buffer{}
		buf.WriteString("foo\nbar\nbaz")
		p.Stdin = buf
		p.Stdout = &bytes.Buffer{}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		resultCh := make(chan error)
		go func() {
			defer close(resultCh)
			select {
			case <-ctx.Done():
				return
			case resultCh <- p.Run(ctx):
				return
			}
		}()

		<-p.Ready()

		var fired bool
		time.AfterFunc(200*time.Millisecond, func() {
			p.screen.SendEvent(termbox.Event{Key: termbox.KeyEsc})
			fired = true
		})

		select {
		case <-ctx.Done():
			t.Errorf("timeout reached")
			return
		case err := <-resultCh:
			if !assert.True(t, util.IsIgnorableError(err), "error should be ignorable: %s", err) {
				return
			}
			p.PrintResults()
		}

		if !assert.True(t, fired, `SendEvent should have fired`) {
			return
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		t.Errorf(`background test did not return in time`)
	case <-done:
	}
}

func TestPecoHelp(t *testing.T) {
	done := make(chan struct{})

	// This whole operation may block, so run the test in the background
	go func() {
		defer close(done)
		p := newPeco()

		p.Argv = []string{"peco", "-h"}
		p.Stdout = &bytes.Buffer{}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		err := p.Run(ctx)
		if !assert.True(t, util.IsIgnorableError(err), "p.Run() should return error with Ignorable() method, and it should return true") {
			return
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		t.Errorf(`background test did not return in time`)
	case <-done:
	}
}

func TestGHIssue331(t *testing.T) {
	// Note: we should check that the drawing process did not
	// use cached display, but ATM this seemed hard to do,
	// so we just check that the proper fields were populated
	// when peco was instantiated
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Second, cancel)

	p := newPeco()
	_ = p.Run(ctx)

	if !assert.NotEmpty(t, p.singleKeyJumpPrefixes, "singleKeyJumpPrefixes is not empty") {
		return
	}
	if !assert.NotEmpty(t, p.singleKeyJumpPrefixMap, "singleKeyJumpPrefixMap is not empty") {
		return
	}
}

func TestConfigFuzzyFilter(t *testing.T) {
	var opts CLIOptions
	p := newPeco()

	// Ensure that it's possible to enable the Fuzzy filter
	opts.OptInitialFilter = "Fuzzy"
	if !assert.NoError(t, p.ApplyConfig(opts), "p.ApplyConfig should succeed") {
		return
	}
}

func TestApplyConfig(t *testing.T) {
	// XXX We should add all the possible configurations that needs to be
	// propagated to Peco from config

	// This is a placeholder test address
	// https://github.com/peco/peco/pull/338#issuecomment-244462220
	var opts CLIOptions

	opts.OptPrompt = "tpmorp>"
	opts.OptQuery = "Hello, World"
	opts.OptBufferSize = 256
	opts.OptInitialIndex = 2
	opts.OptInitialFilter = "Regexp"
	opts.OptLayout = "bottom-up"
	opts.OptSelect1 = true
	opts.OptOnCancel = "error"
	opts.OptSelectionPrefix = ">"
	opts.OptPrintQuery = true

	p := newPeco()
	if !assert.NoError(t, p.ApplyConfig(opts), "p.ApplyConfig should succeed") {
		return
	}

	if !assert.Equal(t, opts.OptQuery, p.initialQuery, "p.initialQuery should be equal to opts.Query") {
		return
	}

	if !assert.Equal(t, opts.OptBufferSize, p.bufferSize, "p.bufferSize should be equal to opts.BufferSize") {
		return
	}

	if !assert.Equal(t, opts.OptEnableNullSep, p.enableSep, "p.enableSep should be equal to opts.OptEnableNullSep") {
		return
	}

	if !assert.Equal(t, opts.OptInitialIndex, p.Location().LineNumber(), "p.Location().LineNumber() should be equal to opts.OptInitialIndex") {
		return
	}

	if !assert.Equal(t, opts.OptInitialFilter, p.filters.Current().String(), "p.initialFilter should be equal to opts.OptInitialFilter") {
		return
	}

	if !assert.Equal(t, opts.OptPrompt, p.prompt, "p.prompt should be equal to opts.OptPrompt") {
		return
	}

	if !assert.Equal(t, opts.OptLayout, p.layoutType, "p.layoutType should be equal to opts.OptLayout") {
		return
	}

	if !assert.Equal(t, opts.OptSelect1, p.selectOneAndExit, "p.selectOneAndExit should be equal to opts.OptSelect1") {
		return
	}

	if !assert.Equal(t, opts.OptOnCancel, p.onCancel, "p.onCancel should be equal to opts.OptOnCancel") {
		return
	}

	if !assert.Equal(t, opts.OptSelectionPrefix, p.selectionPrefix, "p.selectionPrefix should be equal to opts.OptSelectionPrefix") {
		return
	}
	if !assert.Equal(t, opts.OptPrintQuery, p.printQuery, "p.printQuery should be equal to opts.OptPrintQuery") {
		return
	}
}

// While this issue is labeled for Issue363, it tests against 376 as well.
// The test should have caught the bug for 376, but the premise of the test
// itself was wrong
func TestGHIssue363(t *testing.T) {
	ctx, cancel := context.WithTimeout(pdebug.Context(context.Background()), time.Second)
	defer cancel()

	p := newPeco()
	p.Argv = []string{"--select-1"}
	p.Stdin = bytes.NewBufferString("foo\n")
	var out bytes.Buffer
	p.Stdout = &out

	resultCh := make(chan error)
	go func() {
		defer close(resultCh)
		select {
		case <-ctx.Done():
			return
		case resultCh <- p.Run(ctx):
			return
		}
	}()

	select {
	case <-ctx.Done():
		t.Errorf("timeout reached")
		return
	case err := <-resultCh:
		if !assert.True(t, util.IsCollectResultsError(err), "isCollectResultsError") {
			return
		}
		p.PrintResults()
	}

	if !assert.Equal(t, "foo\n", out.String(), "output should match") {
		return
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) {
	return f(p)
}

func TestGHIssue367(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p := newPeco()
	p.Argv = []string{}
	src := [][]byte{
		[]byte("foo\n"),
		[]byte("bar\n"),
	}
	ac := time.After(50 * time.Millisecond)
	p.Stdin = readerFunc(func(p []byte) (int, error) {
		if ac != nil {
			<-ac
			ac = nil
		}

		if len(src) == 0 {
			return 0, io.EOF
		}

		l := len(src[0])
		copy(p, src[0])
		p = p[:l]
		src = src[1:]
		if pdebug.Enabled {
			pdebug.Printf(ctx, "reader func returning %#v", string(p))
		}
		return l, nil
	})
	buf := bytes.Buffer{}
	p.Stdout = &buf

	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		_ = p.Run(ctx)
	}()

	select {
	case <-time.After(100 * time.Millisecond):
		p.screen.SendEvent(termbox.Event{Ch: 'b'})
	case <-time.After(200 * time.Millisecond):
		p.screen.SendEvent(termbox.Event{Ch: 'a'})
	case <-time.After(300 * time.Millisecond):
		p.screen.SendEvent(termbox.Event{Ch: 'r'})
	case <-time.After(900 * time.Millisecond):
		p.screen.SendEvent(termbox.Event{Key: termbox.KeyEnter})
	}

	<-waitCh

	p.PrintResults()

	curbuf := p.CurrentLineBuffer()

	if !assert.Equal(t, curbuf.Size(), 1, "There should be one element in buffer") {
		return
	}

	for i := 0; i < curbuf.Size(); i++ {
		_, err := curbuf.LineAt(i)
		if !assert.NoError(t, err, "LineAt(%d) should succeed", i) {
			return
		}
	}

	if !assert.Equal(t, "bar\n", buf.String(), "output should match") {
		return
	}
}

func TestPrintQuery(t *testing.T) {
	t.Run("Match and print query", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		p := newPeco()
		p.Argv = []string{"--print-query", "--query", "oo", "--select-1"}
		p.Stdin = bytes.NewBufferString("foo\n")
		var out bytes.Buffer
		p.Stdout = &out

		resultCh := make(chan error)
		go func() {
			defer close(resultCh)
			select {
			case <-ctx.Done():
				return
			case resultCh <- p.Run(ctx):
				return
			}
		}()

		select {
		case <-ctx.Done():
			t.Errorf("timeout reached")
			return
		case err := <-resultCh:
			if !assert.True(t, util.IsCollectResultsError(err), "isCollectResultsError") {
				return
			}
			p.PrintResults()
		}

		if !assert.Equal(t, "oo\nfoo\n", out.String(), "output should match") {
			return
		}
	})
	t.Run("No match and print query", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		p := newPeco()
		p.Argv = []string{"--print-query", "--query", "oo"}
		p.Stdin = bytes.NewBufferString("bar\n")
		var out bytes.Buffer
		p.Stdout = &out

		resultCh := make(chan error)
		go func() {
			defer close(resultCh)
			select {
			case <-ctx.Done():
				return
			case resultCh <- p.Run(ctx):
				return
			}
		}()

		<-p.Ready()

		time.AfterFunc(100*time.Millisecond, func() {
			p.screen.SendEvent(termbox.Event{Key: termbox.KeyEnter})
		})

		select {
		case <-ctx.Done():
			t.Errorf("timeout reached")
			return
		case err := <-resultCh:
			if !assert.True(t, util.IsCollectResultsError(err), "isCollectResultsError") {
				return
			}
			p.PrintResults()
		}

		if !assert.Equal(t, "oo\n", out.String(), "output should match") {
			return
		}
	})
}
