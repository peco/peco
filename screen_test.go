package peco

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
			close(replyCh)
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
	// then close the reply channel (as PollEvent does after re-init).
	go func() {
		time.Sleep(50 * time.Millisecond)
		replyCh := <-tb.resumeCh
		close(replyCh)
	}()

	done := make(chan struct{})
	go func() {
		tb.Resume(ctx)
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
		close(replyCh)
	}()

	tb.Resume(ctx)

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
		tb.Resume(ctx)
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
		tb.Resume(ctx)
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
