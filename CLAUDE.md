<!-- Agent-consumed file. Keep terse, unambiguous, machine-parseable. -->

# CLAUDE.md

peco is an interactive filtering tool for the terminal, written in Go.

## Pre-Read Rules

Read the linked doc BEFORE working in that area. No exceptions.

| Trigger | Doc |
|---------|-----|
| Working with any package API or adding imports | `.claude/docs/packages.md` |
| Modifying cross-package dependencies | `.claude/docs/dependencies.md` |
| Writing or running tests | `.claude/docs/testing.md` |
| Modifying CLI flags, entry point, or output | `.claude/docs/cli.md` |
| Modifying concurrency, hub, buffers, filters, layout, screen, actions | `.claude/docs/internals.md` |

## Build & Test Commands

```bash
make                # Build binary via goreleaser (default target)
make build          # Build binary to dist/peco_<os>_<arch>/peco
make test           # Run all tests: go test -v -race ./...
make deps           # Download Go module dependencies
make clean          # Remove build artifacts
```

Run a single test:
```bash
go test -v -run TestFunctionName ./...
go test -v -run TestFunctionName ./filter/  # for a specific package
```

The entry point is `cmd/peco/peco.go`.

## Architecture

### Concurrency Model

peco runs three main goroutines coordinated via context cancellation:

- **Input loop** (`input.go`) — reads tcell key events, resolves key sequences via Keymap, dispatches actions
- **View loop** (`view.go`) — renders screen in response to draw/paging/status messages
- **Filter loop** (`filter.go`) — executes queries against the line buffer when query text changes

These goroutines communicate through the **Hub** (`hub/`), a central message bus with typed channels: `QueryCh`, `DrawCh`, `PagingCh`, `StatusMsgCh`.

### Data Flow

1. **Source** (`source.go`) reads input lines (stdin or file), implements `pipeline.Source`
2. User keystrokes trigger actions that modify the query
3. Query changes are sent to the Filter loop via Hub
4. **Filter** applies the active filter algorithm to produce matched lines
5. Results flow through the **Pipeline** (`pipeline/`) as `Source → Acceptor → Destination`
6. **View** receives draw messages and delegates to **Layout** (`layout.go`) which composes `UserPrompt`, `ListArea`, and `StatusBar`
7. **Screen** (`screen.go`) wraps tcell/v2 for terminal cell rendering

### Key Interfaces

- **`Buffer`** — line storage (`LineAt`, `Size`); implemented by `MemoryBuffer`, `FilteredBuffer`, `Source`
- **`Filter`** (in `filter/`) — `Apply(ctx, []line.Line, ChanOutput)` for each filter algorithm (IgnoreCase, CaseSensitive, SmartCase, Regexp, IRegexp, Fuzzy, ExternalCmd)
- **`Line`** (`line/`) — represents a single line with `ID`, `Buffer`, `DisplayString`, `Output`
- **`Screen`** — terminal abstraction (`Init`, `SetCell`, `Flush`, `PollEvent`); `SimScreen` used in tests
- **`Layout`** — screen composition (`DrawScreen`, `DrawPrompt`, `MovePage`)
- **`Action`** — user actions bound to keys (`action.go`); ~40 built-in actions, supports combined action sequences

### Selection

Uses `google/btree` for ordered selection storage. Supports multi-select, range mode, and sticky selection (persists across query changes).

### Key Sequence Resolution

`internal/keyseq/` implements Trie, TernarySearch, and AhoCorasick for matching multi-key sequences to actions (longest-match-wins).

### Platform-Specific Code

Platform-specific behavior lives in `layout_any.go` / `layout_windows.go` (handling the `extraOffset` layout constant) and in files suffixed `_posix.go` / `_windows.go` under `internal/util/` (TTY detection, shell integration, home directory resolution).

### Code Generation

Uses `go:generate` with `stringer` for enum string representations.

## Testing Patterns

- `newPeco()` helper creates a test instance with `SimScreen` (mock terminal)
- `NewDummyScreen()` returns a `SimScreen` that supports event injection for simulating user input
- Table-driven tests with `t.Run()` subtests are the common pattern
- Regression tests for specific GitHub issues in `issues_test.go`

## Cache Maintenance

These docs cache repository state. Still read source before modifying code.

1. When your changes affect a doc below, update it in the same commit.
2. If you notice any doc is wrong or stale — even on an unrelated task — fix it immediately.

| Doc | Update trigger |
|-----|----------------|
| `packages.md` | Add/remove/rename exported functions, types, or packages |
| `dependencies.md` | Add/remove internal package imports |
| `testing.md` | Change test infrastructure, helpers, or test commands |
| `cli.md` | Add/remove CLI flags, change exit codes or output format |
| `internals.md` | Change concurrency model, hub channels, buffer types, layout system, action system |
