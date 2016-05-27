package peco

import (
	"io"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/google/btree"
	"github.com/nsf/termbox-go"
	"github.com/peco/peco/internal/keyseq"
)

const invalidSelectionRange = -1

type idGen struct {
	genCh chan uint64
}

// Line represents each of the line that peco uses to display
// and match against queries.
type Line interface {
	btree.Item

	ID() uint64

	// Buffer returns the raw buffer
	Buffer() string

	// DisplayString returns the string to be displayed. This means if you have
	// a null separator, the contents after the separator are not included
	// in this string
	DisplayString() string

	// Indices return the matched portion(s) of a string after filtering.
	// Note that while Indices may return nil, that just means that there are
	// no substrings to be highlighted. It doesn't mean there were no matches
	Indices() [][]int

	// Output returns the string to be display as peco finishes up doing its
	// thing. This means if you have null separator, the contents before the
	// separator are not included in this string
	Output() string

	// IsDirty returns true if this line should be forcefully redrawn
	IsDirty() bool

	// SetDirty sets the dirty flag on or off
	SetDirty(bool)
}

// RawLine is the input line as sent to peco, before filtering and what not.
type RawLine struct {
	id            uint64
	buf           string
	sepLoc        int
	displayString string
	dirty         bool
}

// MatchedLine contains the indices to the matches
type MatchedLine struct {
	Line
	indices [][]int
}

// BufferReader reads from either stdin or a file. In case of stdin,
// it also handles possible infinite source.
type BufferReader struct {
	*Ctx
	input        io.ReadCloser
	inputReadyCh chan struct{}
}

type Keyseq interface {
	Add(keyseq.KeyList, interface{})
	AcceptKey(keyseq.Key) (interface{}, error)
	CancelChain()
	Clear()
	Compile() error
	InMiddleOfChain() bool
}

// PagingRequest can be sent to move the selection cursor
type PagingRequestType int

type PagingRequest interface {
	Type() PagingRequestType
}

type JumpToLineRequest int

// Selection stores the line ids that were selected by the user.
// The contents of the Selection is always sorted from smallest to
// largest line ID
type Selection struct{ *btree.BTree }

// StatusMsgRequest specifies the string to be drawn
// on the status message bar and an optional delay that tells
// the view to clear that message
type StatusMsgRequest struct {
	message    string
	clearDelay time.Duration
}

// Screen hides termbox from the consuming code so that
// it can be swapped out for testing
type Screen interface {
	Init() error
	Close() error
	Flush() error
	PollEvent() chan termbox.Event
	SetCell(int, int, rune, termbox.Attribute, termbox.Attribute)
	Size() (int, int)
	SendEvent(termbox.Event)
}

// Termbox just hands out the processing to the termbox library
type Termbox struct{}

/*
// View handles the drawing/updating the screen
type View struct {
	*Ctx
	mutex  sync.Locker
	layout Layout
}
*/

// PageCrop filters out a new LineBuffer based on entries
// per page and the page number
type PageCrop struct {
	perPage     int
	currentPage int
}

// LayoutType describes the types of layout that peco can take
type LayoutType string

// VerticalAnchor describes the direction to which elements in the
// layout are anchored to
type VerticalAnchor int

// Layout represents the component that controls where elements are placed on screen
type Layout interface {
	PrintStatus(string, time.Duration)
	DrawPrompt(*Peco)
	DrawScreen(*Peco, bool)
	MovePage(*Peco, PagingRequest) (moved bool)
	PurgeDisplayCache()
}

// AnchorSettings groups items that are required to control
// where an anchored item is actually placed
type AnchorSettings struct {
	anchor       VerticalAnchor // AnchorTop or AnchorBottom
	anchorOffset int            // offset this many lines from the anchor
}

// UserPrompt draws the prompt line
type UserPrompt struct {
	*AnchorSettings
	prompt    string
	promptLen int
	styles    *StyleSet
}

// StatusBar draws the status message bar
type StatusBar struct {
	*AnchorSettings
	clearTimer *time.Timer
	styles     *StyleSet
	timerMutex sync.Locker
}

// ListArea represents the area where the actual line buffer is
// displayed in the screen
type ListArea struct {
	*AnchorSettings
	sortTopDown  bool
	displayCache []Line
	dirty        bool
	styles       *StyleSet
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

// Keymap holds all the key sequence to action map
type Keymap struct {
	Config map[string]string
	Action map[string][]string // custom actions
	seq    Keyseq
	state  *Peco
}

// internal stuff
type regexpFlags interface {
	flags(string) []string
}
type regexpFlagList []string

type regexpFlagFunc func(string) []string

// Filter is responsible for the actual "grep" part of peco
type Filter struct {
	state *Peco
}

// Action describes an action that can be executed upon receiving user
// input. It's an interface so you can create any kind of Action you need,
// but most everything is implemented in terms of ActionFunc, which is
// callback based Action
type Action interface {
	Register(string, ...termbox.Key)
	RegisterKeySequence(string, keyseq.KeyList)
	Execute(context.Context, *Peco, termbox.Event)
}

// ActionFunc is a type of Action that is basically just a callback.
type ActionFunc func(context.Context, *Peco, termbox.Event)

type Pipeliner interface {
	Pipeline() (chan struct{}, chan Line)
}

type pipelineCtx struct {
	onIncomingLine func(Line) (Line, error)
	onEnd          func()
}

type simplePipeline struct {
	// Close this channel if you want to cancel the entire pipeline.
	// InputReader is the generator for the pipeline, so this is the
	// only object that can create the cancelCh
	cancelCh chan struct{}
	// Consumers of this generator read from this channel.
	outputCh chan Line
}

// LineBuffer represents a set of lines. This could be the
// raw data read in, or filtered data, such as result of
// running a match, or applying a selection by the user
//
// Buffers should be immutable.
type LineBuffer interface {
	Pipeliner

	LineAt(int) (Line, error)
	Size() int

	// Register registers another LineBuffer that is dependent on
	// this buffer.
	Register(LineBuffer)
	Unregister(LineBuffer)

	// InvalidateUpTo is called when a source buffer invalidates
	// some lines. The argument is the largest line number that
	// should be invalidated (so anything up to that line is no
	// longer valid in the source)
	InvalidateUpTo(int)
}

type dependentBuffers []LineBuffer

// RawLineBuffer holds the raw set of lines as read into peco.
type RawLineBuffer struct {
	simplePipeline
	buffers  dependentBuffers
	lines    []Line
	capacity int // max number of lines. 0 means unlimited
	onEnd    func()
}

// FilteredBuffer holds a "filtered" buffer. It holds a reference to
// the source buffer (note: should be immutable) and a list of indices
// into the source buffer
type FilteredBuffer struct {
	src     Buffer
	selection []int // maps from our index to src's index
}

// Config holds all the data that can be configured in the
// external configuran file
type Config struct {
	Action map[string][]string `json:"Action"`
	// Keymap used to be directly responsible for dispatching
	// events against user input, but since then this has changed
	// into something that just records the user's config input
	Keymap              map[string]string `json:"Keymap"`
	Matcher             string            `json:"Matcher"`        // Deprecated.
	InitialMatcher      string            `json:"InitialMatcher"` // Use this instead of Matcher
	InitialFilter       string            `json:"InitialFilter"`
	Style               *StyleSet         `json:"Style"`
	Prompt              string            `json:"Prompt"`
	Layout              string            `json:"Layout"`
	CustomMatcher       map[string][]string
	CustomFilter        map[string]CustomFilterConfig
	Command             []CommandConfig
	QueryExecutionDelay int
	StickySelection     bool

	// If this is true, then the prefix for single key jump mode
	// is displayed by default.
	SingleKeyJump SingleKeyJumpConfig `json:"SingleKeyJump"`
}

type SingleKeyJumpConfig struct {
	ShowPrefix bool          `json:"ShowPrefix"`
	PrefixList []rune        `json:"-"`
	PrefixMap  map[rune]uint `json:"-"`
}

type CommandConfig struct {
	// Name is the name of the command to execute
	Name string

	// TODO: need to check if how we use this is correct
	Args []string

	// Spawn mean the command should be executed asynchronous.
	Spawn bool
}

// CustomFilterConfig is used to specify configuration parameters
// to CustomFilters
type CustomFilterConfig struct {
	// Cmd is the name of the command to invoke
	Cmd string

	// TODO: need to check if how we use this is correct
	Args []string

	// BufferThreshold defines how many lines peco buffers before
	// invoking the external command. If this value is big, we
	// will execute the external command fewer times, but the
	// results will not be generated for longer periods of time.
	// If this value is small, we will execute the external command
	// more often, but you pay the penalty of invoking that command
	// more times.
	BufferThreshold int
}

// StyleSet holds styles for various sections
type StyleSet struct {
	Basic          Style `json:"Basic"`
	SavedSelection Style `json:"SavedSelection"`
	Selected       Style `json:"Selected"`
	Query          Style `json:"Query"`
	Matched        Style `json:"Matched"`
}

// Style describes termbox styles
type Style struct {
	fg termbox.Attribute
	bg termbox.Attribute
}

// CtxOptions is the interface that defines that options can be
// passed in from the command line
type CtxOptions interface {
	// EnableNullSep should return if the null separator is
	// enabled (--null)
	EnableNullSep() bool

	// BufferSize should return the buffer size. By default (i.e.
	// when it returns 0), the buffer size is unlimited.
	// (--buffer-size)
	BufferSize() int

	// InitialIndex is the line number to put the cursor on
	// when peco starts
	InitialIndex() int

	// LayoutType returns the name of the layout to use
	LayoutType() string
}

type QueryFilterer interface {
	Pipeliner
	Cancel()
	Clone() QueryFilterer
	Accept(Pipeliner)
	SetQuery(string)
	String() string
}

type Location struct {
	col     int
	lineno  int
	maxPage int
	page    int
	perPage int
	offset  int
	total   int
}

type Query struct {
	query      []rune
	savedQuery []rune
	mutex      sync.Mutex
}

type FilterQuery Query

type FilterSet struct {
	filters []LineFilter
	current int
}

type State interface {
	Query() Query

	SetCurrentCol(int)
	CurrentCol() int
	SetCurrentLine(int)
	CurrentLine() int
	SetSingleKeyJumpMode(bool)
	SingleKeyJumpMode() bool
}

// Ctx contains all the important data. while you can easily access
// data in this struct from anywhere, only do so via channels
type Ctx struct {
	*FilterQuery
	filters             FilterSet
	caretPosition       int
	enableSep           bool
	resultCh            chan Line
	mutex               sync.Locker
	currentLine         int
	currentCol          int
	selection           *Selection
	activeLineBuffer    LineBuffer
	rawLineBuffer       *RawLineBuffer
	lines               []Line
	linesMutex          sync.Locker
	current             []Line
	currentMutex        sync.Locker
	bufferSize          int
	config              *Config
	selectionRangeStart int
	layoutType          string
	singleKeyJumpMode   bool
	select1             bool

	wait *sync.WaitGroup
	err  error
}

type CLIOptions struct {
	OptHelp           bool   `short:"h" long:"help" description:"show this help message and exit"`
	OptTTY            string `long:"tty" description:"path to the TTY (usually, the value of $TTY)"`
	OptQuery          string `long:"query" description:"initial value for query"`
	OptRcfile         string `long:"rcfile" description:"path to the settings file"`
	OptVersion        bool   `long:"version" description:"print the version and exit"`
	OptBufferSize     int    `long:"buffer-size" short:"b" description:"number of lines to keep in search buffer"`
	OptEnableNullSep  bool   `long:"null" description:"expect NUL (\\0) as separator for target/output"`
	OptInitialIndex   int    `long:"initial-index" description:"position of the initial index of the selection (0 base)"`
	OptInitialMatcher string `long:"initial-matcher" description:"specify the default matcher (deprecated)"`
	OptInitialFilter  string `long:"initial-filter" description:"specify the default filter"`
	OptPrompt         string `long:"prompt" description:"specify the prompt string"`
	OptLayout         string `long:"layout" description:"layout to be used 'top-down' or 'bottom-up'. default is 'top-down'"`
	OptSelect1        bool   `long:"select-1" description:"select first item and immediately exit if the input contains only 1 item"`
}

type CLI struct {
}
