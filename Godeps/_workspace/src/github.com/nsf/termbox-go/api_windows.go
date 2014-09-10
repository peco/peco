package termbox

import (
	"os"
	"syscall"
)

// public API

// Initializes termbox library. This function should be called before any other functions.
// After successful initialization, the library must be finalized using 'Close' function.
//
// Example usage:
//      err := termbox.Init()
//      if err != nil {
//              panic(err)
//      }
//      defer termbox.Close()
func Init() error {
	var err error

	in, err = syscall.GetStdHandle(syscall.STD_INPUT_HANDLE)
	if err != nil {
		return err
	}
	out, err = syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil {
		return err
	}

	err = get_console_mode(in, &orig_mode)
	if err != nil {
		return err
	}

	consolewin = false
	if fin, err := os.Open("CONIN$"); err == nil {
		defer fin.Close()
		consolewin = in == syscall.Handle(fin.Fd())
		if consolewin {
			err = set_console_mode(in, enable_window_input)
			if err != nil {
				return err
			}
		}
	}

	orig_screen = out
	out, err = create_console_screen_buffer()
	if err != nil {
		return err
	}
	w, h := get_win_size(out)

	err = set_console_screen_buffer_size(out, coord{short(w), short(h)})
	if err != nil {
		return err
	}

	err = set_console_active_screen_buffer(out)
	if err != nil {
		return err
	}

	show_cursor(false)
	termw, termh = get_term_size(out)
	back_buffer.init(termw, termh)
	front_buffer.init(termw, termh)
	back_buffer.clear()
	front_buffer.clear()
	clear()

	attrsbuf = make([]word, 0, termw*termh)
	charsbuf = make([]wchar, 0, termw*termh)
	diffbuf = make([]diff_msg, 0, 32)

	go input_event_producer()

	return nil
}

// Finalizes termbox library, should be called after successful initialization
// when termbox's functionality isn't required anymore.
func Close() {
	// we ignore errors here, because we can't really do anything about them
	set_console_mode(in, orig_mode)
	set_console_active_screen_buffer(orig_screen)
	syscall.Close(out)
}

// Synchronizes the internal back buffer with the terminal.
func Flush() error {
	update_size_maybe()
	prepare_diff_messages()
	for _, msg := range diffbuf {
		write_console_output_attribute(out, msg.attrs, msg.pos)
		write_console_output_character(out, msg.chars, msg.pos)
	}
	if !is_cursor_hidden(cursor_x, cursor_y) {
		move_cursor(cursor_x, cursor_y)
	}
	return nil
}

// Sets the position of the cursor. See also HideCursor().
func SetCursor(x, y int) {
	if is_cursor_hidden(cursor_x, cursor_y) && !is_cursor_hidden(x, y) {
		show_cursor(true)
	}

	if !is_cursor_hidden(cursor_x, cursor_y) && is_cursor_hidden(x, y) {
		show_cursor(false)
	}

	cursor_x, cursor_y = x, y
	if !is_cursor_hidden(cursor_x, cursor_y) {
		move_cursor(cursor_x, cursor_y)
	}
}

// The shortcut for SetCursor(-1, -1).
func HideCursor() {
	SetCursor(cursor_hidden, cursor_hidden)
}

// Changes cell's parameters in the internal back buffer at the specified
// position.
func SetCell(x, y int, ch rune, fg, bg Attribute) {
	if x < 0 || x >= back_buffer.width {
		return
	}
	if y < 0 || y >= back_buffer.height {
		return
	}

	back_buffer.cells[y*back_buffer.width+x] = Cell{ch, fg, bg}
}

// Returns a slice into the termbox's back buffer. You can get its dimensions
// using 'Size' function. The slice remains valid as long as no 'Clear' or
// 'Flush' function calls were made after call to this function.
func CellBuffer() []Cell {
	return back_buffer.cells
}

// Wait for an event and return it. This is a blocking function call.
func PollEvent() Event {
	return <-input_comm
}

// Returns the size of the internal back buffer (which is the same as
// terminal's window size in characters).
func Size() (int, int) {
	return termw, termh
}

// Clears the internal back buffer.
func Clear(fg, bg Attribute) error {
	foreground, background = fg, bg
	update_size_maybe()
	back_buffer.clear()
	return nil
}

// Sets termbox input mode. Termbox has two input modes:
//
// 1. Esc input mode. When ESC sequence is in the buffer and it doesn't match
// any known sequence. ESC means KeyEsc. This is the default input mode.
//
// 2. Alt input mode. When ESC sequence is in the buffer and it doesn't match
// any known sequence. ESC enables ModAlt modifier for the next keyboard event.
//
// Both input modes can be OR'ed with Mouse mode. Setting Mouse mode bit up will
// enable mouse button click events.
//
// If 'mode' is InputCurrent, returns the current input mode. See also Input*
// constants.
func SetInputMode(mode InputMode) InputMode {
	if mode == InputCurrent {
		return input_mode
	}
	if consolewin {
		if mode&InputMouse != 0 {
			err := set_console_mode(in, enable_window_input|enable_mouse_input)
			if err != nil {
				panic(err)
			}
		} else {
			err := set_console_mode(in, enable_window_input)
			if err != nil {
				panic(err)
			}
		}
	}

	input_mode = mode
	return input_mode
}

// Sync comes handy when something causes desync between termbox's understanding
// of a terminal buffer and the reality. Such as a third party process. Sync
// forces a complete resync between the termbox and a terminal, it may not be
// visually pretty though. At the moment on Windows it does nothing.
func Sync() error {
	return nil
}
