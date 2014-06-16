package peco

import (
	"encoding/json"
	"fmt"
	"os"
	"unicode"

	"github.com/nsf/termbox-go"
)

type KeymapHandler func(*Input, termbox.Event)
type Keymap map[termbox.Key]KeymapHandler
type KeymapStringKey string

// This map is populated using some magic numbers, which must match
// the values defined in termbox-go. Verification against the actual
// termbox constants are done in the test
var stringToKey = map[string]termbox.Key{}

func init() {
	fidx := 12
	for k := termbox.KeyF1; k > termbox.KeyF12; k-- {
		sk := fmt.Sprintf("F%d", fidx)
		stringToKey[sk] = k
		fidx--
	}

	names := []string{
		"Insert",
		"Delete",
		"Home",
		"End",
		"Pgup",
		"Pgdn",
		"ArrowUp",
		"ArrowDown",
		"ArrowLeft",
		"ArrowRight",
	}
	for i, n := range names {
		stringToKey[n] = termbox.Key(int(termbox.KeyF12) - (i + 1))
	}

	names = []string{
		"Left",
		"Middle",
		"Right",
	}
	for i, n := range names {
		sk := fmt.Sprintf("Mouse%s", n)
		stringToKey[sk] = termbox.Key(int(termbox.KeyArrowRight) - (i + 2))
	}

	whacky := [][]string{
		{"~", "2", "Space"},
		{"a"},
		{"b"},
		{"c"},
		{"d"},
		{"e"},
		{"f"},
		{"g"},
		{"h"},
		{"i"},
		{"j"},
		{"k"},
		{"l"},
		{"m"},
		{"n"},
		{"o"},
		{"p"},
		{"q"},
		{"r"},
		{"s"},
		{"t"},
		{"u"},
		{"v"},
		{"w"},
		{"x"},
		{"y"},
		{"z"},
		{"[", "3"},
		{"4", "\\"},
		{"5", "]"},
		{"6"},
		{"7", "/", "_"},
	}
	for i, list := range whacky {
		for _, n := range list {
			sk := fmt.Sprintf("C-%s", n)
			stringToKey[sk] = termbox.Key(int(termbox.KeyCtrlTilde) + i)
		}
	}

	stringToKey["BS"] = termbox.KeyBackspace
	stringToKey["Tab"] = termbox.KeyTab
	stringToKey["Enter"] = termbox.KeyEnter
	stringToKey["Esc"] = termbox.KeyEsc
	stringToKey["Space"] = termbox.KeySpace
	stringToKey["BS2"] = termbox.KeyBackspace2
	stringToKey["C-8"] = termbox.KeyCtrl8

	//	panic(fmt.Sprintf("%#q", stringToKey))
}

func handleAcceptChar(i *Input, ev termbox.Event) {
	if ev.Key == termbox.KeySpace {
		ev.Ch = ' '
	}

	if ev.Ch > 0 {
		if len(i.query) == i.caretPos {
			i.query = append(i.query, ev.Ch)
		} else {
			buf := make([]rune, len(i.query)+1)
			copy(buf, i.query[:i.caretPos])
			buf[i.caretPos] = ev.Ch
			copy(buf[i.caretPos+1:], i.query[i.caretPos:])
			i.query = buf
		}
		i.caretPos++
		i.ExecQuery(string(i.query))
	}
}

// peco.Finish -> end program, exit with success
func handleFinish(i *Input, _ termbox.Event) {
	// Must end with all the selected lines.
	i.selection.Add(i.currentLine)

	i.result = []string{}
	for _, lineno := range i.selection {
		i.result = append(i.result, i.current[lineno-1].Line())
	}
	i.Finish()
}

func handleToggleSelection(i *Input, _ termbox.Event) {
	if i.selection.Has(i.currentLine) {
		i.selection.Remove(i.currentLine)
		return
	}
	i.selection.Add(i.currentLine)
}

// peco.Cancel -> end program, exit with failure
func handleCancel(i *Input, ev termbox.Event) {
	i.ExitStatus = 1
	i.Finish()
}

func handleSelectPrevious(i *Input, ev termbox.Event) {
	i.PagingCh() <- ToPrevLine
	i.DrawMatches(nil)
}

func handleSelectNext(i *Input, ev termbox.Event) {
	i.PagingCh() <- ToNextLine
	i.DrawMatches(nil)
}

func handleSelectPreviousPage(i *Input, ev termbox.Event) {
	i.PagingCh() <- ToPrevPage
	i.DrawMatches(nil)
}

func handleSelectNextPage(i *Input, ev termbox.Event) {
	i.PagingCh() <- ToNextPage
	i.DrawMatches(nil)
}

func handleRotateMatcher(i *Input, ev termbox.Event) {
	i.Ctx.CurrentMatcher++
	if i.Ctx.CurrentMatcher >= len(i.Ctx.Matchers) {
		i.Ctx.CurrentMatcher = 0
	}
	if len(i.query) > 0 {
		i.ExecQuery(string(i.query))
		return
	}
	i.DrawMatches(nil)
}

func handleForwardWord(i *Input, _ termbox.Event) {
	if i.caretPos >= len(i.query) {
		return
	}

	foundSpace := false
	for pos := i.caretPos; pos < len(i.query); pos++ {
		r := i.query[pos]
		if foundSpace {
			if !unicode.IsSpace(r) {
				i.caretPos = pos
				i.DrawMatches(nil)
				return
			}
		} else {
			if unicode.IsSpace(r) {
				foundSpace = true
			}
		}
	}

	// not found. just move to the end of the buffer
	i.caretPos = len(i.query)
	i.DrawMatches(nil)

}

func handleBackwardWord(i *Input, _ termbox.Event) {
	if i.caretPos == 0 {
		return
	}

	if i.caretPos >= len(i.query) {
		i.caretPos--
	}

	// if we start from a whitespace-ish position, we should
	// rewind to the end of the previous word, and then do the
	// search all over again
SEARCH_PREV_WORD:
	if unicode.IsSpace(i.query[i.caretPos]) {
		for pos := i.caretPos; pos > 0; pos-- {
			if !unicode.IsSpace(i.query[pos]) {
				i.caretPos = pos
				break
			}
		}
	}

	// if we start from the first character of a word, we
	// should attempt to move back and search for the previous word
	if i.caretPos > 0 && unicode.IsSpace(i.query[i.caretPos-1]) {
		i.caretPos--
		goto SEARCH_PREV_WORD
	}

	// Now look for a space
	for pos := i.caretPos; pos > 0; pos-- {
		if unicode.IsSpace(i.query[pos]) {
			i.caretPos = pos + 1
			i.DrawMatches(nil)
			return
		}
	}

	// not found. just move to the beginning of the buffer
	i.caretPos = 0
	i.DrawMatches(nil)
}

func handleForwardChar(i *Input, _ termbox.Event) {
	if i.caretPos >= len(i.query) {
		return
	}
	i.caretPos++
	i.DrawMatches(nil)
}

func handleBackwardChar(i *Input, _ termbox.Event) {
	if i.caretPos <= 0 {
		return
	}
	i.caretPos--
	i.DrawMatches(nil)
}

func handleBeginningOfLine(i *Input, _ termbox.Event) {
	i.caretPos = 0
	i.DrawMatches(nil)
}

func handleEndOfLine(i *Input, _ termbox.Event) {
	i.caretPos = len(i.query)
	i.DrawMatches(nil)
}

func handleEndOfFile(i *Input, ev termbox.Event) {
	if len(i.query) > 0 {
		handleDeleteForwardChar(i, ev)
	} else {
		handleCancel(i, ev)
	}
}

func handleKillBeginOfLine(i *Input, _ termbox.Event) {
	i.query = i.query[i.caretPos:]
	i.caretPos = 0
	if len(i.query) > 0 {
		i.ExecQuery(string(i.query))
		return
	}
	i.current = nil
	i.DrawMatches(nil)
}

func handleKillEndOfLine(i *Input, _ termbox.Event) {
	if len(i.query) <= i.caretPos {
		return
	}

	i.query = i.query[0:i.caretPos]
	if len(i.query) > 0 {
		i.ExecQuery(string(i.query))
		return
	}
	i.current = nil
	i.DrawMatches(nil)
}

func handleDeleteAll(i *Input, _ termbox.Event) {
	i.query = make([]rune, 0)
	i.current = nil
	i.DrawMatches(nil)
}

func handleDeleteForwardChar(i *Input, _ termbox.Event) {
	if len(i.query) <= i.caretPos {
		return
	}

	buf := make([]rune, len(i.query)-1)
	copy(buf, i.query[:i.caretPos])
	copy(buf[i.caretPos:], i.query[i.caretPos+1:])
	i.query = buf
	if len(i.query) > 0 {
		i.ExecQuery(string(i.query))
		return
	}

	i.current = nil
	i.DrawMatches(nil)
}

func handleDeleteBackwardChar(i *Input, ev termbox.Event) {
	if len(i.query) <= 0 {
		return
	}

	switch i.caretPos {
	case 0:
		// No op
		return
	case len(i.query):
		i.query = i.query[:len(i.query)-1]
	default:
		buf := make([]rune, len(i.query)-1)
		copy(buf, i.query[:i.caretPos])
		copy(buf[i.caretPos-1:], i.query[i.caretPos:])
		i.query = buf
	}
	i.caretPos--
	if len(i.query) > 0 {
		i.ExecQuery(string(i.query))
		return
	}

	i.current = nil
	i.DrawMatches(nil)
}

func handleDeleteForwardWord(i *Input, _ termbox.Event) {
	if len(i.query) <= i.caretPos {
		return
	}

	for pos := i.caretPos; pos < len(i.query); pos++ {
		if pos == len(i.query)-1 {
			i.query = i.query[:i.caretPos]
			break
		}

		if unicode.IsSpace(i.query[pos]) {
			buf := make([]rune, len(i.query)-(pos-i.caretPos))
			copy(buf, i.query[:i.caretPos])
			copy(buf[i.caretPos:], i.query[pos:])
			i.query = buf
			break
		}
	}

	if len(i.query) > 0 {
		i.ExecQuery(string(i.query))
		return
	}

	i.current = nil
	i.DrawMatches(nil)
}

func handleDeleteBackwardWord(i *Input, _ termbox.Event) {
	if i.caretPos == 0 {
		return
	}

	for pos := i.caretPos - 1; pos >= 0; pos-- {
		if pos == 0 {
			i.query = i.query[i.caretPos:]
			break
		}

		if unicode.IsSpace(i.query[pos]) {
			buf := make([]rune, len(i.query)-(i.caretPos-pos))
			copy(buf, i.query[:pos])
			copy(buf[pos:], i.query[i.caretPos:])
			i.query = buf
			i.caretPos = pos
			break
		}
	}

	if len(i.query) > 0 {
		i.ExecQuery(string(i.query))
		return
	}

	i.current = nil
	i.DrawMatches(nil)
}

func (ksk KeymapStringKey) ToKey() (k termbox.Key, err error) {
	k, ok := stringToKey[string(ksk)]
	if !ok {
		err = fmt.Errorf("No such key %s", ksk)
	}
	return
}

var handlers = map[string]KeymapHandler{
	"peco.KillEndOfLine":      handleKillEndOfLine,
	"peco.DeleteAll":          handleDeleteAll,
	"peco.BeginningOfLine":    handleBeginningOfLine,
	"peco.EndOfLine":          handleEndOfLine,
	"peco.EndOfFile":          handleEndOfFile,
	"peco.ForwardChar":        handleForwardChar,
	"peco.BackwardChar":       handleBackwardChar,
	"peco.ForwardWord":        handleForwardWord,
	"peco.BackwardWord":       handleBackwardWord,
	"peco.DeleteForwardChar":  handleDeleteForwardChar,
	"peco.DeleteBackwardChar": handleDeleteBackwardChar,
	"peco.DeleteForwardWord":  handleDeleteForwardWord,
	"peco.DeleteBackwardWord": handleDeleteBackwardWord,
	"peco.SelectPreviousPage": handleSelectPreviousPage,
	"peco.SelectNextPage":     handleSelectNextPage,
	"peco.SelectPrevious":     handleSelectPrevious,
	"peco.SelectNext":         handleSelectNext,
	"peco.ToggleSelection":    handleToggleSelection,
	"peco.RotateMatcher":      handleRotateMatcher,
	"peco.Finish":             handleFinish,
	"peco.Cancel":             handleCancel,
}

func NewKeymap() Keymap {
	return Keymap{
		termbox.KeyEsc:        handleCancel,
		termbox.KeyEnter:      handleFinish,
		termbox.KeyArrowUp:    handleSelectPrevious,
		termbox.KeyCtrlP:      handleSelectPrevious,
		termbox.KeyArrowDown:  handleSelectNext,
		termbox.KeyCtrlN:      handleSelectNext,
		termbox.KeyArrowLeft:  handleSelectPreviousPage,
		termbox.KeyArrowRight: handleSelectNextPage,
		termbox.KeyCtrlD:      handleDeleteForwardChar,
		termbox.KeyBackspace:  handleDeleteBackwardChar,
		termbox.KeyBackspace2: handleDeleteBackwardChar,
		termbox.KeyCtrlW:      handleDeleteBackwardWord,
		termbox.KeyCtrlA:      handleBeginningOfLine,
		termbox.KeyCtrlE:      handleEndOfLine,
		termbox.KeyCtrlF:      handleForwardChar,
		termbox.KeyCtrlB:      handleBackwardChar,
		termbox.KeyCtrlK:      handleKillEndOfLine,
		termbox.KeyCtrlU:      handleKillBeginOfLine,
		termbox.KeyCtrlR:      handleRotateMatcher,
	}
}

func (km Keymap) Handler(k termbox.Key) KeymapHandler {
	h, ok := km[k]
	if ok {
		return h
	}
	return handleAcceptChar
}

func (km Keymap) UnmarshalJSON(buf []byte) error {
	raw := map[string]string{}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return err
	}

	for ks, vs := range raw {
		k, err := KeymapStringKey(ks).ToKey()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unknown key %s", ks)
			continue
		}

		v, ok := handlers[vs]
		if !ok {
			fmt.Fprintf(os.Stderr, "Unknown handler %s", vs)
			continue
		}

		km[k] = v
	}

	return nil
}
