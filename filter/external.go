package filter

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"

	pdebug "github.com/lestrrat/go-pdebug"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/pkg/errors"
)

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
		outCh:           pipeline.OutputChannel(make(chan interface{})),
		thresholdBufsiz: threshold,
	}
}

func (ecf *ExternalCmd) Verify() error {
	if ecf.cmd == "" {
		return errors.Errorf("no executable specified for custom matcher '%s'", ecf.name)
	}

	if _, err := exec.LookPath(ecf.cmd); err != nil {
		return errors.Wrap(err, "failed to locate command")
	}
	return nil
}

func (ecf *ExternalCmd) Apply(ctx context.Context, l line.Line) (line.Line, error) {
	return nil, nil
}

func (ecf *ExternalCmd) Accept(ctx context.Context, in chan interface{}, out pipeline.OutputChannel) {
	if pdebug.Enabled {
		g := pdebug.Marker("ExternalCmd.Accept")
		defer g.End()
	}
	defer out.SendEndMark("end of ExternalCmd")

	buf := make([]line.Line, 0, ecf.thresholdBufsiz)
	for {
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf("ExternalCmd received done")
			}
			return
		case v := <-in:
			switch v.(type) {
			case error:
				if pipeline.IsEndMark(v.(error)) {
					if pdebug.Enabled {
						pdebug.Printf("ExternalCmd received end mark")
					}
					if len(buf) > 0 {
						ecf.launchExternalCmd(ctx, buf, out)
					}
				}
				return
			case line.Line:
				if pdebug.Enabled {
					pdebug.Printf("ExternalCmd received new line")
				}
				buf = append(buf, v.(line.Line))
				if len(buf) < ecf.thresholdBufsiz {
					continue
				}

				ecf.launchExternalCmd(ctx, buf, out)
				buf = buf[0:0]
			}
		}
	}
}

func (ecf ExternalCmd) String() string {
	return ecf.name
}

func (ecf *ExternalCmd) launchExternalCmd(ctx context.Context, buf []line.Line, out pipeline.OutputChannel) {
	defer func() { recover() }() // ignore errors
	if pdebug.Enabled {
		g := pdebug.Marker("ExternalCmd.launchExternalCmd")
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
		pdebug.Printf("Executing command %s %v", cmd.Path, cmd.Args)
	}

	inbuf := &bytes.Buffer{}
	for _, l := range buf {
		inbuf.WriteString(l.DisplayString() + "\n")
	}

	cmd.Stdin = inbuf
	r, err := cmd.StdoutPipe()
	if err != nil {
		return
	}

	err = cmd.Start()
	if err != nil {
		return
	}

	go cmd.Wait()

	cmdCh := make(chan line.Line)
	go func(cmdCh chan line.Line, rdr *bufio.Reader) {
		defer func() { recover() }()
		defer close(cmdCh)
		for {
			b, _, err := rdr.ReadLine()
			if len(b) > 0 {
				// TODO: need to redo the spec for custom matchers
				// This is the ONLY location where we need to actually
				// RECREATE a Raw, and thus the only place where
				// ctx.enableSep is required.
				cmdCh <- line.NewMatched(line.NewRaw(ecf.idgen.Next(), string(b), ecf.enableSep), nil)
			}
			if err != nil {
				break
			}
		}
	}(cmdCh, bufio.NewReader(r))

	defer func() {
		if p := cmd.Process; p != nil {
			p.Kill()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case l, ok := <-cmdCh:
			if l == nil || !ok {
				return
			}
			out.Send(l)
		}
	}
}
