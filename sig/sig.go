package sig

import (
	"os"
	"os/signal"
	"syscall"

	"context"
)

type ReceivedHandler interface {
	Handle(os.Signal)
}

type ReceivedHandlerFunc func(os.Signal)

// Handle calls the underlying function with the received signal.
func (s ReceivedHandlerFunc) Handle(sig os.Signal) {
	s(sig)
}

type Handler struct {
	onSignalReceived ReceivedHandler
	sigCh            chan os.Signal
}

// New creates a new signal handler that forwards the specified signals (default: SIGTERM, SIGINT, SIGHUP) to h.
func New(h ReceivedHandler, sigs ...os.Signal) *Handler {
	if len(sigs) == 0 {
		sigs = append(sigs, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sigs...)

	return &Handler{
		onSignalReceived: h,
		sigCh:            ch,
	}
}

// Loop listens for OS signals and invokes the handler when one is received, then returns.
func (h *Handler) Loop(ctx context.Context, cancel func()) error {
	defer cancel()
	defer signal.Stop(h.sigCh)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sig := <-h.sigCh:
			h.onSignalReceived.Handle(sig)
			return nil
		}
	}
}
