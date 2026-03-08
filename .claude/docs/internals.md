<!-- Agent-consumed file. Keep terse, unambiguous, machine-parseable. -->

# Internals

## Concurrency Model

Three main goroutines coordinated via context cancellation:

1. **Input loop** (`input.go`) — reads terminal events, resolves key sequences via Keymap, dispatches actions
2. **View loop** (`view.go`) — renders screen in response to hub messages (draw, paging, status)
3. **Filter loop** (`filter.go`) — executes query against line buffer when query changes

Communication → **Hub** (`hub/`), central message bus with typed generic channels.

## Data Flow

```
stdin/file → Source → MemoryBuffer
                         ↓
User keystroke → Input → Hub.SendQuery() → Filter loop
                         ↓
Filter.Apply() → FilteredBuffer → Hub.SendDraw() → View loop
                         ↓
View → Layout → Screen (tcell) → terminal
```

## Hub Message Types

| Channel | Payload | Sender | Receiver |
|---------|---------|--------|----------|
| `QueryCh` | `string` | Input (action) | Filter loop |
| `DrawCh` | `*DrawOptions` | Filter, actions | View loop |
| `PagingCh` | `PagingRequest` | Input (action) | View loop |
| `StatusMsgCh` | `StatusMsg` | Various | View loop |

Hub supports **batch mode** — multiple sends within `Batch()` callback are processed together.

## Buffer Architecture

- `MemoryBuffer` — stores all input lines, grows as Source reads
- `FilteredBuffer` — wraps any Buffer with index range (page slice)
- `Source` — implements `pipeline.Source`, reads input lines into MemoryBuffer
- `ContextBuffer` — adds surrounding context lines for zoom view
- `CurrentLineBuffer()` returns raw MemoryBuffer (no filter) or FilteredBuffer (after filter)

## Filter Pipeline

1. Query change arrives via Hub
2. Filter loop creates pipeline: `MemoryBufferSource → filter.Apply → MemoryBuffer`
3. Filter.Apply runs in parallel chunks (if `SupportsParallel()`)
4. Results collected into new MemoryBuffer → set as CurrentLineBuffer
5. Hub.SendDraw() triggers View redraw

## Screen Abstraction

- `Screen` interface wraps terminal operations
- `TcellScreen` — production impl using tcell/v2
- `InlineScreen` — wraps TcellScreen for height-limited display
- `DummyScreen` — test mock with event injection

## Layout System

- `BasicLayout` composes: `UserPrompt` + `ListArea` + `StatusBar`
- Layout variants registered via `RegisterLayout(name, LayoutBuilder)`
- Built-in: `top-down` (default), `bottom-up`, `top-down-query-bottom`
- `AnchorSettings` controls vertical positioning (top/bottom anchor)

## Key Sequence Resolution

- `internal/keyseq.Keyseq` uses AhoCorasick matcher
- Supports multi-key sequences (e.g., C-x,C-c)
- Longest-match-wins semantics
- `InMiddleOfChain()` indicates partial match in progress

## Selection Model

- `selection.Set` uses `google/btree` for ordered storage by line ID
- Supports: single select, multi-select (toggle), range select, select-all
- Sticky selection — persists across query changes (configurable)

## Action System

- ~40 built-in actions in `action.go`
- Actions implement `Action` interface: `Execute(ctx, *Peco, Event)`
- `ActionFunc` — function adapter with `Register()` for key binding
- Combined actions — multiple actions bound to single key sequence
- `Keymap.LookupAction(Event) → Action` — resolve event to action

## State Objects

| State | Purpose |
|-------|---------|
| `Location` | Current page, line number, offset, per-page count |
| `SingleKeyJumpState` | Single-key jump mode toggle and prefix map |
| `ZoomState` | Zoom view buffer and line reference |
| `FrozenState` | Frozen source buffer for suspend/resume |
| `QueryExecState` | Query execution delay timer |

## Object Pools

- `line.GetMatched/ReleaseMatched` — pool for Matched line wrappers
- `internal/buffer.GetLineListBuf/ReleaseLineListBuf` — pool for line slices
