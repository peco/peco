package sig

import (
	"os"
	"os/signal"
	"syscall"

	"context"
)

type SigReceivedHandler interface {
	Handle(os.Signal)
}

type SigReceivedHandlerFunc func(os.Signal)

func (s SigReceivedHandlerFunc) Handle(sig os.Signal) {
	s(sig)
}

type Handler struct {
	onSignalReceived SigReceivedHandler
	sigCh            chan os.Signal
}

func New(h SigReceivedHandler, sigs ...os.Signal) *Handler {
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

func (h *Handler) Loop(ctx context.Context, cancel func()) error {
	defer cancel()

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
