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
var queryKey = struct{}{}

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
	rx       []*regexp.Regexp
	lastUsed time.Time
}

type Fuzzy struct {
}

type Regexp struct {
	factory   *regexpQueryFactory
	flags     regexpFlags
	quotemeta bool
	mutex     sync.Mutex
	name      string
	onEnd     func()
	outCh     pipeline.OutputChannel
}

type ExternalCmd struct {
	args            []string
	cmd             string
	enableSep       bool
	idgen           line.IDGenerator
	outCh           pipeline.OutputChannel
	name            string
	thresholdBufsiz int
}

type Filter interface {
	Apply(context.Context, line.Line) (line.Line, error)
	String() string
}
