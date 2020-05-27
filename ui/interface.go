package ui

import (
	"context"
	"sync"
	"time"

	"github.com/google/btree"
	"github.com/nsf/termbox-go"
	"github.com/peco/peco/buffer"
	"github.com/peco/peco/filter"
	"github.com/peco/peco/internal/location"
	"github.com/peco/peco/line"
	"github.com/peco/peco/query"
)

// Selection stores the line ids that were selected by the user.
// The contents of the Selection is always sorted from smallest to
// largest line ID
type Selection struct {
	mutex sync.Mutex
	tree  *btree.BTree
}

type State interface {
	AnchorPosition() int
	Caret() *Caret
	CurrentLineBuffer() buffer.Buffer
	Filters() *filter.Set
	Location() *location.Location
	Prompt() string
	Query() *query.Query
	Screen() Screen
	Selection() *Selection
	SelectionPrefix() string
	SelectionRangeStart() *RangeStart
	SingleKeyJumpMode() bool
	SingleKeyJumpPrefixes() []rune
	SingleKeyJumpShowPrefix() bool
	Styles() *StyleSet
}

// AnchorSettings groups items that are required to control
// where an anchored item is actually placed
type AnchorSettings struct {
	anchor       VerticalAnchor // AnchorTop or AnchorBottom
	anchorOffset int            // offset this many lines from the anchor
	screen       Screen
}

// BasicLayout is... the basic layout :) At this point this is the
// only struct for layouts, which means that while the position
// of components may be configurable, the actual types of components
// that are used are set and static
type BasicLayout struct {
	*StatusBar
	prompt *UserPrompt
	list   *ListArea
}

type Caret struct {
	mutex sync.Mutex
	pos   int
}

type JumpToLineRequest int

// Layout represents the component that controls where elements are placed on screen
type Layout interface {
	PrintStatus(string, time.Duration)
	DrawPrompt(context.Context, State)
	DrawScreen(context.Context, State, ...Option)
	MovePage(State, PagingRequest) (moved bool)
	PurgeDisplayCache()
}

// LayoutType describes the types of layout that peco can take
type LayoutType string

const (
	DefaultLayoutType  = LayoutTypeTopDown // LayoutTypeTopDown makes the layout so the items read from top to bottom
	LayoutTypeTopDown  = "top-down"        // LayoutTypeBottomUp changes the layout to read from bottom to up
	LayoutTypeBottomUp = "bottom-up"
)

// ListArea represents the area where the actual line buffer is
// displayed in the screen
type ListArea struct {
	*AnchorSettings
	sortTopDown  bool
	displayCache []line.Line
	dirty        bool
	styles       *StyleSet
}

type MatchIndexer interface {
	// Indices return the matched portion(s) of a string after filtering.
	// Note that while Indices may return nil, that just means that there are
	// no substrings to be highlighted. It doesn't mean there were no matches
	Indices() [][]int
}

type PagingRequest interface {
	Type() PagingRequestType
}

// PagingRequest can be sent to move the selection cursor
type PagingRequestType int

const (
	ToLineAbove       PagingRequestType = iota // ToLineAbove moves the selection to the line above
	ToScrollPageDown                           // ToScrollPageDown moves the selection to the next page
	ToLineBelow                                // ToLineBelow moves the selection to the line below
	ToScrollPageUp                             // ToScrollPageUp moves the selection to the previous page
	ToScrollLeft                               // ToScrollLeft scrolls screen to the left
	ToScrollRight                              // ToScrollRight scrolls screen to the right
	ToLineInPage                               // ToLineInPage jumps to a particular line on the page
	ToScrollFirstItem                          // ToScrollFirstItem
	ToScrollLastItem                           // ToScrollLastItem
)

type RangeStart struct {
	val   int
	valid bool
}

// Screen hides termbox from the consuming code so that
// it can be swapped out for testing
type Screen interface {
	Init() error
	Close() error
	Flush() error
	PollEvent(context.Context) chan termbox.Event
	Start() *PrintCtx
	Resume()
	SetCell(int, int, rune, termbox.Attribute, termbox.Attribute)
	SetCursor(int, int)
	Size() (int, int)
	SendEvent(termbox.Event)
	Suspend()
}

// StatusBar draws the status message bar
type StatusBar struct {
	*AnchorSettings
	clearTimer *time.Timer
	styles     *StyleSet
	timerMutex sync.Mutex
}

// Style describes termbox styles
type Style struct {
	fg termbox.Attribute
	bg termbox.Attribute
}

// StyleSet holds styles for various sections
type StyleSet struct {
	Basic          *Style `json:"Basic"`
	SavedSelection *Style `json:"SavedSelection"`
	Selected       *Style `json:"Selected"`
	Query          *Style `json:"Query"`
	Matched        *Style `json:"Matched"`
}

// Termbox just hands out the processing to the termbox library
type Termbox struct {
	mutex     sync.Mutex
	resumeCh  chan chan struct{}
	suspendCh chan struct{}
}

// UserPrompt draws the prompt line
type UserPrompt struct {
	*AnchorSettings
	prompt    string
	promptLen int
	styles    *StyleSet
}

// VerticalAnchor describes the direction to which elements in the
// layout are anchored to
type VerticalAnchor int

const (
	AnchorTop    VerticalAnchor = iota + 1 // AnchorTop anchors elements towards the top of the screen
	AnchorBottom                           // AnchorBottom anchors elements towards the bottom of the screen
)
