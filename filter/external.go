package filter

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"

	pdebug "github.com/lestrrat-go/pdebug/v2"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/pkg/errors"
)

// NewExternalCmd creates a new filter that uses an external
// command to filter the input
func NewExternalCmd(name string, cmd string, args []string, threshold int, idgen line.IDGenerator, enableSep bool) *ExternalCmd {
	if len(args) == 0 {
		args = []string{"$QUERY"}
	}

	if threshold <= 0 {
		threshold = DefaultCustomFilterBufferThreshold
	}

	return &ExternalCmd{
		args:            args,
		cmd:             cmd,
		enableSep:       enableSep,
		idgen:           idgen,
		name:            name,
		outCh:           pipeline.ChanOutput(make(chan interface{})),
		thresholdBufsiz: threshold,
	}
}

func (ecf ExternalCmd) BufSize() int {
	return ecf.thresholdBufsiz
}

func (ecf *ExternalCmd) NewContext(ctx context.Context, query string) context.Context {
	return newContext(ctx, query)
}

func (ecf ExternalCmd) String() string {
	return ecf.name
}

func (ecf *ExternalCmd) Apply(ctx context.Context, buf []line.Line, out pipeline.ChanOutput) (err error) {
	defer func() {
		if err := recover(); err != nil {
			if pdebug.Enabled {
				pdebug.Printf(ctx, "err: %s", err)
			}
		}
	}() // ignore errors
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "ExternalCmd.Apply").BindError(&err)
		defer g.End()
	}

	query := ctx.Value(queryKey).(string)
	args := append([]string(nil), ecf.args...)
	for i, v := range args {
		if v == "$QUERY" {
			args[i] = query
		}
	}

	cmd := exec.Command(ecf.cmd, args...)
	if pdebug.Enabled {
		pdebug.Printf(ctx, "Executing command %s %v", cmd.Path, cmd.Args)
	}

	inbuf := &bytes.Buffer{}
	for _, l := range buf {
		inbuf.WriteString(l.DisplayString() + "\n")
	}

	cmd.Stdin = inbuf
	r, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, `failed to get stdout pipe`)
	}

	err = cmd.Start()
	if err != nil {
		return errors.Wrap(err, `failed to start command`)
	}

	go func() { _ = cmd.Wait() }()

	cmdCh := make(chan line.Line)
	go func(ctx context.Context, cmdCh chan line.Line, rdr *bufio.Reader) {
		defer func() { _ = recover() }()
		defer close(cmdCh)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			b, _, err := rdr.ReadLine()
			if len(b) > 0 {
				// TODO: need to redo the spec for custom matchers
				// This is the ONLY location where we need to actually
				// RECREATE a Raw, and thus the only place where
				// ctx.enableSep is required.
				select {
				case cmdCh <- line.NewRaw(ecf.idgen.Next(), string(b), ecf.enableSep):
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				return
			}
		}
	}(ctx, cmdCh, bufio.NewReader(r))

	defer func() {
		if p := cmd.Process; p != nil {
			_ = p.Kill()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case l, ok := <-cmdCh:
			if l == nil || !ok {
				return nil
			}
			_ = out.Send(l)
		}
	}
}
