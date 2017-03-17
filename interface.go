package peco

import (
	"io"
	"sync"
	"time"

	"context"

	"github.com/google/btree"
	"github.com/nsf/termbox-go"
	"github.com/peco/peco/filter"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/keyseq"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

const (
	successKey = "success"
	errorKey   = "error"
)

const (
	ToLineAbove      PagingRequestType = iota // ToLineAbove moves the selection to the line above
	ToScrollPageDown                          // ToScrollPageDown moves the selection to the next page
	ToLineBelow                               // ToLineBelow moves the selection to the line below
	ToScrollPageUp                            // ToScrollPageUp moves the selection to the previous page
	ToScrollLeft                              // ToScrollLeft scrolls screen to the left
	ToScrollRight                             // ToScrollRight scrolls screen to the right
	ToLineInPage                              // ToLineInPage jumps to a particular line on the page
)

const (
	DefaultLayoutType  = LayoutTypeTopDown // LayoutTypeTopDown makes the layout so the items read from top to bottom
	LayoutTypeTopDown  = "top-down"        // LayoutTypeBottomUp changes the layout to read from bottom to up
	LayoutTypeBottomUp = "bottom-up"
)

const (
	AnchorTop    VerticalAnchor = iota + 1 // AnchorTop anchors elements towards the top of the screen
	AnchorBottom                           // AnchorBottom anchors elements towards the bottom of the screen
)

// These are used as keys in the config file
const (
	IgnoreCaseMatch    = "IgnoreCase"
	CaseSensitiveMatch = "CaseSensitive"
	SmartCaseMatch     = "SmartCase"
	RegexpMatch        = "Regexp"
)

type idgen struct {
	ch chan uint64
}

// Peco is the global object containing everything required to run peco.
// It also contains the global state of the program.
type Peco struct {
	Argv   []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	hub    MessageHub

	args       []string
	bufferSize int
	caret      Caret
	// Config contains the values read in from config file
	config                  Config
	currentLineBuffer       Buffer
	enableSep               bool // Enable parsing on separators
	execOnFinish            string
	filters                 filter.Set
	idgen                   *idgen
	initialFilter           string
	initialQuery            string   // populated if --query is specified
	inputseq                Inputseq // current key sequence (just the names)
	keymap                  Keymap
	layoutType              string
	location                Location
	maxScanBufferSize       int
	mutex                   sync.Mutex
	onCancel                string
	prompt                  string
	query                   Query
	queryExecDelay          time.Duration
	queryExecMutex          sync.Mutex
	queryExecTimer          *time.Timer
	readyCh                 chan struct{}
	resultCh                chan line.Line
	screen                  Screen
	selection               *Selection
	selectionPrefix         string
	selectionRangeStart     RangeStart
	selectOneAndExit        bool // True if --select-1 is enabled
	singleKeyJumpMode       bool
	singleKeyJumpPrefixes   []rune
	singleKeyJumpPrefixMap  map[rune]uint
	singleKeyJumpShowPrefix bool
	skipReadConfig          bool
	styles                  StyleSet

	// Source is where we buffer input. It gets reused when a new query is
	// executed.
	source *Source

	// cancelFunc is called for Exit()
	cancelFunc func()
	// Errors are stored here
	err error
}

type MatchIndexer interface {
	// Indices return the matched portion(s) of a string after filtering.
	// Note that while Indices may return nil, that just means that there are
	// no substrings to be highlighted. It doesn't mean there were no matches
	Indices() [][]int
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
type Selection struct {
	mutex sync.Mutex
	tree  *btree.BTree
}

// Screen hides termbox from the consuming code so that
// it can be swapped out for testing
type Screen interface {
	Init() error
	Close() error
	Flush() error
	PollEvent(context.Context) chan termbox.Event
	Print(PrintArgs) int
	Resume()
	SetCell(int, int, rune, termbox.Attribute, termbox.Attribute)
	Size() (int, int)
	SendEvent(termbox.Event)
	Suspend()
}

// Termbox just hands out the processing to the termbox library
type Termbox struct {
	mutex     sync.Mutex
	resumeCh  chan chan struct{}
	suspendCh chan struct{}
}

// View handles the drawing/updating the screen
type View struct {
	layout Layout
	state  *Peco
}

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
	DrawScreen(*Peco, *DrawOptions)
	MovePage(*Peco, PagingRequest) (moved bool)
	PurgeDisplayCache()
}

// AnchorSettings groups items that are required to control
// where an anchored item is actually placed
type AnchorSettings struct {
	anchor       VerticalAnchor // AnchorTop or AnchorBottom
	anchorOffset int            // offset this many lines from the anchor
	screen       Screen
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
	timerMutex sync.Mutex
}

// ListArea represents the area where the actual line buffer is
// displayed in the screen
type ListArea struct {
	*AnchorSettings
	sortTopDown  bool
	displayCache []line.Line
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
}

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

// FilteredBuffer holds a "filtered" buffer. It holds a reference to
// the source buffer (note: should be immutable) and a list of indices
// into the source buffer
type FilteredBuffer struct {
	maxcols   int
	src       Buffer
	selection []int // maps from our index to src's index
}

// Config holds all the data that can be configured in the
// external configuration file
type Config struct {
	Action map[string][]string `json:"Action"`
	// Keymap used to be directly responsible for dispatching
	// events against user input, but since then this has changed
	// into something that just records the user's config input
	Keymap              map[string]string `json:"Keymap"`
	Matcher             string            `json:"Matcher"`        // Deprecated.
	InitialMatcher      string            `json:"InitialMatcher"` // Use this instead of Matcher
	InitialFilter       string            `json:"InitialFilter"`
	Style               StyleSet          `json:"Style"`
	Prompt              string            `json:"Prompt"`
	Layout              string            `json:"Layout"`
	OnCancel            string            `json:"OnCancel"`
	CustomMatcher       map[string][]string
	CustomFilter        map[string]CustomFilterConfig
	QueryExecutionDelay int
	StickySelection     bool
	MaxScanBufferSize   int

	// If this is true, then the prefix for single key jump mode
	// is displayed by default.
	SingleKeyJump SingleKeyJumpConfig `json:"SingleKeyJump"`

	// Use this prefix to denote currently selected line
	SelectionPrefix string `json:"SelectionPrefix"`
}

type SingleKeyJumpConfig struct {
	ShowPrefix bool `json:"ShowPrefix"`
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

type Caret struct {
	mutex sync.Mutex
	pos   int
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

// Source implements pipeline.Source, and is the buffer for the input
type Source struct {
	pipeline.ChanOutput

	capacity  int
	enableSep bool
	idgen     line.IDGenerator
	in        io.Reader
	lines     []line.Line
	name      string
	mutex     sync.RWMutex
	ready     chan struct{}
	setupDone chan struct{}
	setupOnce sync.Once
}

type State interface {
	Keymap() *Keymap
	Query() Query
	Screen() Screen
	SetCurrentCol(int)
	CurrentCol() int
	SetCurrentLine(int)
	CurrentLine() int
	SetSingleKeyJumpMode(bool)
	SingleKeyJumpMode() bool
}

type CLIOptions struct {
	OptHelp            bool   `short:"h" long:"help" description:"show this help message and exit"`
	OptQuery           string `long:"query" description:"initial value for query"`
	OptRcfile          string `long:"rcfile" description:"path to the settings file"`
	OptVersion         bool   `long:"version" description:"print the version and exit"`
	OptBufferSize      int    `long:"buffer-size" short:"b" description:"number of lines to keep in search buffer"`
	OptEnableNullSep   bool   `long:"null" description:"expect NUL (\\0) as separator for target/output"`
	OptInitialIndex    int    `long:"initial-index" description:"position of the initial index of the selection (0 base)"`
	OptInitialMatcher  string `long:"initial-matcher" description:"specify the default matcher (deprecated)"`
	OptInitialFilter   string `long:"initial-filter" description:"specify the default filter"`
	OptPrompt          string `long:"prompt" description:"specify the prompt string"`
	OptLayout          string `long:"layout" description:"layout to be used. 'top-down' or 'bottom-up'. default is 'top-down'"`
	OptSelect1         bool   `long:"select-1" description:"select first item and immediately exit if the input contains only 1 item"`
	OptOnCancel        string `long:"on-cancel" description:"specify action on user cancel. 'success' or 'error'.\ndefault is 'success'. This may change in future versions"`
	OptSelectionPrefix string `long:"selection-prefix" description:"use a prefix instead of changing line color to indicate currently selected lines.\ndefault is to use colors. This option is experimental"`
	OptExec            string `long:"exec" description:"execute command instead of finishing/terminating peco.\nPlease note that this command will receive selected line(s) from stdin,\nand will be executed via '/bin/sh -c' or 'cmd /c'"`
}

type CLI struct {
}

type RangeStart struct {
	val   int
	valid bool
}

// Buffer interface is used for containers for lines to be
// processed by peco.
type Buffer interface {
	linesInRange(int, int) []line.Line
	LineAt(int) (line.Line, error)
	Size() int
}

// MemoryBuffer is an implementation of Buffer
type MemoryBuffer struct {
	done         chan struct{}
	lines        []line.Line
	mutex        sync.RWMutex
	PeriodicFunc func()
}

type ActionMap interface {
	ExecuteAction(context.Context, *Peco, termbox.Event) error
}

type Input struct {
	actions ActionMap
	evsrc   chan termbox.Event
	mod     *time.Timer
	mutex   sync.Mutex
	state   *Peco
}

// MessageHub is the interface that must be satisfied by the
// message hub component. Unless we're in testing, github.com/peco/peco/hub.Hub
// is used.
type MessageHub interface {
	Batch(func(), bool)
	DrawCh() chan hub.Payload
	PagingCh() chan hub.Payload
	QueryCh() chan hub.Payload
	SendDraw(interface{})
	SendDrawPrompt()
	SendPaging(interface{})
	SendQuery(string)
	SendStatusMsg(string)
	SendStatusMsgAndClear(string, time.Duration)
	StatusMsgCh() chan hub.Payload
}

type filterProcessor struct {
	filter filter.Filter
	query  string
}
