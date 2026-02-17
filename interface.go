package peco

import (
	"fmt"
	"io"
	"sync"
	"time"

	"context"

	"github.com/google/btree"
	"github.com/peco/peco/filter"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/keyseq"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

// OnCancelBehavior specifies what happens when the user cancels peco.
type OnCancelBehavior string

const (
	OnCancelSuccess OnCancelBehavior = "success"
	OnCancelError   OnCancelBehavior = "error"
)

func (o *OnCancelBehavior) UnmarshalText(b []byte) error {
	switch s := string(b); s {
	case "", "success":
		*o = OnCancelSuccess
	case "error":
		*o = OnCancelError
	default:
		return fmt.Errorf("invalid OnCancel value %q: must be %q or %q", s, OnCancelSuccess, OnCancelError)
	}
	return nil
}

const (
	DefaultLayoutType            = LayoutTypeTopDown       // LayoutTypeTopDown makes the layout so the items read from top to bottom
	LayoutTypeTopDown            = "top-down"              // LayoutTypeTopDown displays prompt at top, list top-to-bottom
	LayoutTypeBottomUp           = "bottom-up"             // LayoutTypeBottomUp displays prompt at bottom, list bottom-to-top
	LayoutTypeTopDownQueryBottom = "top-down-query-bottom"  // LayoutTypeTopDownQueryBottom displays list top-to-bottom, prompt at bottom
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
	IRegexpMatch       = "IRegexp"
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
	onCancel                OnCancelBehavior
	printQuery              bool
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
	exitZeroAndExit         bool // True if --exit-0 is enabled
	selectOneAndExit        bool // True if --select-1 is enabled
	selectAllAndExit        bool // True if --select-all is enabled
	singleKeyJump SingleKeyJumpState
	heightSpec              *HeightSpec
	readConfigFn            func(*Config, string) error
	styles                  StyleSet
	enableANSI              bool // Enable ANSI color code support
	use256Color             bool
	fuzzyLongestSort        bool

	// Source is where we buffer input. It gets reused when a new query is
	// executed.
	source *Source

	// frozenSource holds a snapshot of filter results when the user
	// "freezes" the current results to filter on top of them.
	frozenSource *MemoryBuffer

	zoom ZoomState

	// cancelFunc is called for Exit()
	cancelFunc func()
	// Errors are stored here
	err error
}

// ContextLine wraps a line.Line to mark it as a context line (non-matched
// surrounding line shown during ZoomIn). Detected via type assertion in
// ListArea.Draw() to apply the Context style.
type ContextLine struct {
	line.Line
}

type Keyseq interface {
	Add(keyseq.KeyList, interface{})
	AcceptKey(keyseq.Key) (interface{}, error)
	CancelChain()
	Clear()
	Compile() error
	InMiddleOfChain() bool
}

// Selection stores the line ids that were selected by the user.
// The contents of the Selection is always sorted from smallest to
// largest line ID
type Selection struct {
	mutex sync.RWMutex
	tree  *btree.BTree
}

// Screen hides the terminal library from the consuming code so that
// it can be swapped out for testing
type Screen interface {
	Init(*Config) error
	Close() error
	Flush() error
	PollEvent(context.Context, *Config) chan Event
	Print(PrintArgs) int
	Resume(context.Context)
	SetCell(int, int, rune, Attribute, Attribute)
	SetCursor(int, int)
	Size() (int, int)
	SendEvent(Event)
	Suspend()
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
	DrawScreen(*Peco, *hub.DrawOptions)
	MovePage(*Peco, hub.PagingRequest) (moved bool)
	PurgeDisplayCache()
	SortTopDown() bool
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

// StatusBar is the interface for printing status messages
type StatusBar interface {
	PrintStatus(string, time.Duration)
}

// screenStatusBar draws the status message bar on screen
type screenStatusBar struct {
	*AnchorSettings
	clearTimer *time.Timer
	styles     *StyleSet
	timerMutex sync.Mutex
}

// nullStatusBar is a no-op status bar used when SuppressStatusMsg is true
type nullStatusBar struct{}

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
	statusBar StatusBar
	screen    Screen
	prompt    *UserPrompt
	list      *ListArea
}

// Keymap holds all the key sequence to action map
type Keymap struct {
	Config map[string]string
	Action map[string][]string // custom actions
	seq    Keyseq
}

// Filter is responsible for the actual "grep" part of peco
type Filter struct {
	state          *Peco
	prevQuery      string
	prevResults    *MemoryBuffer
	prevFilterName string
	prevMu         sync.Mutex
}

// Action describes an action that can be executed upon receiving user input.
type Action interface {
	Execute(context.Context, *Peco, Event)
}

// ActionFunc is a type of Action that is basically just a callback.
type ActionFunc func(context.Context, *Peco, Event)

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
	Action map[string][]string `json:"Action" yaml:"Action"`
	// Keymap used to be directly responsible for dispatching
	// events against user input, but since then this has changed
	// into something that just records the user's config input
	Keymap        map[string]string `json:"Keymap" yaml:"Keymap"`
	InitialFilter string            `json:"InitialFilter" yaml:"InitialFilter"`
	Style         StyleSet          `json:"Style" yaml:"Style"`
	Prompt        string            `json:"Prompt" yaml:"Prompt"`
	Layout        string            `json:"Layout" yaml:"Layout"`
	Use256Color   bool              `json:"Use256Color" yaml:"Use256Color"`
	OnCancel     OnCancelBehavior              `json:"OnCancel" yaml:"OnCancel"`
	CustomFilter map[string]CustomFilterConfig `json:"CustomFilter" yaml:"CustomFilter"`
	QueryExecutionDelay int                                `json:"QueryExecutionDelay" yaml:"QueryExecutionDelay"`
	StickySelection     bool                               `json:"StickySelection" yaml:"StickySelection"`
	MaxScanBufferSize   int                                `json:"MaxScanBufferSize" yaml:"MaxScanBufferSize"`
	FilterBufSize       int                                `json:"FilterBufSize" yaml:"FilterBufSize"`
	FuzzyLongestSort    bool                               `json:"FuzzyLongestSort" yaml:"FuzzyLongestSort"`
	SuppressStatusMsg   bool                               `json:"SuppressStatusMsg" yaml:"SuppressStatusMsg"`
	ANSI                bool                               `json:"ANSI" yaml:"ANSI"`

	// If this is true, then the prefix for single key jump mode
	// is displayed by default.
	SingleKeyJump SingleKeyJumpConfig `json:"SingleKeyJump" yaml:"SingleKeyJump"`

	// Use this prefix to denote currently selected line
	SelectionPrefix string `json:"SelectionPrefix" yaml:"SelectionPrefix"`

	// Height specifies the display height in lines or percentage (e.g. "10", "50%").
	// When set, peco renders inline without using the alternate screen buffer.
	Height string `json:"Height" yaml:"Height"`
}

type SingleKeyJumpConfig struct {
	ShowPrefix bool `json:"ShowPrefix" yaml:"ShowPrefix"`
}

// CustomFilterConfig is used to specify configuration parameters
// to CustomFilters
type CustomFilterConfig struct {
	// Cmd is the name of the command to invoke
	Cmd string `json:"Cmd" yaml:"Cmd"`

	// TODO: need to check if how we use this is correct
	Args []string `json:"Args" yaml:"Args"`

	// BufferThreshold defines how many lines peco buffers before
	// invoking the external command. If this value is big, we
	// will execute the external command fewer times, but the
	// results will not be generated for longer periods of time.
	// If this value is small, we will execute the external command
	// more often, but you pay the penalty of invoking that command
	// more times.
	BufferThreshold int `json:"BufferThreshold" yaml:"BufferThreshold"`
}

// StyleSet holds styles for various sections
type StyleSet struct {
	Basic          Style `json:"Basic" yaml:"Basic"`
	SavedSelection Style `json:"SavedSelection" yaml:"SavedSelection"`
	Selected       Style `json:"Selected" yaml:"Selected"`
	Query          Style `json:"Query" yaml:"Query"`
	Matched        Style `json:"Matched" yaml:"Matched"`
	Prompt         Style `json:"Prompt" yaml:"Prompt"`
	Context        Style `json:"Context" yaml:"Context"`
}

// Attribute represents terminal display attributes such as colors
// and text styling (bold, underline, reverse). It is a uint32 bitfield:
//
//	Bits 0-8:   Palette color index (0=default, 1-256 for 256-color palette)
//	Bits 0-23:  RGB color value (when AttrTrueColor flag is set)
//	Bit 24:     AttrTrueColor flag — distinguishes true color from palette
//	Bit 25:     AttrBold
//	Bit 26:     AttrUnderline
//	Bit 27:     AttrReverse
//	Bits 28-31: Reserved
type Attribute uint32

// Named palette color constants (values 0-8).
const (
	ColorDefault Attribute = 0x0000
	ColorBlack   Attribute = 0x0001
	ColorRed     Attribute = 0x0002
	ColorGreen   Attribute = 0x0003
	ColorYellow  Attribute = 0x0004
	ColorBlue    Attribute = 0x0005
	ColorMagenta Attribute = 0x0006
	ColorCyan    Attribute = 0x0007
	ColorWhite   Attribute = 0x0008
)

const (
	AttrTrueColor Attribute = 0x01000000
	AttrBold      Attribute = 0x02000000
	AttrUnderline Attribute = 0x04000000
	AttrReverse   Attribute = 0x08000000
)

// Style describes display attributes for foreground and background.
type Style struct {
	fg Attribute
	bg Attribute
}

type Caret struct {
	mutex sync.Mutex
	pos   int
}

type Location struct {
	mutex   sync.RWMutex
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

	capacity   int
	enableSep  bool
	enableANSI bool
	idgen      line.IDGenerator
	in         io.Reader
	inClosed   bool
	isInfinite bool
	lines      []line.Line
	name       string
	mutex      sync.RWMutex
	ready      chan struct{}
	setupDone  chan struct{}
	setupOnce  sync.Once
}

type CLIOptions struct {
	OptHelp            bool   `short:"h" long:"help" description:"show this help message and exit"`
	OptQuery           string `long:"query" description:"initial value for query"`
	OptRcfile          string `long:"rcfile" description:"path to the settings file"`
	OptVersion         bool   `long:"version" description:"print the version and exit"`
	OptBufferSize      int    `long:"buffer-size" short:"b" description:"number of lines to keep in search buffer"`
	OptEnableNullSep   bool   `long:"null" description:"expect NUL (\\0) as separator for target/output"`
	OptInitialIndex    int    `long:"initial-index" description:"position of the initial index of the selection (0 base)"`
	OptInitialFilter   string `long:"initial-filter" description:"specify the default filter"`
	OptPrompt          string `long:"prompt" description:"specify the prompt string"`
	OptLayout          string `long:"layout" description:"layout to be used. 'top-down', 'bottom-up', or 'top-down-query-bottom'. default is 'top-down'"`
	OptSelect1         bool   `long:"select-1" description:"select first item and immediately exit if the input contains only 1 item"`
	OptExitZero        bool   `long:"exit-0" description:"exit immediately with status 1 if the input is empty"`
	OptSelectAll       bool   `long:"select-all" description:"select all items and immediately exit"`
	OptOnCancel        string `long:"on-cancel" description:"specify action on user cancel. 'success' or 'error'.\ndefault is 'success'. This may change in future versions"`
	OptSelectionPrefix string `long:"selection-prefix" description:"use a prefix instead of changing line color to indicate currently selected lines.\ndefault is to use colors. This option is experimental"`
	OptExec            string `long:"exec" description:"execute command instead of finishing/terminating peco.\nPlease note that this command will receive selected line(s) from stdin,\nand will be executed via '/bin/sh -c' or 'cmd /c'"`
	OptPrintQuery      bool   `long:"print-query" description:"print out the current query as first line of output"`
	OptANSI            bool   `long:"ansi" description:"enable ANSI color code support"`
	OptHeight          string `long:"height" description:"display height in lines or percentage (e.g. '10', '50%')"`
}

type CLI struct {
}

type RangeStart struct {
	val   int
	valid bool
}

// Buffer interface is used for containers for lines to be
// processed by peco. The unexported linesInRange method seals this
// interface to the peco package — external packages cannot implement it.
// This is intentional: linesInRange is an internal optimization for
// efficient pagination in NewFilteredBuffer, not part of the public contract.
type Buffer interface {
	linesInRange(int, int) []line.Line
	LineAt(int) (line.Line, error)
	Size() int
}

// MemoryBuffer is an implementation of Buffer
type MemoryBuffer struct {
	done         chan struct{}
	doneOnce     sync.Once
	lines        []line.Line
	mutex        sync.RWMutex
	PeriodicFunc func()
}

type ActionMap interface {
	ExecuteAction(context.Context, *Peco, Event) error
}

type Input struct {
	actions    ActionMap
	evsrc      chan Event
	pendingEsc chan Event // receives Esc events from the timer callback
	mod        *time.Timer
	modGen     uint64 // generation counter to invalidate stale timer callbacks.
	// uint64 holds up to ~1.8×10¹⁹. At most 2 increments per Esc key event
	// and a generous 100 keystrokes/second, overflow would take ~2.9 trillion years.
	mutex sync.Mutex
	state *Peco
}

// HubSender provides methods for sending messages to the hub.
// Most code (actions, input handling, source setup) only needs
// the sender side.
type HubSender interface {
	Batch(context.Context, func(context.Context), bool)
	SendDraw(context.Context, *hub.DrawOptions)
	SendDrawPrompt(context.Context)
	SendPaging(context.Context, hub.PagingRequest)
	SendQuery(context.Context, string)
	SendStatusMsg(context.Context, string, time.Duration)
}

// HubReceiver provides methods for receiving messages from the hub.
// Only the view loop and filter loop consume from these channels.
type HubReceiver interface {
	DrawCh() chan *hub.Payload[*hub.DrawOptions]
	PagingCh() chan *hub.Payload[hub.PagingRequest]
	QueryCh() chan *hub.Payload[string]
	StatusMsgCh() chan *hub.Payload[hub.StatusMsg]
}

// MessageHub is the interface that must be satisfied by the
// message hub component. Unless we're in testing, github.com/peco/peco/hub.Hub
// is used. It combines HubSender (for dispatching messages) and
// HubReceiver (for consuming them via channels).
type MessageHub interface {
	HubSender
	HubReceiver
}

type filterProcessor struct {
	filter  filter.Filter
	query   string
	bufSize int
}
