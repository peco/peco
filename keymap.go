package peco

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nsf/termbox-go"
)

type KeymapHandler func(*Input, termbox.Event)
type Keymap map[termbox.Key]KeymapHandler
type KeymapStringKey string
type KeymapStringHandler string

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
	if len(i.current) == 1 {
		i.result = i.current[0].line
	} else if i.selectedLine > 0 && i.selectedLine < len(i.current) {
		i.result = i.current[i.selectedLine-1].line
	}
	i.Finish()
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

func handleDeleteBackwardChar(i *Input, ev termbox.Event) {
	if len(i.query) <= 0 {
		return
	}

	if i.caretPos == len(i.query) {
		i.query = i.query[:len(i.query)-1]
	} else {
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

func (ksk KeymapStringKey) ToKey() (k termbox.Key, err error) {
	k, ok := stringToKey[string(ksk)]
	if !ok {
		err = fmt.Errorf("No such key %s", ksk)
	}
	return
}

func (ksh KeymapStringHandler) ToHandler() (h KeymapHandler, err error) {
	switch ksh {
	case "peco.ForwardChar":
		h = handleForwardChar
	case "peco.BackwardChar":
		h = handleBackwardChar
	case "peco.DeleteBackwardChar":
		h = handleDeleteBackwardChar
	case "peco.SelectPreviousPage":
		h = handleSelectPreviousPage
	case "peco.SelectNextPage":
		h = handleSelectNextPage
	case "peco.SelectPrevious":
		h = handleSelectPrevious
	case "peco.SelectNext":
		h = handleSelectNext
	case "peco.Finish":
		h = handleFinish
	case "peco.Cancel":
		h = handleCancel
	default:
		err = fmt.Errorf("No such handler %s", ksh)
	}
	return
}

func NewKeymap() Keymap {
	return Keymap{
		termbox.KeyEsc:        handleCancel,
		termbox.KeyEnter:      handleFinish,
		termbox.KeyArrowUp:    handleSelectPrevious,
		termbox.KeyCtrlK:      handleSelectPrevious,
		termbox.KeyArrowDown:  handleSelectNext,
		termbox.KeyCtrlJ:      handleSelectNext,
		termbox.KeyArrowLeft:  handleSelectPreviousPage,
		termbox.KeyArrowRight: handleSelectNextPage,
		termbox.KeyBackspace:  handleDeleteBackwardChar,
		termbox.KeyBackspace2: handleDeleteBackwardChar,
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

		v, err := KeymapStringHandler(vs).ToHandler()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unknown handler %s", vs)
			continue
		}

		km[k] = v
	}

	return nil
}
