package peco

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/peco/peco/internal/keyseq"
	"github.com/stretchr/testify/require"
)

func TestInputDispatchesEscImmediately(t *testing.T) {
	// After removing the application-layer Esc timer (tcell handles
	// Esc/Alt disambiguation), verify that a bare Esc event is
	// dispatched immediately without delay.

	var escCount int64

	testAction := ActionFunc(func(_ context.Context, _ *Peco, _ Event) {
		atomic.AddInt64(&escCount, 1)
	})
	nameToActions["peco.TestEscImmediate"] = testAction
	t.Cleanup(func() { delete(nameToActions, "peco.TestEscImmediate") })

	state := New()
	state.hub = nullHub{}
	state.config.Keymap = map[string]string{
		"Esc": "peco.TestEscImmediate",
	}
	state.config.Action = map[string][]string{}
	require.NoError(t, state.populateKeymap())

	ctx := context.Background()
	input := NewInput(state, state.Keymap(), make(chan Event))

	escEv := Event{Type: EventKey, Key: keyseq.KeyEsc, Ch: 0}
	input.handleInputEvent(ctx, escEv)

	require.Equal(t, int64(1), atomic.LoadInt64(&escCount),
		"Esc action should fire immediately without timer delay")
}

func TestInputAltKeyPassthrough(t *testing.T) {
	// Verify that events arriving with ModAlt already set by tcell
	// are dispatched correctly.

	var gotMod keyseq.ModifierKey

	testAction := ActionFunc(func(_ context.Context, _ *Peco, ev Event) {
		gotMod = ev.Mod
	})
	nameToActions["peco.TestAltRecord"] = testAction
	t.Cleanup(func() { delete(nameToActions, "peco.TestAltRecord") })

	state := New()
	state.hub = nullHub{}
	state.config.Keymap = map[string]string{
		"M-f": "peco.TestAltRecord",
	}
	state.config.Action = map[string][]string{}
	require.NoError(t, state.populateKeymap())

	ctx := context.Background()
	input := NewInput(state, state.Keymap(), make(chan Event))

	// Simulate tcell delivering Alt+f with ModAlt already set
	altEv := Event{Type: EventKey, Key: 0, Ch: 'f', Mod: keyseq.ModAlt}
	input.handleInputEvent(ctx, altEv)

	require.Equal(t, keyseq.ModAlt, gotMod,
		"ModAlt should be preserved from tcell event")
}
