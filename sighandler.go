package peco

import (
	"os"
	"os/signal"
	"syscall"
)

type signalHandler struct {
	loopCh           chan struct{}
	onEnd            func()
	onSignalReceived func()
	sigCh            chan os.Signal
}

func NewSignalHandler(loopCh chan struct{}, onEnd, onSignalReceived func()) *signalHandler {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	return &signalHandler{
		loopCh:           loopCh,
		onEnd:            onEnd,
		onSignalReceived: onSignalReceived,
		sigCh:            sigCh,
	}
}

func (s *signalHandler) Loop() {
	defer s.onEnd()

	for {
		select {
		case <-s.loopCh:
			return
		case <-s.sigCh:
			s.onSignalReceived()
			return
		}
	}
}
