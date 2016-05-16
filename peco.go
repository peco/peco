package peco

import (
	"bufio"
	"os"
	"sync"

	"golang.org/x/net/context"

	"github.com/peco/peco/input"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/pipeline"
	"github.com/peco/peco/sig"
	"github.com/pkg/errors"
)

const version = "v0.3.6"

// Global variable that bridges the "screen", so testing is easier
var screen Screen = Termbox{}

// Source implements pipline.Source, and is the buffer for the input
type Source struct {
	pipeline.OutputChannel
	in        *os.File
	lines     []string
	ready     chan struct{}
	setupOnce sync.Once
}

// Creates a new Source. Does not start processing the input until you
// call Setup()
func NewSource(in *os.File) *Source {
	return &Source{
		in:            in, // Note that this may be closed, so do not rely on it
		lines:         nil,
		ready:         make(chan struct{}),
		setupOnce:     sync.Once{},
		OutputChannel: pipeline.OutputChannel(make(chan interface{})),
	}
}

// Setup reads from the input os.File.
func (s *Source) Setup() {
	s.setupOnce.Do(func() {
		var notify sync.Once
		notifycb := func() {
			close(s.ready)
		}
		scanner := bufio.NewScanner(s.in)
		for scanner.Scan() {
			notify.Do(notifycb)
			s.lines = append(s.lines, scanner.Text())
		}
	})
}

func (s *Source) Start(ctx context.Context) {
	go func() {
		defer s.OutputChannel.SendEndMark("end of input")

		for i := 0; i < len(s.lines); i++ {
			select {
			case <-ctx.Done():
				return
			default:
				s.OutputChannel <- s.lines[i]
			}
		}
	}()
}

// Ready returns the "input ready" channel. It will be closed as soon as
// the first line of input is processed via Setup()
func (s *Source) Ready() <-chan struct{} {
	return s.ready
}

// Peco is the global object containing everything required to run peco.
type Peco struct {
	// Args (usually) has a copy of os.Args
	Args []string

	// Config contains the values read in from config file
	Config Config

	Options CLIOptions

	// Source is where we buffer input. It gets reused when a new query is
	// executed.
	Source pipeline.Source

	// cancelFunc is called for Exit()
	cancelFunc func()
	// Errors are stored here
	err error

	layoutType string
}

func (p *Peco) Err() error {
	return p.err
}

func (p *Peco) Exit(err error) {
	p.err = err
	if cf := p.cancelFunc; cf != nil {
		cf()
	}
}

func (p *Peco) MergedKeymap() Keymap {
	return Keymap{}
}

func (p *Peco) Run() error {
	err := p.ParseCommandLine()
	if err != nil {
		return errors.Wrap(err, "failed to parse command line")
	}

	// Read config
	if err := p.ReadConfig(p.Options.OptRcfile); err != nil {
		return errors.Wrap(err, "failed to setup configuration")
	}

	// Setup source buffer
	src, err := p.SetupSource()
	if err != nil {
		return errors.Wrap(err, "failed to setup input source")
	}
	p.Source = src

	var ctx context.Context
	var cancel func()

	ctx = context.Background()

	ctx, cancel = context.WithCancel(ctx)
	// remember this cancel func so p.Exit works (XXX requires locking?)
	p.cancelFunc = cancel

	loopers := []interface {
		Loop(ctx context.Context, cancel func()) error
	}{
		input.New(p.MergedKeymap(), screen.PollEvent()),
		sig.New(sig.SigReceivedHandlerFunc(func(sig os.Signal) {
			p.Exit(errors.New("received signal: " + sig.String()))
		})),
	}

	for _, l := range loopers {
		go l.Loop(ctx, cancel)
	}

	<-ctx.Done()

	return p.Err()
}

func (p *Peco) ParseCommandLine() error {
	args, err := p.Options.parse(p.Args)
	if err != nil {
		return errors.Wrap(err, "failed to parse command line options")
	}
	p.Args = args

	return nil
}

func (p *Peco) SetupSource() (pipeline.Source, error) {
	var in *os.File
	var err error
	switch {
	case len(p.Args) > 0:
		in, err = os.Open(p.Args[0])
		if err != nil {
			return nil, errors.Wrap(err, "failed to open file for input")
		}
	case !util.IsTty(os.Stdin.Fd()):
		in = os.Stdin
	default:
		return nil, errors.Wrap(err, "error: You must supply something to work with via filename or stdin")
	}
	defer in.Close()

	src := NewSource(in)
	// Block until we receive something from `in`
	go src.Setup()
	<-src.Ready()

	return src, nil
}

func (p *Peco) ReadConfig(filename string) error {
	if filename != "" {
		if err := p.Config.ReadFilename(filename); err != nil {
			return errors.Wrap(err, "failed to read config file")
		}
	}

	// Apply config values where applicable
	if err := p.ApplyConfig(); err != nil {
		return errors.Wrap(err, "failed to apply config valeus")
	}

	return nil
}

func (p *Peco) populateCommandList() error {
  for _, v := range p.Config.Command {
    if len(v.Args) == 0 {
      continue
    }
    makeCommandAction(&v).Register("ExecuteCommand." + v.Name)
  }

	return nil
}

func (p *Peco) ApplyConfig() error {
	// If layoutType is not set and is set in the config, set it
	if p.layoutType == "" {
		if v := p.Config.Layout; v != "" {
			p.layoutType = v
		} else {
			p.layoutType = DefaultLayoutType
		}
	}

	if err := p.populateCommandList(); err != nil {
		return err
	}

	return nil
}
