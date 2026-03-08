<!-- Agent-consumed file. Keep terse, unambiguous, machine-parseable. -->

# Internal Dependency Graph

```
cmd/peco → peco (root), internal/util
cmd/filterbench → peco (root), filter, line, pipeline

peco (root) → config, filter, hub, line, pipeline, query, selection, sig
            → internal/ansi, internal/keyseq, internal/util, internal/buffer

config → internal/util

filter → line, pipeline, internal/util

selection → line

line → internal/ansi

pipeline → line

internal/buffer → line
```

## Layer Grouping

### Leaf (no internal deps)
- `hub` — message bus, no imports
- `query` — query text/caret, no imports
- `sig` — signal handling, no imports
- `internal/ansi` — ANSI parser, no imports
- `internal/keyseq` — key matching, no imports
- `internal/util` — platform utils, no imports

### Core
- `line` → internal/ansi
- `pipeline` → line
- `internal/buffer` → line

### Processing
- `config` → internal/util
- `filter` → line, pipeline, internal/util
- `selection` → line

### Application
- `peco` (root) → all above
- `cmd/peco` → peco, internal/util
- `cmd/filterbench` → filter

## External Dependencies
- `github.com/gdamore/tcell/v2` — terminal UI (screen.go)
- `github.com/goccy/go-yaml` — config parsing
- `github.com/google/btree` — ordered selection storage
- `github.com/jessevdk/go-flags` — CLI flag parsing
- `github.com/lestrrat-go/pdebug` — debug logging
- `github.com/mattn/go-runewidth` — Unicode width calculation
- `github.com/stretchr/testify` — test assertions
