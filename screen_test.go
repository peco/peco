package peco

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/require"
)

// setCellCall records a single SetCell invocation.
type setCellCall struct {
	x, y int
	ch   rune
}

// recordingScreen is a minimal Screen implementation that records SetCell calls.
// Used by screenPrint tests to verify exactly which cells are written.
type recordingScreen struct {
	cells []setCellCall
	w, h  int
}

func (s *recordingScreen) Init(*Config) error                            { return nil }
func (s *recordingScreen) Close() error                                  { return nil }
func (s *recordingScreen) Flush() error                                  { return nil }
func (s *recordingScreen) PollEvent(context.Context, *Config) chan Event { return nil }
func (s *recordingScreen) Print(args PrintArgs) int                      { return screenPrint(s, args) }
func (s *recordingScreen) Resume(context.Context) error                  { return nil }
func (s *recordingScreen) SetCursor(int, int)                            {}
func (s *recordingScreen) SendEvent(Event)                               {}
func (s *recordingScreen) Suspend()                                      {}
func (s *recordingScreen) Sync()                                         {}
func (s *recordingScreen) Size() (int, int)                              { return s.w, s.h }
func (s *recordingScreen) SetCell(x, y int, ch rune, _, _ Attribute) {
	s.cells = append(s.cells, setCellCall{x: x, y: y, ch: ch})
}

func TestScreenPrintTabWidth(t *testing.T) {
	tests := []struct {
		name      string
		msg       string
		x         int
		xOffset   int
		wantCells int   // number of SetCell calls for tab expansion
		wantX     []int // x coordinates of SetCell calls
	}{
		{
			name:      "tab at column 0 expands to 4 spaces",
			msg:       "\t",
			x:         0,
			xOffset:   0,
			wantCells: 4,
			wantX:     []int{0, 1, 2, 3},
		},
		{
			name:      "tab at column 1 expands to 3 spaces (next tab stop at 4)",
			msg:       "\t",
			x:         1,
			xOffset:   0,
			wantCells: 3,
			wantX:     []int{1, 2, 3},
		},
		{
			name:      "tab at column 2 expands to 2 spaces",
			msg:       "\t",
			x:         2,
			xOffset:   0,
			wantCells: 2,
			wantX:     []int{2, 3},
		},
		{
			name:      "tab at column 3 expands to 1 space",
			msg:       "\t",
			x:         3,
			xOffset:   0,
			wantCells: 1,
			wantX:     []int{3},
		},
		{
			name:      "tab at column 4 expands to 4 spaces (next tab stop at 8)",
			msg:       "\t",
			x:         4,
			xOffset:   0,
			wantCells: 4,
			wantX:     []int{4, 5, 6, 7},
		},
		{
			name:      "xOffset affects tab stop calculation",
			msg:       "\t",
			x:         0,
			xOffset:   1,
			wantCells: 3, // 4 - (0+1)%4 = 3
			wantX:     []int{0, 1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scr := &recordingScreen{w: 80, h: 24}
			written := screenPrint(scr, PrintArgs{
				X:       tt.x,
				XOffset: tt.xOffset,
				Y:       0,
				Msg:     tt.msg,
			})

			require.Equal(t, tt.wantCells, len(scr.cells), "number of SetCell calls")
			require.Equal(t, tt.wantCells, written, "written count should match cells drawn")

			gotX := make([]int, len(scr.cells))
			for i, c := range scr.cells {
				gotX[i] = c.x
				require.Equal(t, ' ', c.ch, "tab should expand to spaces")
			}
			require.Equal(t, tt.wantX, gotX, "x coordinates of drawn cells")
		})
	}
}

func TestScreenPrintTabThenChar(t *testing.T) {
	// Verifies that after a tab, the next character is placed at the correct position.
	// With tab at column 0 expanding to 4 spaces, the next char should be at column 4.
	scr := &recordingScreen{w: 80, h: 24}
	written := screenPrint(scr, PrintArgs{
		X:   0,
		Y:   0,
		Msg: "\tA",
	})

	// Tab (4 spaces) + 'A' (1 cell) = 5 cells drawn, 5 written
	require.Equal(t, 5, len(scr.cells))
	require.Equal(t, 5, written)

	// The 'A' should be at x=4 (right after the 4-space tab)
	lastCell := scr.cells[len(scr.cells)-1]
	require.Equal(t, 'A', lastCell.ch)
	require.Equal(t, 4, lastCell.x, "character after tab should be at x=4")
}

// panickingScreen wraps a tcell.Screen and panics on PollEvent.
// Used to test that TcellScreen's PollEvent goroutine logs panics
// instead of silently swallowing them.
type panickingScreen struct {
	tcell.Screen
}

func (s *panickingScreen) PollEvent() tcell.Event {
	panic("test: deliberate panic in PollEvent")
}

// TestTcellScreenPollEventLogsPanic verifies that when a panic occurs
// in the PollEvent goroutine, it is logged to errWriter rather than
// being silently swallowed (the bug described in CODE_REVIEW.md §3.2).
func TestTcellScreenPollEventLogsPanic(t *testing.T) {
	var buf bytes.Buffer
	ts := NewTcellScreen()
	ts.errWriter = &buf

	// Set the screen to a wrapper that panics on PollEvent.
	sim := tcell.NewSimulationScreen("")
	sim.Init()
	ts.screen = &panickingScreen{Screen: sim}

	ctx := t.Context()

	evCh := ts.PollEvent(ctx, nil)

	// The goroutine should panic, log it, and close the channel.
	select {
	case _, ok := <-evCh:
		require.False(t, ok, "expected channel to be closed after panic")
	case <-time.After(2 * time.Second):
		t.Fatal("PollEvent channel was not closed after panic")
	}

	// Verify that the panic was logged (not silently swallowed).
	output := buf.String()
	require.Contains(t, output, "peco: panic in PollEvent goroutine")
	require.Contains(t, output, "test: deliberate panic in PollEvent")

	ts.Close()
}

// TestTcellScreenSuspendHandlerExitsOnClose verifies that the suspend handler
// goroutine (started by PollEvent) exits when Close() is called, even if
// the context has not been cancelled. This is the goroutine leak described
// in CODE_REVIEW.md §7.1.
func TestTcellScreenSuspendHandlerExitsOnClose(t *testing.T) {
	tb := NewTcellScreen()

	// Use a context that will NOT be cancelled during this test.
	// The goroutine must exit via doneCh, not ctx.Done().
	ctx := context.Background()

	// Start a goroutine that mimics the suspend handler's select loop.
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		for {
			select {
			case <-ctx.Done():
				return
			case <-tb.doneCh:
				return
			case <-tb.suspendCh:
				tb.finiScreen()
			}
		}
	}()

	// Permanently close the screen.
	tb.Close()

	select {
	case <-exited:
		// Goroutine exited via doneCh — no leak.
	case <-time.After(2 * time.Second):
		t.Fatal("suspend handler goroutine did not exit after Close()")
	}
}

// TestTcellScreenPollingGoroutineExitsOnClose verifies that a goroutine blocked
// on resumeCh (as the polling goroutine would be after screen finalization)
// exits when Close() is called.
func TestTcellScreenPollingGoroutineExitsOnClose(t *testing.T) {
	tb := NewTcellScreen()

	ctx := context.Background()

	exited := make(chan struct{})
	go func() {
		defer close(exited)
		// Simulate the polling goroutine waiting for resume after screen==nil.
		select {
		case <-ctx.Done():
			return
		case <-tb.doneCh:
			return
		case replyCh := <-tb.resumeCh:
			replyCh <- nil
		}
	}()

	tb.Close()

	select {
	case <-exited:
		// Goroutine exited via doneCh.
	case <-time.After(2 * time.Second):
		t.Fatal("polling goroutine did not exit after Close()")
	}
}

// TestTcellScreenCloseIdempotent verifies that Close() can be called multiple
// times without panicking (important because the suspend handler calls
// finiScreen and then Close() is called at shutdown).
func TestTcellScreenCloseIdempotent(t *testing.T) {
	tb := NewTcellScreen()

	require.NotPanics(t, func() {
		tb.Close()
		tb.Close()
		tb.Close()
	})
}

// TestTcellScreenSuspendThenClose verifies that a suspend (which calls finiScreen)
// followed by a permanent Close() works correctly — the doneCh should be
// closed by Close() even though finiScreen was already called.
func TestTcellScreenSuspendThenClose(t *testing.T) {
	tb := NewTcellScreen()

	ctx := context.Background()

	exited := make(chan struct{})
	go func() {
		defer close(exited)
		for {
			select {
			case <-ctx.Done():
				return
			case <-tb.doneCh:
				return
			case <-tb.suspendCh:
				tb.finiScreen()
			}
		}
	}()

	// Send a suspend signal, which calls finiScreen (not Close).
	tb.Suspend()
	// Give the goroutine time to process the suspend.
	time.Sleep(50 * time.Millisecond)

	// Now permanently close.
	tb.Close()

	select {
	case <-exited:
		// Goroutine exited after Close() following a suspend.
	case <-time.After(2 * time.Second):
		t.Fatal("suspend handler goroutine did not exit after suspend + Close()")
	}
}

func TestTcellScreenResumeNoDeadlock(t *testing.T) {
	tb := NewTcellScreen()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Simulate the polling goroutine: receive from resumeCh after a short delay,
	// then send nil error (as PollEvent does after successful re-init).
	go func() {
		time.Sleep(50 * time.Millisecond)
		replyCh := <-tb.resumeCh
		replyCh <- nil
	}()

	done := make(chan struct{})
	go func() {
		require.NoError(t, tb.Resume(ctx))
		close(done)
	}()

	select {
	case <-done:
		// Resume completed without deadlock.
	case <-time.After(2 * time.Second):
		t.Fatal("Resume() deadlocked")
	}
}

func TestTcellScreenResumeDoesNotDropSend(t *testing.T) {
	tb := NewTcellScreen()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	received := make(chan struct{})
	go func() {
		replyCh := <-tb.resumeCh
		close(received)
		replyCh <- nil
	}()

	require.NoError(t, tb.Resume(ctx))

	select {
	case <-received:
		// The receiver goroutine got the message.
	default:
		t.Fatal("receiver did not get the resume message")
	}
}

func TestTcellScreenResumeContextCancelled(t *testing.T) {
	tb := NewTcellScreen()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately so Resume cannot deliver on resumeCh.
	cancel()

	done := make(chan struct{})
	go func() {
		err := tb.Resume(ctx)
		require.Error(t, err)
		close(done)
	}()

	select {
	case <-done:
		// Resume returned promptly after context cancellation.
	case <-time.After(2 * time.Second):
		t.Fatal("Resume() did not unblock after context cancellation")
	}
}

func TestTcellScreenResumeContextCancelledWhileWaitingForReply(t *testing.T) {
	tb := NewTcellScreen()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Accept the resume request but never close the reply channel.
	// This tests that the second select also respects ctx.Done().
	go func() {
		<-tb.resumeCh // receive but don't close replyCh
	}()

	done := make(chan struct{})
	go func() {
		err := tb.Resume(ctx)
		require.Error(t, err)
		close(done)
	}()

	// Give Resume time to pass the first select and block on the second.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Resume returned after context cancellation during reply wait.
	case <-time.After(2 * time.Second):
		t.Fatal("Resume() did not unblock after context cancellation while waiting for reply")
	}

	// Verify context was indeed cancelled.
	require.Error(t, ctx.Err())
}

func TestTcellScreenResumeInitError(t *testing.T) {
	tb := NewTcellScreen()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	initErr := errors.New("simulated screen init failure")

	// Simulate the polling goroutine: receive from resumeCh and send
	// an error as if Init() failed.
	go func() {
		replyCh := <-tb.resumeCh
		replyCh <- initErr
	}()

	err := tb.Resume(ctx)
	require.Error(t, err)
	require.Equal(t, initErr, err)
}
