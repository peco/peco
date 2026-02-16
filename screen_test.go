package peco

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTermboxResumeNoDeadlock(t *testing.T) {
	tb := NewTermbox()

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

func TestTermboxResumeDoesNotDropSend(t *testing.T) {
	tb := NewTermbox()

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

func TestTermboxResumeContextCancelled(t *testing.T) {
	tb := NewTermbox()

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

func TestTermboxResumeContextCancelledWhileWaitingForReply(t *testing.T) {
	tb := NewTermbox()

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
