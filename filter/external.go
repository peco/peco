package filter

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"

	pdebug "github.com/lestrrat-go/pdebug"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
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
		thresholdBufsiz: threshold,
	}
}

func (ecf ExternalCmd) BufSize() int {
	return ecf.thresholdBufsiz
}

func (ecf *ExternalCmd) NewContext(ctx context.Context, query string) context.Context {
	return newContext(ctx, query)
}

func (ecf ExternalCmd) SupportsParallel() bool {
	return true
}

func (ecf ExternalCmd) String() string {
	return ecf.name
}

func (ecf *ExternalCmd) Apply(ctx context.Context, buf []line.Line, out pipeline.ChanOutput) (err error) {
	var readerPanicErr error

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in external filter %q: %v", ecf.name, r)
		}
		if err == nil && readerPanicErr != nil {
			err = readerPanicErr
		}
	}()
	if pdebug.Enabled {
		g := pdebug.Marker("ExternalCmd.Apply").BindError(&err)
		defer g.End()
	}

	query := pipeline.QueryFromContext(ctx)
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

	// When enableSep is true (--null mode), we need to preserve the original
	// lines so that Output() returns the correct post-separator text.
	// Build an index of display string -> original lines for lookup.
	var originalLines map[string][]line.Line
	if ecf.enableSep {
		originalLines = make(map[string][]line.Line)
		for _, l := range buf {
			ds := l.DisplayString()
			originalLines[ds] = append(originalLines[ds], l)
		}
	}

	inbuf := &bytes.Buffer{}
	for _, l := range buf {
		inbuf.WriteString(l.DisplayString())
		inbuf.WriteByte('\n')
	}

	cmd.Stdin = inbuf
	r, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	var wg sync.WaitGroup
	cmdCh := make(chan line.Line)
	wg.Add(1)
	go func(ctx context.Context, cmdCh chan line.Line, rdr *bufio.Reader) {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				readerPanicErr = fmt.Errorf("panic in external filter %q reader: %v", ecf.name, r)
			}
		}()
		defer close(cmdCh)
		defer cmd.Wait()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			b, _, err := rdr.ReadLine()
			if len(b) > 0 {
				s := string(b)
				var l line.Line

				if originalLines != nil {
					if candidates, ok := originalLines[s]; ok && len(candidates) > 0 {
						// Pop the first matching original line to preserve order
						l = candidates[0]
						originalLines[s] = candidates[1:]
					}
				}

				if l == nil {
					// No original line found (or enableSep is false):
					// create a new Raw line as before
					l = line.NewRaw(ecf.idgen.Next(), s, ecf.enableSep, false)
				}

				select {
				case cmdCh <- l:
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
			p.Kill()
		}
		wg.Wait()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case l, ok := <-cmdCh:
			if l == nil || !ok {
				return nil
			}
			out.Send(ctx, l)
		}
	}
}
