package sighandler

import (
	"os"
	"os/signal"
)

type Handler struct {
	EndFunc            func()               // Called once when Loop() exits
	SignalReceivedFunc func(os.Signal) bool // Called each time when signal is received
	sigCh              chan os.Signal
}

func New(signals ...os.Signal) *Handler {
	sigCh := make(chan os.Signal, len(signals))
	signal.Notify(sigCh, signals...)
	return &Handler{sigCh: sigCh}
}

func (s *Handler) runEndFunc() {
	if f := s.EndFunc; f != nil {
		f()
	}
}

func (s *Handler) runSignalReceivedFunc(sig os.Signal) bool {
	if f := s.SignalReceivedFunc; f != nil {
		return f(sig)
	}
	return false
}

// Loop loops until we are told that we are done vi the given channel,
// or when a signal is received and the function returns false
// TODO: replace channel with context.Context
func (s *Handler) Loop(loopCh chan struct{}) {
	defer s.runEndFunc()

	for {
		select {
		case <-loopCh:
			return
		case sig := <-s.sigCh:
			if !s.runSignalReceivedFunc(sig) {
				return
			}
		}
	}
}
