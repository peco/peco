package peco

import (
	"io"
	"sync"
	"time"

	"context"

	"github.com/nsf/termbox-go"
	"github.com/peco/peco/buffer"
	"github.com/peco/peco/filter"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/keyseq"
	"github.com/peco/peco/internal/location"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/peco/peco/query"
	"github.com/peco/peco/ui"
)

const (
	successKey = "success"
	errorKey   = "error"
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
	anchor     ui.AnchorSettings
	bufferSize int
	caret      ui.Caret
	// Config contains the values read in from config file
	config                  Config
	currentLineBuffer       buffer.Buffer
	enableSep               bool // Enable parsing on separators
	execOnFinish            string
	filters                 filter.Set
	idgen                   *idgen
	initialFilter           string
	initialQuery            string   // populated if --query is specified
	inputseq                Inputseq // current key sequence (just the names)
	keymap                  Keymap
	layoutType              string
	location                location.Location
	maxScanBufferSize       int
	mutex                   sync.Mutex
	onCancel                string
	printQuery              bool
	prompt                  string
	query                   *query.Query
	queryExecDelay          time.Duration
	queryExecMutex          sync.Mutex
	queryExecTimer          *time.Timer
	readyCh                 chan struct{}
	resultCh                chan line.Line
	screen                  ui.Screen
	selection               *ui.Selection
	selectionPrefix         string
	selectionRangeStart     ui.RangeStart
	selectOneAndExit        bool // True if --select-1 is enabled
	singleKeyJumpMode       bool
	singleKeyJumpPrefixes   []rune
	singleKeyJumpPrefixMap  map[rune]uint
	singleKeyJumpShowPrefix bool
	skipReadConfig          bool
	styles                  *ui.StyleSet

	// Source is where we buffer input. It gets reused when a new query is
	// executed.
	source *Source

	// cancelFunc is called for Exit()
	cancelFunc func()
	// Errors are stored here
	err error
}

type Keyseq interface {
	Add(keyseq.KeyList, interface{})
	AcceptKey(keyseq.Key) (interface{}, error)
	CancelChain()
	Clear()
	Compile() error
	InMiddleOfChain() bool
}

// View handles the drawing/updating the screen
type View struct {
	layout ui.Layout
	state  *Peco
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
	Style               *ui.StyleSet      `json:"Style"`
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

type FilterQuery query.Query

// Source implements pipeline.Source, and is the buffer for the input
type Source struct {
	pipeline.ChanOutput

	capacity   int
	enableSep  bool
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

type State interface {
	Keymap() *Keymap
	Query() query.Query
	Screen() ui.Screen
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
	OptPrintQuery      bool   `long:"print-query" descritpion:"print out the current query as first line of output"`
}

type CLI struct {
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
	Batch(context.Context, func(context.Context), bool)
	DrawCh() chan hub.Payload
	PagingCh() chan hub.Payload
	QueryCh() chan hub.Payload
	SendDraw(context.Context, interface{})
	SendDrawPrompt(context.Context)
	SendPaging(context.Context, interface{})
	SendQuery(context.Context, string)
	SendStatusMsg(context.Context, string)
	SendStatusMsgAndClear(context.Context, string, time.Duration)
	StatusMsgCh() chan hub.Payload
}

type filterProcessor struct {
	filter filter.Filter
	query  string
}
