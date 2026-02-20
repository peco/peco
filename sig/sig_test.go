package sig

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestLoopContextCancel verifies that Loop exits when the context is cancelled
// and that signal.Stop is called (the channel no longer receives signals).
func TestLoopContextCancel(t *testing.T) {
	var received os.Signal
	h := New(ReceivedHandlerFunc(func(sig os.Signal) {
		received = sig
	}), syscall.SIGUSR1)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- h.Loop(ctx, cancel)
	}()

	// Cancel the context to make Loop exit
	cancel()

	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		require.Fail(t, "Loop did not exit after context cancellation")
	}

	// After Loop returns, the signal channel should be deregistered.
	// Sending SIGUSR1 to ourselves should NOT be received by the handler.
	require.Nil(t, received, "handler should not have been called")

	// Verify the channel is stopped: send a signal and confirm it doesn't
	// arrive on the (now-stopped) channel.
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	require.Never(t, func() bool {
		select {
		case _, ok := <-h.sigCh:
			return ok
		default:
			return false
		}
	}, 100*time.Millisecond, 10*time.Millisecond,
		"signal was delivered to channel after Loop returned — signal.Stop was not called")
}

// TestLoopSignalReceived verifies that Loop calls the handler and exits
// when a signal is received, and that signal.Stop is called afterward.
func TestLoopSignalReceived(t *testing.T) {
	var received os.Signal
	h := New(ReceivedHandlerFunc(func(sig os.Signal) {
		received = sig
	}), syscall.SIGUSR1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- h.Loop(ctx, cancel)
	}()

	// Give Loop a moment to start
	time.Sleep(50 * time.Millisecond)

	// Send SIGUSR1 to ourselves
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.Fail(t, "Loop did not exit after signal")
	}

	require.Equal(t, syscall.SIGUSR1, received, "handler should have received SIGUSR1")

	// After Loop exits, send another signal — it should NOT be delivered
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	require.Never(t, func() bool {
		select {
		case _, ok := <-h.sigCh:
			return ok
		default:
			return false
		}
	}, 100*time.Millisecond, 10*time.Millisecond,
		"signal was delivered to channel after Loop returned — signal.Stop was not called")
}
