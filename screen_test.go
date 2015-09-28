package peco

import (
	"testing"

	"github.com/nsf/termbox-go"
)

func TestTermbox(t *testing.T) {
	var (
		oTermboxClose     = termboxClose
		oTermboxFlush     = termboxFlush
		oTermboxInit      = termboxInit
		oTermboxPollEvent = termboxPollEvent
		oTermboxSetCell   = termboxSetCell
		oTermboxSize      = termboxSize
	)
	defer func() {
		termboxClose = oTermboxClose
		termboxFlush = oTermboxFlush
		termboxInit = oTermboxInit
		termboxPollEvent = oTermboxPollEvent
		termboxSetCell = oTermboxSetCell
		termboxSize = oTermboxSize
	}()

	termboxCloseCalled := 0
	termboxClose = func() { termboxCloseCalled++ }

	termboxInitCalled := 0
	termboxInit = func() error {
		termboxInitCalled++
		return nil
	}

	termboxFlushCalled := 0
	termboxFlush = func() error {
		termboxFlushCalled++
		return nil
	}

	termboxPollEventCalled := 0
	termboxPollEvent = func() termbox.Event {
		termboxPollEventCalled++
		return termbox.Event{}
	}

	termboxSetCellCalled := 0
	termboxSetCell = func(_, _ int, _ rune, _, _ termbox.Attribute) {
		termboxSetCellCalled++
	}

	termboxSizeCalled := 0
	termboxSize = func() (int, int) {
		termboxSizeCalled++
		return 0, 0
	}

	func() {
		screen.Init()
		defer screen.Close()

		screen.SetCell(0, 0, 'a', termbox.ColorDefault, termbox.ColorDefault)
		screen.Flush()

		evCh := screen.PollEvent()
		_ = <-evCh

		_, _ = screen.Size()
	}()

	if termboxInitCalled != 1 {
		t.Errorf("termbox.Init was called %d times (expected 1)", termboxInitCalled)
	}
	if termboxCloseCalled != 1 {
		t.Errorf("termbox.Close was called %d times (expected 1)", termboxCloseCalled)
	}
	if termboxSetCellCalled != 1 {
		t.Errorf("termbox.SetCell was called %d times (expected 1)", termboxSetCellCalled)
	}
	if termboxFlushCalled != 1 {
		t.Errorf("termbox.Flush was called %d times (expected 1)", termboxFlushCalled)
	}
	if termboxPollEventCalled != 2 {
		t.Errorf("termbox.PollEvent was called %d times (expected 2)", termboxPollEventCalled)
	}
	if termboxSizeCalled != 1 {
		t.Errorf("termbox.Size was called %d times (expected 1)", termboxSizeCalled)
	}
}