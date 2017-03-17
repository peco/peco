package peco

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lestrrat/go-pdebug"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
	"github.com/peco/peco/line"
	"github.com/pkg/errors"
)

var extraOffset int = 0

// IsValidLayoutType checks if a string is a supported layout type
func IsValidLayoutType(v LayoutType) bool {
	return v == LayoutTypeTopDown || v == LayoutTypeBottomUp
}

// IsValidVerticalAnchor checks if the specified anchor is supported
func IsValidVerticalAnchor(anchor VerticalAnchor) bool {
	return anchor == AnchorTop || anchor == AnchorBottom
}

// Utility function
func mergeAttribute(a, b termbox.Attribute) termbox.Attribute {
	if a&0x0F == 0 || b&0x0F == 0 {
		return a | b
	}
	return ((a - 1) | (b - 1)) + 1
}

// NewAnchorSettings creates a new AnchorSetting struct. Panics if
// an unknown VerticalAnchor is sent
func NewAnchorSettings(screen Screen, anchor VerticalAnchor, offset int) *AnchorSettings {
	if !IsValidVerticalAnchor(anchor) {
		panic("Invalid vertical anchor specified")
	}

	return &AnchorSettings{
		anchor:       anchor,
		anchorOffset: offset,
		screen:       screen,
	}
}

// AnchorPosition returns the starting y-offset, based on the
// anchor type and offset
func (as AnchorSettings) AnchorPosition() int {
	var pos int
	switch as.anchor {
	case AnchorTop:
		pos = as.anchorOffset
	case AnchorBottom:
		_, h := as.screen.Size()
		pos = int(h) - as.anchorOffset - 1 // -1 is required because y is 0 base, but h is 1 base
	default:
		panic("Unknown anchor type!")
	}

	return pos
}

// NewUserPrompt creates a new UserPrompt struct
func NewUserPrompt(screen Screen, anchor VerticalAnchor, anchorOffset int, prompt string, styles *StyleSet) *UserPrompt {
	if len(prompt) <= 0 { // default
		prompt = "QUERY>"
	}
	promptLen := runewidth.StringWidth(prompt)

	return &UserPrompt{
		AnchorSettings: NewAnchorSettings(screen, anchor, anchorOffset),
		prompt:         prompt,
		promptLen:      int(promptLen),
		styles:         styles,
	}
}

// Draw draws the query prompt
func (u UserPrompt) Draw(state *Peco) {
	if pdebug.Enabled {
		g := pdebug.Marker("UserPrompt.Draw")
		defer g.End()
	}

	location := u.AnchorPosition()

	// print "QUERY>"
	u.screen.Print(PrintArgs{
		Y:   location,
		Fg:  u.styles.Basic.fg,
		Bg:  u.styles.Basic.bg,
		Msg: u.prompt,
	})

	c := state.Caret()
	if c.Pos() <= 0 { // XXX Do we really need this?
		c.SetPos(0) // sanity
	}

	q := state.Query()
	qs := q.String()
	ql := q.Len()
	if c.Pos() > ql { // XXX Do we really need this?
		c.SetPos(ql)
	}

	fg := u.styles.Query.fg
	bg := u.styles.Query.bg
	switch ql {
	case 0:
		u.screen.Print(PrintArgs{
			X:    u.promptLen,
			Y:    location,
			Fg:   fg,
			Bg:   bg,
			Fill: true,
		})
		u.screen.Print(PrintArgs{
			X:    u.promptLen + 1,
			Y:    location,
			Bg:   bg | termbox.AttrReverse,
			Fg:   fg | termbox.AttrReverse,
			Msg:  " ",
			Fill: false,
		})
	case c.Pos():
		// the entire string + the caret after the string
		u.screen.Print(PrintArgs{
			X:    u.promptLen,
			Y:    location,
			Fg:   fg,
			Bg:   bg,
			Fill: true,
		})
		u.screen.Print(PrintArgs{
			X:    u.promptLen + 1,
			Y:    location,
			Fg:   fg,
			Bg:   bg,
			Msg:  qs,
			Fill: false,
		})
		u.screen.Print(PrintArgs{
			X:    u.promptLen + 1 + int(runewidth.StringWidth(qs)),
			Y:    location,
			Fg:   fg | termbox.AttrReverse,
			Bg:   bg | termbox.AttrReverse,
			Msg:  " ",
			Fill: false,
		})
	default:
		// the caret is in the middle of the string
		prev := int(0)
		var i int
		for r := range q.Runes() {
			fg := u.styles.Query.fg
			bg := u.styles.Query.bg
			if i == c.Pos() {
				fg |= termbox.AttrReverse
				bg |= termbox.AttrReverse
			}
			u.screen.SetCell(int(u.promptLen+1+prev), int(location), r, fg, bg)
			prev += int(runewidth.RuneWidth(r))
			i++
		}
		fg := u.styles.Query.fg
		bg := u.styles.Query.bg
		u.screen.Print(PrintArgs{
			X:    u.promptLen + prev + 1,
			Y:    location,
			Fg:   fg,
			Bg:   bg,
			Fill: true,
		})
	}

	width, _ := u.screen.Size()

	loc := state.Location()
	pmsg := fmt.Sprintf("%s [%d (%d/%d)]", state.Filters().Current().String(), loc.Total(), loc.Page(), loc.MaxPage())
	u.screen.Print(PrintArgs{
		X:   int(width - runewidth.StringWidth(pmsg)),
		Y:   location,
		Fg:  u.styles.Basic.fg,
		Bg:  u.styles.Basic.bg,
		Msg: pmsg,
	})

	u.screen.Flush()
}

// NewStatusBar creates a new StatusBar struct
func NewStatusBar(screen Screen, anchor VerticalAnchor, anchorOffset int, styles *StyleSet) *StatusBar {
	return &StatusBar{
		AnchorSettings: NewAnchorSettings(screen, anchor, anchorOffset),
		clearTimer:     nil,
		styles:         styles,
	}
}

func (s *StatusBar) stopTimer() {
	s.timerMutex.Lock()
	defer s.timerMutex.Unlock()
	if t := s.clearTimer; t != nil {
		t.Stop()
		s.clearTimer = nil
	}
}

func (s *StatusBar) setClearTimer(t *time.Timer) {
	s.timerMutex.Lock()
	defer s.timerMutex.Unlock()
	s.clearTimer = t
}

// PrintStatus prints a new status message. This also resets the
// timer created by ClearStatus()
func (s *StatusBar) PrintStatus(msg string, clearDelay time.Duration) {
	if pdebug.Enabled {
		g := pdebug.Marker("StatusBar.PrintStatus")
		defer g.End()
	}

	s.stopTimer()

	location := s.AnchorPosition()

	w, _ := s.screen.Size()
	width := runewidth.StringWidth(msg)
	for width > w {
		_, rw := utf8.DecodeRuneInString(msg)
		width = width - rw
		msg = msg[rw:]
	}

	var pad []byte
	if w > width {
		pad = make([]byte, w-width)
		for i := 0; i < w-width; i++ {
			pad[i] = ' '
		}
	}

	fgAttr := s.styles.Basic.fg
	bgAttr := s.styles.Basic.bg

	if w > width {
		s.screen.Print(PrintArgs{
			Y:   location,
			Fg:  fgAttr,
			Bg:  bgAttr,
			Msg: string(pad),
		})
	}

	if width > 0 {
		s.screen.Print(PrintArgs{
			X:   int(w - width),
			Y:   location,
			Fg:  fgAttr | termbox.AttrReverse | termbox.AttrBold | termbox.AttrReverse,
			Bg:  bgAttr | termbox.AttrReverse,
			Msg: msg,
		})
	}
	s.screen.Flush()

	// if everything is successful AND the clearDelay timer is specified,
	// then set a timer to clear the status
	if clearDelay != 0 {
		s.setClearTimer(time.AfterFunc(clearDelay, func() {
			s.PrintStatus("", 0)
		}))
	}
}

// NewListArea creates a new ListArea struct
func NewListArea(screen Screen, anchor VerticalAnchor, anchorOffset int, sortTopDown bool, styles *StyleSet) *ListArea {
	return &ListArea{
		AnchorSettings: NewAnchorSettings(screen, anchor, anchorOffset),
		displayCache:   []line.Line{},
		dirty:          false,
		sortTopDown:    sortTopDown,
		styles:         styles,
	}
}

func (l *ListArea) purgeDisplayCache() {
	l.displayCache = []line.Line{}
}

func (l *ListArea) IsDirty() bool {
	return l.dirty
}

func (l *ListArea) SetDirty(dirty bool) {
	l.dirty = dirty
}

func selectionContains(state *Peco, n int) bool {
	if l, err := state.CurrentLineBuffer().LineAt(n); err == nil {
		return state.Selection().Has(l)
	}
	return false
}

type DrawOptions struct {
	RunningQuery bool
	DisableCache bool
}

// Draw displays the ListArea on the screen
func (l *ListArea) Draw(state *Peco, parent Layout, perPage int, options *DrawOptions) {
	if pdebug.Enabled {
		g := pdebug.Marker("ListArea.Draw pp = %d, options = %#v", perPage, options)
		defer g.End()
	}

	if perPage < 1 {
		panic("perPage < 1 (was " + strconv.Itoa(perPage) + ")")
	}

	loc := state.Location()

	linebuf := state.CurrentLineBuffer()

	// Should only get into this clause if we are RUNNING A QUERY.
	// regular paging shouldn't be affected. This clause basically
	// makes sure that we never have an empty screen when we are
	// at a large enough page, but we don't have enough entries
	// to fill that many pages in the buffer
	if options != nil && options.RunningQuery {
		bufsiz := linebuf.Size()
		page := loc.Page()

		for page > 1 {
			if (loc.PerPage()*(page-1) < bufsiz) &&
				(loc.PerPage()*page) >= bufsiz {
				break
			}

			page--
		}
		if loc.Page() != page {
			loc.SetPage(page)
			parent.DrawPrompt(state)
		}
	}

	pf := loc.PageCrop()
	if pdebug.Enabled {
		pdebug.Printf("Cropping linebuf which contains %d lines at page %d (%d entries per page)", linebuf.Size(), pf.currentPage, pf.perPage)
	}
	buf := pf.Crop(linebuf)
	bufsiz := buf.Size()

	// This protects us from losing the selected line in case our selected
	// line is greater than the buffer
	if lbufsiz := linebuf.Size(); lbufsiz > 0 && loc.LineNumber() >= lbufsiz {
		loc.SetLineNumber(lbufsiz - 1)
	}

	// The max column size is calculated by buf. we check against where the
	// loc variable thinks we should be scrolling to, and make sure that this
	// falls in range with what we got
	width, _ := state.screen.Size()
	if max := maxOf(buf.MaxColumn()-width, 0); loc.Column() > max {
		loc.SetColumn(max)
	}

	// previously drawn lines are cached. first, truncate the cache
	// to current size of the drawable area
	if ldc := int(len(l.displayCache)); ldc != perPage {
		newCache := make([]line.Line, perPage)
		copy(newCache, l.displayCache)
		l.displayCache = newCache
	} else if perPage > bufsiz {
		l.displayCache = l.displayCache[:bufsiz]
	}

	var y int
	start := l.AnchorPosition()

	// If our buffer is smaller than perPage, we may need to
	// clear some lines
	if pdebug.Enabled {
		pdebug.Printf("ListArea.Draw: buffer size is %d, our view area is %d", bufsiz, perPage)
	}

	for n := bufsiz; n < perPage; n++ {
		if l.sortTopDown {
			y = n + start
		} else {
			y = start - n
		}

		l.screen.Print(PrintArgs{
			Y:    y,
			Fg:   l.styles.Basic.fg,
			Bg:   l.styles.Basic.bg,
			Fill: true,
		})
	}

	var cached, written int
	var fgAttr, bgAttr termbox.Attribute
	var selectionPrefix = state.selectionPrefix
	var prefix = ""

	var prefixCurrentSelection string
	var prefixSavedSelection string
	var prefixDefault string
	if len := len(selectionPrefix); len > 0 {
		prefixCurrentSelection = selectionPrefix + " "
		prefixSavedSelection = "*" + strings.Repeat(" ", len)
		prefixDefault = strings.Repeat(" ", len+1)
	}

	for n := 0; n < perPage; n++ {
		if len(selectionPrefix) > 0 {
			switch {
			case n+loc.Offset() == loc.LineNumber():
				prefix = prefixCurrentSelection
			case selectionContains(state, n+loc.Offset()):
				prefix = prefixSavedSelection
			default:
				prefix = prefixDefault
			}
		} else {
			switch {
			case n+loc.Offset() == loc.LineNumber():
				fgAttr = l.styles.Selected.fg
				bgAttr = l.styles.Selected.bg
			case selectionContains(state, n+loc.Offset()):
				fgAttr = l.styles.SavedSelection.fg
				bgAttr = l.styles.SavedSelection.bg
			default:
				fgAttr = l.styles.Basic.fg
				bgAttr = l.styles.Basic.bg
			}
		}

		if n >= bufsiz {
			break
		}

		if l.sortTopDown {
			y = n + start
		} else {
			y = start - n
		}

		target, err := buf.LineAt(n)
		if err != nil {
			break
		}

		if (options != nil && options.DisableCache) || l.IsDirty() || target.IsDirty() {
			target.SetDirty(false)
		} else if l.displayCache[n] == target {
			cached++
			continue
		}

		written++
		l.displayCache[n] = target

		x := -1 * loc.Column()
		xOffset := loc.Column()
		line := target.DisplayString()

		if len := len(prefix); len > 0 {
			l.screen.Print(PrintArgs{
				X:       x,
				Y:       y,
				XOffset: xOffset,
				Fg:      fgAttr,
				Bg:      bgAttr,
				Msg:     prefix,
			})
			x += len
		}
		if state.SingleKeyJumpMode() || state.SingleKeyJumpShowPrefix() {
			prefixes := state.SingleKeyJumpPrefixes()
			if n < int(len(prefixes)) {
				l.screen.Print(PrintArgs{
					X:       x,
					Y:       y,
					XOffset: xOffset,
					Fg:      fgAttr | termbox.AttrBold | termbox.AttrReverse,
					Bg:      bgAttr,
					Msg:     string(prefixes[n]),
				})
				l.screen.Print(PrintArgs{
					X:       x + 1,
					Y:       y,
					XOffset: xOffset,
					Fg:      fgAttr,
					Bg:      bgAttr,
					Msg:     " ",
				})
			} else {
				l.screen.Print(PrintArgs{
					X:       x,
					Y:       y,
					XOffset: xOffset,
					Fg:      fgAttr,
					Bg:      bgAttr,
					Msg:     "  ",
				})
			}

			x += 2
		}

		ix, ok := target.(MatchIndexer)
		if !ok {
			l.screen.Print(PrintArgs{
				X:       x,
				Y:       y,
				XOffset: xOffset,
				Fg:      fgAttr,
				Bg:      bgAttr,
				Msg:     line,
				Fill:    true,
			})
			continue
		}

		matches := ix.Indices()
		prev := x
		index := 0

		for _, m := range matches {
			if m[0] > index {
				c := line[index:m[0]]
				n := l.screen.Print(PrintArgs{
					X:       prev,
					Y:       y,
					XOffset: xOffset,
					Fg:      fgAttr,
					Bg:      bgAttr,
					Msg:     c,
				})
				prev += n
				index += len(c)
			}
			c := line[m[0]:m[1]]

			n := l.screen.Print(PrintArgs{
				X:       prev,
				Y:       y,
				XOffset: xOffset,
				Fg:      l.styles.Matched.fg,
				Bg:      mergeAttribute(bgAttr, l.styles.Matched.bg),
				Msg:     c,
				Fill:    true,
			})
			prev += n
			index += len(c)
		}

		m := matches[len(matches)-1]
		if m[0] > index {
			l.screen.Print(PrintArgs{
				X:       prev,
				Y:       y,
				XOffset: xOffset,
				Fg:      l.styles.Query.fg,
				Bg:      mergeAttribute(bgAttr, l.styles.Query.bg),
				Msg:     line[m[0]:m[1]],
				Fill:    true,
			})
		} else if len(line) > m[1] {
			l.screen.Print(PrintArgs{
				X:       prev,
				Y:       y,
				XOffset: xOffset,
				Fg:      fgAttr,
				Bg:      bgAttr,
				Msg:     line[m[1]:len(line)],
				Fill:    true,
			})
		}
	}
	l.SetDirty(false)
	if pdebug.Enabled {
		pdebug.Printf("ListArea.Draw: Written total of %d lines (%d cached)", written+cached, cached)
	}
}

func maxOf(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// NewDefaultLayout creates a new Layout in the default format (top-down)
func NewDefaultLayout(state *Peco) *BasicLayout {
	return &BasicLayout{
		StatusBar: NewStatusBar(state.Screen(), AnchorBottom, 0+extraOffset, state.Styles()),
		// The prompt is at the top
		prompt: NewUserPrompt(state.Screen(), AnchorTop, 0, state.Prompt(), state.Styles()),
		// The list area is at the top, after the prompt
		// It's also displayed top-to-bottom order
		list: NewListArea(state.Screen(), AnchorTop, 1, true, state.Styles()),
	}
}

// NewBottomUpLayout creates a new Layout in bottom-up format
func NewBottomUpLayout(state *Peco) *BasicLayout {
	return &BasicLayout{
		StatusBar: NewStatusBar(state.Screen(), AnchorBottom, 0+extraOffset, state.Styles()),
		// The prompt is at the bottom, above the status bar
		prompt: NewUserPrompt(state.Screen(), AnchorBottom, 1+extraOffset, state.Prompt(), state.Styles()),
		// The list area is at the bottom, above the prompt
		// It's displayed in bottom-to-top order
		list: NewListArea(state.Screen(), AnchorBottom, 2+extraOffset, false, state.Styles()),
	}
}

func (l *BasicLayout) PurgeDisplayCache() {
	l.list.purgeDisplayCache()
}

// CalculatePage calculates which page we're displaying
func (l *BasicLayout) CalculatePage(state *Peco, perPage int) error {
	if pdebug.Enabled {
		g := pdebug.Marker("BasicLayout.Calculate %d", perPage)
		defer g.End()
	}
	buf := state.CurrentLineBuffer()
	loc := state.Location()
	loc.SetPage((loc.LineNumber() / perPage) + 1)
	loc.SetOffset((loc.Page() - 1) * perPage)
	loc.SetPerPage(perPage)
	loc.SetTotal(buf.Size())

	if loc.Total() == 0 {
		loc.SetMaxPage(1)
	} else {
		loc.SetMaxPage((loc.Total() + perPage - 1) / perPage)
	}

	if loc.MaxPage() < loc.Page() {
		if buf.Size() == 0 {
			// wait for targets
			return errors.New("no targets or query. nothing to do")
		}
		loc.SetLineNumber(loc.Offset())
	}

	return nil
}

// DrawPrompt draws the prompt to the terminal
func (l *BasicLayout) DrawPrompt(state *Peco) {
	l.prompt.Draw(state)
}

// DrawScreen draws the entire screen
func (l *BasicLayout) DrawScreen(state *Peco, options *DrawOptions) {
	if pdebug.Enabled {
		g := pdebug.Marker("BasicLayout.DrawScreen")
		defer g.End()
	}

	perPage := l.linesPerPage()

	if err := l.CalculatePage(state, perPage); err != nil {
		return
	}

	l.DrawPrompt(state)
	l.list.Draw(state, l, perPage, options)

	if err := l.screen.Flush(); err != nil {
		return
	}
}

func (l *BasicLayout) linesPerPage() int {
	_, height := l.screen.Size()

	// list area is always the display area - 2 lines for prompt and status
	reservedLines := 2 + extraOffset
	pp := height - reservedLines
	if pp < 1 {
		// This is an error condition, and while we probably should handle this
		// error more gracefully, the consumers of this method do not really
		// do anything with this error. I think it's just safe to "2", which just
		// means no space left to draw anything
		if pdebug.Enabled {
			pdebug.Printf(
				"linesPerPage is < 1 (height = %d, reservedLines = %d), forcing return value of 2",
				height,
				reservedLines,
			)
		}
		return 2
	}
	return pp
}

// MovePage scrolls the screen
func (l *BasicLayout) MovePage(state *Peco, p PagingRequest) (moved bool) {
	switch p.Type() {
	case ToScrollLeft, ToScrollRight:
		moved = horizontalScroll(state, l, p)
	default:
		moved = verticalScroll(state, l, p)
	}
	return
}

// verticalScroll moves the cursor position vertically
func verticalScroll(state *Peco, l *BasicLayout, p PagingRequest) bool {
	// Before we move, on which line were we located?
	loc := state.Location()
	lineBefore := loc.LineNumber()
	lineno := lineBefore

	if pdebug.Enabled {
		defer func() {
			pdebug.Printf("currentLine changed from %d -> %d", lineBefore, state.Location().LineNumber())
		}()
	}

	buf := state.CurrentLineBuffer()
	lcur := buf.Size()

	defer func() {
		for _, lno := range []int{lineBefore, loc.LineNumber()} {
			if oldLine, err := buf.LineAt(lno); err == nil {
				oldLine.SetDirty(true)
			}
		}
	}()

	lpp := l.linesPerPage()
	if l.list.sortTopDown {
		switch p.Type() {
		case ToLineAbove:
			lineno--
		case ToLineBelow:
			lineno++
		case ToScrollPageDown:
			lineno += lpp
			if loc.Page() == loc.MaxPage()-1 && lcur < lineno && (lcur-lineBefore) < lpp {
				lineno = lcur - 1
			}
		case ToScrollPageUp:
			lineno -= lpp
		case ToLineInPage:
			lineno = loc.PerPage()*(loc.Page()-1) + p.(JumpToLineRequest).Line()
		}
	} else {
		switch p.Type() {
		case ToLineAbove:
			lineno++
		case ToLineBelow:
			lineno--
		case ToScrollPageDown:
			lineno -= lpp
		case ToScrollPageUp:
			lineno += lpp
		case ToLineInPage:
			lineno = loc.PerPage()*(loc.Page()-1) - p.(JumpToLineRequest).Line()
		}
	}

	if lineno < 0 {
		if lcur > 0 {
			// Go to last page, if possible
			lineno = lcur - 1
		} else {
			lineno = 0
		}
	} else if lcur > 0 && lineno >= lcur {
		lineno = 0
	}

	// XXX DO NOT RETURN UNTIL YOU SET THE LINE NUMBER HERE
	loc.SetLineNumber(lineno)

	// if we were in range mode, we need to do stuff. otherwise
	// just bail out
	r := state.SelectionRangeStart()
	if !r.Valid() {
		return true
	}

	sel := state.Selection()
	if l.list.sortTopDown {
		if loc.LineNumber() < r.Value() {
			for lineno := loc.LineNumber(); lineno <= r.Value(); lineno++ {
				if line, err := buf.LineAt(lineno); err == nil {
					sel.Add(line)
				}
			}
			switch {
			case r.Value() <= lineBefore:
				for lineno := r.Value(); lineno <= lcur && lineno < lineBefore; lineno++ {
					if line, err := buf.LineAt(lineno); err == nil {
						sel.Remove(line)
					}
				}
			case lineBefore < loc.LineNumber():
				for lineno := lineBefore; lineno < loc.LineNumber(); lineno++ {
					if line, err := buf.LineAt(lineno); err == nil {
						sel.Remove(line)
					}
				}
			}
		} else {
			for lineno := r.Value(); lineno <= lcur && lineno <= loc.LineNumber(); lineno++ {
				if line, err := buf.LineAt(lineno); err == nil {
					sel.Add(line)
				}
			}

			switch {
			case lineBefore <= r.Value():
				for lineno := lineBefore; lineno < r.Value(); lineno++ {
					if line, err := buf.LineAt(lineno); err == nil {
						sel.Remove(line)
					}
				}
			case loc.LineNumber() < lineBefore:
				for lineno := loc.LineNumber(); lineno <= lineBefore; lineno++ {
					if line, err := buf.LineAt(lineno); err == nil {
						sel.Remove(line)
					}
				}
			}
		}
	}

	return true
}

// horizontalScroll scrolls screen horizontal
func horizontalScroll(state *Peco, l *BasicLayout, p PagingRequest) bool {
	width, _ := state.screen.Size()
	loc := state.Location()
	if p.Type() == ToScrollRight {
		loc.SetColumn(loc.Column() + width/2)
	} else if loc.Column() > 0 {
		loc.SetColumn(loc.Column() - width/2)
		if loc.Column() < 0 {
			loc.SetColumn(0)
		}
	} else {
		return false
	}

	l.list.SetDirty(true)

	return true
}
