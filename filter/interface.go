package filter

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"time"

	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

var ErrFilterNotFound = errors.New("specified filter was not found")

var ignoreCaseFlags = regexpFlagList([]string{"i"})
var defaultFlags = regexpFlagList{}
var queryKey = &struct{}{}

// DefaultCustomFilterBufferThreshold is the default value
// for BufferThreshold setting on CustomFilters.
const DefaultCustomFilterBufferThreshold = 100

type Set struct {
	current int
	filters []Filter
	mutex   sync.Mutex
}

// internal stuff
type regexpFlags interface {
	flags(string) []string
}
type regexpFlagList []string

type regexpFlagFunc func(string) []string

type regexpQueryFactory struct {
	compiled  map[string]regexpQuery
	mutex     sync.Mutex
	threshold time.Duration
}

type regexpQuery struct {
	positive []*regexp.Regexp
	negative []*regexp.Regexp
	lastUsed time.Time
}

type Fuzzy struct {
	sortLongest bool
}

type Regexp struct {
	factory   *regexpQueryFactory
	flags     regexpFlags
	quotemeta bool
	mutex     sync.Mutex
	name      string
	outCh     pipeline.ChanOutput
}

type ExternalCmd struct {
	args            []string
	cmd             string
	enableSep       bool
	idgen           line.IDGenerator
	outCh           pipeline.ChanOutput
	name            string
	thresholdBufsiz int
}

type Filter interface {
	Apply(context.Context, []line.Line, pipeline.ChanOutput) error
	BufSize() int
	NewContext(context.Context, string) context.Context
	String() string
	// SupportsParallel returns true if this filter can safely be invoked
	// concurrently on independent chunks of lines. Filters that require
	// global state across all lines (e.g. sorted output) should return false.
	SupportsParallel() bool
}

// Collector is an optional interface that filters can implement to return
// matched lines directly as a slice, bypassing channel-based output.
// This avoids per-chunk channel allocation and goroutine overhead in the
// parallel filter path.
type Collector interface {
	ApplyCollect(context.Context, []line.Line) ([]line.Line, error)
}
