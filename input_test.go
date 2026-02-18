package peco

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/peco/peco/internal/keyseq"
	"github.com/stretchr/testify/require"
)

func TestInputModifierKeyRace(t *testing.T) {
	// This test verifies that the generation counter in handleInputEvent
	// prevents stale Alt-key timer callbacks from firing a spurious Esc
	// action when Stop() fails to cancel an already-queued timer.

	t.Run("esc followed by char cancels timer", func(t *testing.T) {
		var escCount int64

		// Register a test action for Esc so we can count invocations.
		testAction := ActionFunc(func(_ context.Context, _ *Peco, _ Event) {
			atomic.AddInt64(&escCount, 1)
		})
		nameToActions["peco.TestEscRecord"] = testAction
		t.Cleanup(func() { delete(nameToActions, "peco.TestEscRecord") })

		state := New()
		state.hub = nullHub{}
		state.config.Keymap = map[string]string{
			"Esc": "peco.TestEscRecord",
		}
		state.config.Action = map[string][]string{}
		require.NoError(t, state.populateKeymap())

		ctx := context.Background()
		input := NewInput(state, state.Keymap(), make(chan Event))

		// Send Esc event — starts the 50ms timer
		escEv := Event{Type: EventKey, Key: keyseq.KeyEsc, Ch: 0}
		input.handleInputEvent(ctx, escEv)

		// Verify timer was created and generation was bumped
		input.mutex.Lock()
		require.NotNil(t, input.mod, "timer should be set after Esc")
		gen := input.modGen
		require.True(t, gen > 0, "generation should be incremented")
		input.mutex.Unlock()

		// Send a char event immediately — should cancel the timer
		charEv := Event{Type: EventKey, Key: 0, Ch: 'x'}
		input.handleInputEvent(ctx, charEv)

		// Verify timer was stopped and generation was bumped
		input.mutex.Lock()
		require.Nil(t, input.mod, "timer should be nil after char event")
		require.Greater(t, input.modGen, gen, "generation should be incremented again")
		input.mutex.Unlock()

		// The Esc action must NOT have fired because the timer was cancelled.
		// Before the generation counter fix, a stale timer callback could
		// still call ExecuteAction after Stop() returned false.
		// Use require.Never to verify the count stays 0 well past the timer duration.
		require.Never(t, func() bool {
			return atomic.LoadInt64(&escCount) != 0
		}, 200*time.Millisecond, 10*time.Millisecond,
			"Esc action should not fire when followed by an immediate key")
	})

	t.Run("esc alone fires after timeout", func(t *testing.T) {
		var escCount int64

		testAction := ActionFunc(func(_ context.Context, _ *Peco, _ Event) {
			atomic.AddInt64(&escCount, 1)
		})
		nameToActions["peco.TestEscRecord2"] = testAction
		t.Cleanup(func() { delete(nameToActions, "peco.TestEscRecord2") })

		state := New()
		state.hub = nullHub{}
		state.config.Keymap = map[string]string{
			"Esc": "peco.TestEscRecord2",
		}
		state.config.Action = map[string][]string{}
		require.NoError(t, state.populateKeymap())

		ctx := t.Context()
		input := NewInput(state, state.Keymap(), make(chan Event))

		// Start a goroutine that drains pendingEsc and executes actions,
		// simulating what Loop does.
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case ev := <-input.pendingEsc:
					input.state.Keymap().ExecuteAction(ctx, input.state, ev)
				}
			}
		}()

		// Send Esc event — starts the 50ms timer
		escEv := Event{Type: EventKey, Key: keyseq.KeyEsc, Ch: 0}
		input.handleInputEvent(ctx, escEv)

		// Wait for the timer to fire (50ms timer) — use Eventually to poll
		// instead of a fixed sleep.
		require.Eventually(t, func() bool {
			return atomic.LoadInt64(&escCount) == 1
		}, 2*time.Second, 10*time.Millisecond,
			"Esc action should fire once when no follow-up key arrives")
	})
}
