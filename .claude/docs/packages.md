<!-- Agent-consumed file. Keep terse, unambiguous, machine-parseable. -->

# Package Map

## peco (root)

Interactive filtering tool core. Holds global state, goroutine loops, UI components.

- **New() → *Peco** — create new instance
- **(*Peco).Setup() → error** — initialize from config/options
- **(*Peco).Run(ctx) → error** — main event loop
- **(*Peco).ApplyConfig(CLIOptions) → error** — apply CLI flags to config
- **(*Peco).SetupSource(ctx) → (*Source, error)** — initialize input source
- **(*Peco).PrintResults()** — output selected lines to stdout
- **(*Peco).CurrentLineBuffer() → Buffer** — active line buffer (raw or filtered)
- **(*Peco).ExecQuery(ctx, func()) → bool** — execute filter with debounce
- Key types: `Peco`, `Buffer`, `FilteredBuffer`, `MemoryBuffer`, `Source`, `Screen`, `Layout`, `Action`, `Keymap`, `Event`, `CLIOptions`, `Location`, `PageCrop`
- Key interfaces: `MessageHub`, `Screen`, `Layout`, `Action`, `ActionMap`, `Buffer`, `Keyseq`, `ConfigReader`
- Screen impls: `TcellScreen` (production), `InlineScreen` (height-limited), `DummyScreen` (tests)
- Layout impls: `BasicLayout` with builders: `DefaultLayout`, `BottomUpLayout`, `TopDownQueryBottomLayout`
- Files: `peco.go`, `action.go`, `buffer.go`, `caret.go`, `event.go`, `filter.go`, `input.go`, `keymap.go`, `layout.go`, `layout_any.go`, `layout_windows.go`, `options.go`, `page.go`, `screen.go`, `screen_inline.go`, `source.go`, `state.go`, `view.go`, `vertical_anchor_gen.go`
- Imports: config, filter, hub, line, pipeline, query, selection, sig, internal/ansi, internal/keyseq, internal/util

## cmd/peco

CLI entry point.

- Parses flags via `go-flags`, creates `peco.New()`, calls `Run(ctx)`
- Files: `peco.go`
- Imports: peco (root), internal/util

## cmd/filterbench

Benchmark tool for filter performance.

- Files: `main.go`
- Imports: filter

## config/

Configuration loading and types.

- **Config** — main config struct (Keymap, Action, Style, Layout, CustomFilter, SingleKeyJump, Height, etc.)
- **(*Config).Init() → error** — set defaults
- **(*Config).ReadFilename(string) → error** — load YAML config file
- **LocateRcfile(Locator) → (string, error)** — find config file path
- Key types: `Config`, `StyleSet`, `Style`, `Attribute`, `OnCancelBehavior`, `ColorMode`, `CustomFilterConfig`, `SingleKeyJumpConfig`, `HeightSpec`
- Color constants: `ColorDefault`, `ColorBlack`..`ColorWhite`, `AttrBold`, `AttrUnderline`, `AttrReverse`, `AttrTrueColor`
- Layout constants: `LayoutTypeTopDown`, `LayoutTypeBottomUp`, `LayoutTypeTopDownQueryBottom`
- Files: `config.go`, `style.go`, `height.go`, `layout.go`
- Imports: internal/util

## filter/

Filter algorithm implementations.

- **Filter** interface — `Apply(ctx, []line.Line, ChanOutput) → error`, `BufSize() → int`, `NewContext(ctx, string) → ctx`, `String() → string`, `SupportsParallel() → bool`
- **Collector** interface (optional) — `ApplyCollect(ctx, []line.Line) → ([]line.Line, error)`
- **Set** — filter collection with rotation: `Add(Filter)`, `Rotate()`, `Current() → Filter`, `SetCurrentByName(string) → error`
- Implementations: `NewIgnoreCase()`, `NewCaseSensitive()`, `NewSmartCase()`, `NewRegexp()`, `NewIRegexp()`, `NewFuzzy(longestSort bool)`, `NewExternalCmd(name, cmd string, args []string, threshold int, idgen IDGenerator, enableSep bool)`
- Files: `filter.go`, `base.go`, `regexp.go`, `fuzzy.go`, `external.go`, `set.go`
- Imports: line, pipeline, internal/util

## hub/

Central message bus for goroutine communication.

- **New(bufsize int) → *Hub** — create hub with channel buffer size
- **(*Hub).SendDraw(ctx, *DrawOptions)** — trigger screen redraw
- **(*Hub).SendQuery(ctx, string)** — send query change
- **(*Hub).SendPaging(ctx, PagingRequest)** — send paging command
- **(*Hub).SendStatusMsg(ctx, string, time.Duration)** — show status message
- **(*Hub).Batch(ctx, func(ctx))** — batch multiple sends atomically
- Channel accessors: `DrawCh()`, `PagingCh()`, `QueryCh()`, `StatusMsgCh()`
- Key types: `Hub`, `Payload[T]`, `DrawOptions`, `PagingRequest`, `PagingRequestType`, `StatusMsg`
- Paging types: `ToLineAbove`, `ToLineBelow`, `ToScrollPageDown`, `ToScrollPageUp`, `ToScrollLeft`, `ToScrollRight`, `ToScrollFirstItem`, `ToScrollLastItem`, `ToLineInPage`
- Files: `hub.go`, `draw.go`, `paging.go`, `paging_request_type_gen.go`
- Imports: (none internal)

## line/

Line data types for display and selection.

- **Line** interface — `ID() → uint64`, `Buffer() → string`, `DisplayString() → string`, `Output() → string`, `IsDirty() → bool`, `SetDirty(bool)`, implements `btree.Item`
- **NewRaw(id uint64, s string, enableSep bool, stripANSI bool) → *Raw** — create raw line
- **NewMatched(Line, [][]int) → *Matched** — wrap line with match indices
- **GetMatched(Line, [][]int) → *Matched** — pooled allocation
- **ReleaseMatched(*Matched)** — return to pool
- **IDGenerator** interface — `Next() → uint64`
- Files: `raw.go`, `matched.go`
- Imports: internal/ansi, btree

## pipeline/

Generic source→acceptor→destination pipeline.

- **New() → *Pipeline** — create pipeline
- **(*Pipeline).SetSource(Source)** — set data source
- **(*Pipeline).Add(Acceptor)** — add processing stage
- **(*Pipeline).SetDestination(Destination)** — set terminal stage
- **(*Pipeline).Run(ctx) → error** — execute pipeline
- Key interfaces: `Source` (`Start`, `Reset`), `Acceptor` (`Accept`), `Destination` (`Accept`, `Reset`, `Done`), `Suspender` (optional `Suspend`/`Resume`)
- **ChanOutput** (chan line.Line) — `Send(ctx, line.Line) → error`, `OutCh() → <-chan line.Line`
- **NewQueryContext(ctx, string) → ctx** / **QueryFromContext(ctx) → string** — pass query through context
- Files: `pipeline.go`
- Imports: line

## query/

Query text and caret management.

- **Text** — query string with save/restore: `Set(string)`, `Reset()`, `SaveQuery()`, `RestoreSavedQuery()`, `DeleteRange(int, int)`, `InsertAt(rune, int)`, `String()`, `Len()`, `RuneSlice()`, `RuneAt(int)`
- **Caret** — cursor position: `Pos() → int`, `SetPos(int)`, `Move(int)`
- Files: `query.go`
- Imports: (none internal)

## selection/

Ordered selection storage using btree.

- **New() → *Set** — create selection set
- **(*Set).Add(line.Line)** — add to selection
- **(*Set).Remove(line.Line)** — remove from selection
- **(*Set).Has(line.Line) → bool** — check membership
- **(*Set).Len() → int** — count selected
- **(*Set).Ascend(func(line.Line) bool)** — iterate in order
- **(*Set).Copy(dst *Set)** — copy all items
- **RangeStart** — range selection start marker: `Valid()`, `Value()`, `SetValue(int)`, `Reset()`
- Files: `selection.go`
- Imports: line, btree

## sig/

OS signal handling.

- **New(handler ReceivedHandler, sigs ...os.Signal) → *Handler**
- **(*Handler).Loop(ctx, func()) → error** — signal listening loop
- **ReceivedHandler** interface — `Handle(os.Signal)`
- Files: `sig.go`
- Imports: (none internal)

## internal/ansi

ANSI escape sequence parser.

- **Parse(string) → ParseResult** — strip ANSI, extract color spans
- **ExtractSegment([]AttrSpan, start, end int) → []AttrSpan** — slice attr spans for substring
- Key types: `ParseResult` (`Stripped string`, `Attrs []AttrSpan`), `AttrSpan` (`Fg, Bg Attribute`, `Length int`)
- Files: `parser.go`
- Imports: (none internal)

## internal/buffer

Line list buffer pool.

- **GetLineListBuf() → []line.Line** — get from pool
- **ReleaseLineListBuf([]line.Line)** — return to pool
- Files: `line.go`
- Imports: line

## internal/keyseq

Key sequence matching (multi-key bindings).

- **New() → *Keyseq** — create matcher (uses AhoCorasick internally)
- **(*Keyseq).Add(KeyList, any)** — register key sequence → action
- **(*Keyseq).Compile() → error** — build matcher
- **(*Keyseq).AcceptKey(Key) → (any, error)** — feed key, get action if matched
- **ToKeyList(string) → (KeyList, error)** — parse "C-x,C-c" → KeyList
- **KeyEventToString(KeyType, rune, ModifierKey) → (string, error)** — event → name
- Key types: `Key`, `KeyList`, `KeyType`, `ModifierKey`
- Matcher impls: Trie, TernarySearch, AhoCorasick (AhoCorasick used by default)
- Files: `keyseq.go`, `keys.go`, `trie.go`, `ternary.go`, `ahocorasick.go`
- Imports: (none internal)

## internal/util

Platform utilities.

- **IsTty(io.Reader) → bool** — check if reader is terminal
- **Homedir() → (string, error)** — user home directory
- **Shell(ctx, string) → *exec.Cmd** — create shell command
- **StripANSISequence(string) → string** — remove ANSI escapes (deprecated, use internal/ansi)
- **IsCollectResultsError(error) → bool** — check for collect-results sentinel
- **IsIgnorableError(error) → bool** — check for ignorable errors
- **GetExitStatus(error) → (int, bool)** — extract exit code
- Platform files: `tty_posix.go`, `tty_bsd.go`, `tty_windows.go`, `shell_unix.go`, `shell_windows.go`, `homedir_posix.go`, `homedir_darwin.go`, `homedir_windows.go`
- Files: `util.go`
- Imports: (none internal)
