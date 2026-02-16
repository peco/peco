# Peco Codebase Review

Comprehensive review of redundancies, waste, and design issues in the peco codebase.
Each finding includes an importance rating, estimated fix cost, risk assessment, and
a verdict on whether fixing is recommended.

**Rating key:**
- Importance: Critical / High / Medium / Low
- Fix cost: Trivial / Small / Medium / Large
- Risk of fix: None / Low / Medium / High

Items marked **(DONE)** were fixed in previous work and are kept for reference.

---

## Table of Contents

1. [Architecture & Design](#1-architecture--design)
2. [Concurrency Issues](#2-concurrency-issues)
3. [Error Handling](#3-error-handling)
4. [Code Duplication](#4-code-duplication)
5. [Dead Code & Waste](#5-dead-code--waste)
6. [Interface Design](#6-interface-design)
7. [Dependencies & Build](#7-dependencies--build)
8. [Testing](#8-testing)
9. [Previously Fixed Items](#9-previously-fixed-items)
10. [Priority Summary](#10-priority-summary)

---

## 1. Architecture & Design

### 1.1 God Object — `Peco` Struct (32+ fields, 40+ methods)

**Location:** `interface.go:66-136` (type definition), `peco.go` (methods)

The `Peco` struct holds everything: I/O streams, config, UI state, concurrency
primitives, feature flags, and business logic coordination. Almost every function
in the codebase takes `*Peco` as a parameter. This is the root cause of many
downstream issues (tight coupling, testing difficulty, thread-safety concerns
from a single mutex guarding unrelated state).

| Attribute   | Value |
|-------------|-------|
| Importance  | **High** (structural/long-term) |
| Fix cost    | **Large** — careful decomposition into sub-structs |
| Risk of fix | Medium — wide-reaching refactor |
| Notes       | Could be done incrementally by extracting logical groups (e.g. `QueryState`, `UIState`, `FilterManager`) as embedded structs. This is the single highest-value structural improvement. |
| Verdict     | **Plan for major refactor** — incremental approach recommended |

### 1.2 Global Mutable State — Action and Layout Registries

**Location:** `action.go:23-26` (global maps), `action.go:68-170` (`init()` populating them), `layout.go:27-32` (layout registry)

40+ actions and 3 layouts are registered in package-level maps during `init()`. These
cannot be overridden per-instance. Tests share global state, preventing test isolation.

The `init()` function in `action.go` creates fresh maps every time. If the package is
loaded in tests multiple times, registrations restart. There's no `sync.Once` guard.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Medium** |
| Fix cost    | **Medium** — move registries into Peco or a dedicated type |
| Risk of fix | Low-Medium |
| Verdict     | **Fix when refactoring Peco struct** |

### 1.3 Layout Coupled to Full `*Peco`

**Location:** `layout.go` — `DrawScreen(*Peco, ...)`, `DrawPrompt(*Peco)`, `MovePage(*Peco, ...)`

Layout code reaches deep into `*Peco` to read styles, selection state, current line,
caret position, screen, filters, etc. This makes the layout untestable without a full
Peco instance and tightly couples rendering to the state machine.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Medium** |
| Fix cost    | **Medium** — define a `LayoutState` interface with just what layout needs |
| Risk of fix | Medium — touches many call sites |
| Verdict     | **Fix when refactoring Peco struct** |

### 1.4 Hub.Batch Swallows All Panics

**Location:** `hub/hub.go:67-68`

```go
defer func() { recover() }()
```

Any panic inside a batch callback is silently discarded. This can hide bugs during
development and make debugging production issues impossible.

| Attribute   | Value |
|-------------|-------|
| Importance  | **High** |
| Fix cost    | Small — log or propagate the panic |
| Risk of fix | Low |
| Verdict     | **Fix** — at minimum log the panic |

### 1.5 Hub Uses Context Values for Out-of-Band Signaling

**Location:** `hub/hub.go:50-51, 70, 89-96`

Batch mode is detected via `ctx.Value(batchPayloadKey{})`. This is a Go anti-pattern
— context values are meant for request-scoped metadata, not control flow. The
`operationNameKey` context value is only used for debug logging.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Medium — pass batch flag explicitly |
| Risk of fix | Low |
| Verdict     | **Consider** — functional but non-idiomatic |

### 1.6 `Peco.Run()` Orchestrates Everything Directly

**Location:** `peco.go:380-525`

`Run()` is a 145-line method that directly creates and coordinates all goroutines
(Input, View, Filter, signal handler, idgen, source setup), handles early-exit
modes (`--select-1`, `--exit-0`, `--select-all`), and manages initial query
execution. No abstraction for component lifecycle management.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Medium** |
| Fix cost    | **Medium** |
| Risk of fix | Medium |
| Verdict     | **Address during Peco refactor** |

---

## 2. Concurrency Issues

### 2.1 Hub `Payload.waitDone()` Potential Race

**Location:** `hub/hub.go:79-87`

```go
func (p *Payload[T]) waitDone() {
    // MAKE SURE p.done is valid. XXX needs locking?
    <-p.done
    ch := p.done
    p.done = nil
    defer doneChPool.Put(ch)
}
```

The `p.done` field is read/written without synchronization. While `waitDone()` runs
on the sender goroutine after sending, and `Done()` runs on the receiver, the field
is set in `send()` (line 107) and read in `waitDone()`. The channel receive provides
happens-before for the data, but the nil-write and pool-put happen after the receive
with no further synchronization. The code itself has an XXX comment acknowledging
this concern.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Medium** |
| Fix cost    | Small — add mutex or use atomic |
| Risk of fix | Low |
| Notes       | The pattern works in practice because the sender is blocked on `waitDone()` during the receiver's `Done()` call, so there's no true concurrent access. But it relies on subtle ordering guarantees. |
| Verdict     | **Fix** — remove the ambiguity, it's cheap insurance |

### 2.2 `context.Background()` Used Where Parent Context Should Propagate

**Location:** `peco.go:214, 791, 818, 858, 873, 896`

Several internal calls use `context.Background()` instead of the parent context.
This means context cancellation from shutdown doesn't propagate to these operations,
and they can outlive the intended lifecycle.

Examples:
- `peco.go:791` — `SetCurrentLineBuffer` sends draw via `context.Background()`
- `peco.go:818,858` — `sendQuery` and `ExecQuery` use `context.Background()`
- `peco.go:896` — timer callback uses `context.Background()`

| Attribute   | Value |
|-------------|-------|
| Importance  | **Medium** |
| Fix cost    | Small — pass parent context through |
| Risk of fix | Low |
| Verdict     | **Fix** |

### 2.3 Input Handler Alt Key Timer Race

**Location:** `input.go:56-92`

Timer + mutex state machine for Alt/Esc key detection. The generation counter
(`modGen`) mitigates stale timer callbacks, but `ExecuteAction()` at line 76 is
called without the mutex held. Between releasing the lock and the timer firing,
another key event can arrive and `Stop()` may return false (timer already queued).

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Medium — needs redesign of alt-key detection |
| Risk of fix | Medium — timing-sensitive code |
| Notes       | Works in practice for most users. The generation counter prevents executing stale actions. |
| Verdict     | **Consider** — works in practice, fix is non-trivial |

### 2.4 Filter Loop Doesn't Wait for Previous Work Goroutine

**Location:** `filter.go:490-520`

Each new query spawns `go f.Work(workctx, q)`. The previous goroutine is cancelled
but not waited for:

```go
previous = workcancel  // Cancel stored, but no WaitGroup
```

With rapid typing, multiple goroutines may briefly accumulate. The context cancellation
handles correctness, but resources aren't cleaned up eagerly.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small — add sync.WaitGroup |
| Risk of fix | Low |
| Notes       | The context cancellation prevents incorrect behavior. This is a resource tidiness issue, not a correctness issue. |
| Verdict     | **Consider** — minor improvement |

### 2.5 Query Exec Timer Callback Can Fire After Shutdown

**Location:** `peco.go:892-906, 913-921`

`stopQueryExecTimer()` calls `timer.Stop()`, but `Stop()` doesn't guarantee the
callback won't fire — it may already be queued. The callback calls `sendQuery()`
which spawns goroutines that may access torn-down state. The code acknowledges this
with a comment at line 519-521 but the fix (`Stop()`) isn't sufficient.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small — add context check at start of timer callback |
| Risk of fix | Low |
| Notes       | In practice, the 50ms delay and Go's scheduler make this extremely unlikely to cause issues. Adding `if ctx.Err() != nil { return }` at the callback start is sufficient. |
| Verdict     | **Fix** — cheap safety check |

---

## 3. Error Handling

### 3.1 External Filter Swallows Panics Silently

**Location:** `filter/external.go:55-61`

```go
defer func() {
    if err := recover(); err != nil {
        if pdebug.Enabled {
            pdebug.Printf("err: %s", err)
        }
    }
}()
```

Panics are only logged when pdebug is enabled (which it isn't in production builds).
In production, panics are completely swallowed. Same pattern at line 114 in the
reader goroutine.

| Attribute   | Value |
|-------------|-------|
| Importance  | **High** |
| Fix cost    | Small — log unconditionally or convert to error return |
| Risk of fix | Low |
| Verdict     | **Fix** |

### 3.2 Screen PollEvent Goroutine Swallows Panics

**Location:** `screen.go:271`

```go
defer func() { recover() }()
```

Same pattern — panic is recovered and discarded with no logging. If the event polling
goroutine panics, the entire input loop silently dies.

| Attribute   | Value |
|-------------|-------|
| Importance  | **High** |
| Fix cost    | Small |
| Risk of fix | Low |
| Verdict     | **Fix** |

### 3.3 Ignored `Write()` Errors **(DONE)**

**Location:** `peco.go:534, 956`

```go
p.Stdout.Write(opts.help())  // error ignored
// ...
p.Stdout.Write(buf.Bytes())  // error ignored in PrintResults
```

If stdout is a pipe and the reader closes, writes fail silently.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Trivial |
| Risk of fix | None |
| Verdict     | **Fix** — quick win |

### 3.4 Panics Instead of Error Returns in Layout **(DONE)**

**Location:** `layout.go` — `linesPerPage()` returns 2 when height is insufficient
(line 865), but earlier versions of `NewAnchorSettings`, `NewListArea`, etc. validate
their inputs and return errors properly. The `Draw` method at line 376 silently
returns on `perPage < 1`.

The `maxOf` helper at line 685 is a reimplementation of the stdlib `max()` built-in
available since Go 1.21.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Trivial — replace `maxOf` with `max` |
| Risk of fix | None |
| Verdict     | **Fix** — use stdlib `max()` |

---

## 4. Code Duplication

### 4.1 `acceptAndFilterSerial` vs `acceptAndFilterParallel` — Nearly Identical Loops **(DONE)**

**Location:** `filter.go:224-321`

These two functions have identical structure:
- Create flush channel and flusher goroutine
- Defer waiting for flusher
- Start ticker
- Loop reading from `in`, batching into `buf`, flushing on ticker or buffer full

The only difference is the channel type (`chan []line.Line` vs `chan orderedChunk`) and
sequence numbering. ~100 lines of duplicated loop logic.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Medium** |
| Fix cost    | Small-Medium — extract common loop with a generic flush callback |
| Risk of fix | Low |
| Notes       | Go generics could eliminate this cleanly. |
| Verdict     | **Fix** — meaningful reduction in maintenance surface |

### 4.2 Query-Exec-Then-Draw Pattern — 7 Duplicates **(DONE)**

**Location:** `action.go` — lines around `doRotateFilter`, `doDeleteBackwardWord`,
`doForwardWord`, `doKillBeginningOfLine`, `doKillEndOfLine`, `doDeleteForwardWord`,
`doDeleteBackwardChar`, `doToggleQuery`

Seven functions repeat this exact pattern:
```go
if state.ExecQuery(nil) {
    return
}
state.Hub().SendDrawPrompt(ctx)
```

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small — extract `execQueryAndRedraw(ctx, state)` helper |
| Risk of fix | Very low |
| Verdict     | **Fix** — quick cleanup |

### 4.3 pdebug Marker Blocks — ~40 Identical Blocks

**Location:** `action.go`, `peco.go`, `filter.go`, `layout.go`, `source.go`, `keymap.go`, and more

Always the same pattern:
```go
if pdebug.Enabled {
    g := pdebug.Marker("functionName")
    defer g.End()
}
```

~40 occurrences across the main source files. If pdebug is replaced (see 7.1),
this goes away automatically.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Resolved by fixing 7.1 |
| Verdict     | **Addressed by pdebug removal** |

### 4.4 Duplicate Comments in `query.go` **(DONE)**

**Location:** `query.go:45-48`

```go
// everything up to "start" is left in tact
// everything between start <-> end is deleted
// everything up to "start" is left in tact     // ← duplicate
// everything between start <-> end is deleted   // ← duplicate
```

Also has a typo: "in tact" should be "intact".

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Trivial |
| Risk of fix | None |
| Verdict     | **Fix** |

### 4.5 Platform-Specific `PostInit` — Identical No-ops **(DONE)**

**Location:** `screen_posix.go:6-10`, `screen_windows.go:3-6`

Both files contain identical no-op `PostInit()` implementations. After the tcell
migration, there's no platform-specific behavior. The build tags serve no purpose.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Trivial — consolidate into `screen.go` |
| Risk of fix | None |
| Verdict     | **Fix** — removes unnecessary platform split |

---

## 5. Dead Code & Waste

### 5.1 Unused Keyseq Implementation — AhoCorasick

**Location:** `internal/keyseq/ahocorasick.go`

Three key-sequence matching algorithms exist: Trie (via `NewTrie()`), TernaryTrie,
and AhoCorasick. `NewTrie()` at `trie.go:15-16` delegates to `NewTernaryTrie()`.
AhoCorasick is only constructed in its own test file. It wraps `TernaryTrie` and
adds failure-link matching, but is never used at runtime.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Trivial — delete `ahocorasick.go` and its test |
| Risk of fix | None |
| Notes       | The TernaryTrie tests also cover the core functionality. AhoCorasick may be intended for future use or serves as a reference implementation. The code is small, self-contained, and causes no harm. |
| Verdict     | **Won't fix** — not worth removing; keeping as a reference implementation is fine |

### 5.2 Deprecated Config Fields Still in Struct

**Location:** `interface.go:314-315, 322`

`Matcher` (deprecated in favor of `InitialFilter`), `InitialMatcher` (deprecated in
favor of `InitialFilter`), and `CustomMatcher` (deprecated in favor of `CustomFilter`)
are still defined in the `Config` struct and handled in `config.go`.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small |
| Risk of fix | **Medium** — breaks backward compat for users with old configs |
| Notes       | Consider how long these have been deprecated. If multiple versions, safe to remove. |
| Verdict     | **Remove in next major version** |

### 5.3 `skipReadConfig` — Test-Only Field in Production Struct

**Location:** `interface.go:112`, `peco.go:331`

```go
if !p.skipReadConfig {  // This can only be set via test
```

Production code contains a test-only escape hatch. Tests should inject a mock config
reader instead of having the production struct carry a test flag.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small — use a config reader function that can be swapped in tests |
| Risk of fix | Low |
| Verdict     | **Consider** — minor hygiene improvement |

### 5.4 `maxOf` Reimplements `max()` Built-in **(DONE)**

**Location:** `layout.go:685-690`

```go
func maxOf(a, b int) int {
    if a > b { return a }
    return b
}
```

Go 1.21+ (the project targets 1.24) provides `max()` as a built-in.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Trivial |
| Risk of fix | None |
| Verdict     | **Fix** |

### 5.5 `extraOffset` Global Variable

**Location:** `layout.go:17`

```go
var extraOffset int = 0
```

Package-level mutable variable. Used in layout calculations but only ever 0 in the
current code. It appears to be a leftover from a feature that was removed or never
completed. It's used in `linesPerPage()`, `NewBottomUpLayout()`, and
`NewTopDownQueryBottomLayout()`.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Trivial — remove and inline 0 |
| Risk of fix | Low |
| Notes       | Check if inline screen mode sets this. If `screen_inline.go` modifies it, it should be a configuration field, not a global. |
| Verdict     | **Investigate and fix** |

---

## 6. Interface Design

### 6.1 `Layout` Interface — Only One Implementation

**Location:** `interface.go:206-213`

```go
type Layout interface {
    PrintStatus(string, time.Duration)
    DrawPrompt(*Peco)
    DrawScreen(*Peco, *hub.DrawOptions)
    MovePage(*Peco, hub.PagingRequest) (moved bool)
    PurgeDisplayCache()
    SortTopDown() bool
}
```

`BasicLayout` is the only implementation. Layout variants (top-down, bottom-up,
query-bottom) are handled by configuration within `BasicLayout`, not separate types.
The layout registry pattern adds indirection without benefit.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small |
| Risk of fix | Low |
| Notes       | Removing the interface simplifies code but reduces future extensibility. |
| Verdict     | **Consider** — not urgent |

### 6.2 `Action` Interface Mixes Registration and Execution

**Location:** `interface.go:288-292`

```go
type Action interface {
    Register(string, ...keyseq.KeyType)
    RegisterKeySequence(string, keyseq.KeyList)
    Execute(context.Context, *Peco, Event)
}
```

Registration is only called during `init()`. The interface should only require
`Execute`. Registration is a setup concern, not a runtime concern.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Medium** |
| Fix cost    | Medium |
| Risk of fix | Low |
| Verdict     | **Fix** — split registration from execution |

### 6.3 `MessageHub` Interface — 11 Methods

**Location:** `interface.go:525-537`

Mixes sender methods (`SendDraw`, `SendQuery`, ...) with channel getters (`DrawCh`,
`QueryCh`, ...) and coordination (`Batch`). Also has redundant `SendStatusMsg` /
`SendStatusMsgAndClear` that differ only by a duration parameter.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Medium** |
| Fix cost    | Medium |
| Risk of fix | Medium |
| Notes       | Could split into `HubSender` and `HubReceiver`. Merge the StatusMsg methods. |
| Verdict     | **Fix when refactoring hub** |

### 6.4 `StatusBar` Interface — Trivial

**Location:** `interface.go:232-234`

Single-method interface with only two implementations: `screenStatusBar` and
`nullStatusBar`. The null variant is a no-op. This could be a simple nil check.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small |
| Risk of fix | Low |
| Verdict     | **Consider** — standard Go pattern (null object), minor |

### 6.5 `Buffer` Interface Has Unexported Method

**Location:** `interface.go:492-496`

```go
type Buffer interface {
    linesInRange(int, int) []line.Line  // unexported
    LineAt(int) (line.Line, error)
    Size() int
}
```

Unexported method in an exported interface prevents external implementation.
If intentional (restricts to this package), it should be documented. If not,
export or remove.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small |
| Risk of fix | Low |
| Verdict     | **Consider** |

### 6.6 `MatchIndexer` Interface — Barely Used

**Location:** `interface.go:145-150`

Used via type assertion in a single place (`layout.go:592`). The `line.Matched` type
has an `Indices()` method but the interface adds little value for a single-use type
assertion.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small |
| Risk of fix | Low |
| Verdict     | **Consider** |

---

## 7. Dependencies & Build

### 7.1 `github.com/lestrrat-go/pdebug` — Unmaintained, Runtime Overhead

**Location:** `go.mod:10` — 14+ source files, ~40 guard blocks

2018-era debug library. Every guard block adds a branch even when disabled (the
`Enabled` constant is always `false` in production, but the `if` is still compiled).
The pattern is always identical:

```go
if pdebug.Enabled {
    g := pdebug.Marker("functionName")
    defer g.End()
}
```

| Attribute   | Value |
|-------------|-------|
| Importance  | **Medium** |
| Fix cost    | **Medium** — replace with `log/slog` or build-tag-based debug, ~40 sites |
| Risk of fix | Low |
| Notes       | Could use `//go:build debug` conditional compilation for zero cost when disabled. The `Enabled` const means the compiler already dead-code-eliminates the blocks, but the source noise remains. |
| Verdict     | **Fix** — removes unmaintained dep, cleans up code |

### 7.2 Makefile Issues **(DONE)**

**Location:** `Makefile`

1. **Unnecessary `GO111MODULE=on`** — Set on 3 lines. Default since Go 1.16.
2. **No `-race` flag in test target** — Race detection not enabled.
3. **No `go generate` before test** — Stringer-generated files can get out of sync.
4. **No test coverage target** — No way to measure coverage.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Trivial per item |
| Risk of fix | None |
| Verdict     | **Fix** — all quick wins |

### 7.3 Generated Stringer Files Committed

**Location:** `stringer_vertical_anchor.go`, `hub/pagingrequesttype_string.go`

These are auto-generated but committed. If someone adds a new enum value and forgets
`go generate`, the string representation is silently wrong. No CI check for staleness.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small — add `go generate` to Makefile test target |
| Risk of fix | None |
| Verdict     | **Fix** |

---

## 8. Testing

### 8.1 Major Test Coverage Gaps **(DONE)**

The following packages/files have **no tests**:

**Packages with no test files:**
- `hub/` — the central message bus (critical concurrency code)
- `filter/` — filter implementations (the core feature)
- `internal/util/` — platform-specific TTY detection, shell integration
- `internal/buffer/` — buffer pool utilities
- `pipeline/` — pipeline orchestration
- `cmd/peco/` — entry point
- `cmd/filterbench/` — benchmark utility

**Root package files with no dedicated tests:**
- `view.go` — main view loop
- `caret.go` — cursor position tracking
- `query.go` — query buffer operations
- `filter.go` — filter management / incremental filtering
- `keymap.go` — key binding system
- `screen.go` — terminal abstraction
- `layout.go` — UI layout (partially tested in `layout_test.go`)

| Attribute   | Value |
|-------------|-------|
| Importance  | **High** |
| Fix cost    | **Large** — writing tests for all untested code is significant effort |
| Risk of fix | Low |
| Notes       | Priority should be: hub (concurrency-critical), filter (core feature), query/caret (state management). The `DummyScreen` test helper already exists for UI testing. |
| Verdict     | **Fix incrementally** — start with hub and filter packages |

### 8.2 Excessive `time.Sleep` in Tests **(DONE)**

**Location:** `action_test.go` — ~28 instances with 500ms-1s durations

Tests use hardcoded sleep durations for synchronization. This is:
1. Fragile (can fail on slow CI)
2. Slow (each 500ms sleep adds up)
3. Non-deterministic

| Attribute   | Value |
|-------------|-------|
| Importance  | **Medium** |
| Fix cost    | Medium — each sleep needs a proper synchronization replacement |
| Risk of fix | Medium — can introduce flakiness if done wrong |
| Notes       | Could use `require.Eventually()` from testify, or channel-based synchronization. |
| Verdict     | **Fix incrementally** — high value for CI speed and reliability |

### 8.3 Duplicated Test Setup

**Location:** `action_test.go` — ~15 instances of identical setup block

Almost every test repeats:
```go
state := newPeco()
ctx, cancel := context.WithCancel(context.Background())
go state.Run(ctx)
defer cancel()
<-state.Ready()
```

| Attribute   | Value |
|-------------|-------|
| Importance  | **Medium** |
| Fix cost    | Small — extract `setupTest(t)` helper returning `(state, ctx, cancel)` |
| Risk of fix | Very low |
| Verdict     | **Fix** — reduces boilerplate |

### 8.4 Tests Checking Implementation Details

**Location:** `action_test.go` — `TestActionNames`

```go
for _, name := range names {
    if _, ok := nameToActions[name]; !ok {
        t.Errorf(...)
    }
}
```

Directly checks the global `nameToActions` map — an implementation detail.
Better to test that key bindings resolve to correct actions.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small |
| Risk of fix | Low |
| Verdict     | **Consider** — functional but couples test to implementation |

### 8.5 Incomplete Test Cases

**Location:** `action_test.go` — `TestRotateFilter`

```go
// TODO toggle ExecQuery()
```

Test ends with unfinished TODO. `TestGHIssue331` in `peco_test.go` also acknowledges
it's not testing the intended behavior:

```go
// Note: we should check that the drawing process did not
// use cached display, but ATM this seemed hard to do
```

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Medium per test |
| Risk of fix | Low |
| Verdict     | **Fix when touching these tests** |

---

## 9. Previously Fixed Items

These were identified in earlier reviews and have been resolved. Kept for reference.

| ID | Issue | Status |
|----|-------|--------|
| ~~1.1~~ | Fragile manual close of frozen buffer `done` channel | **DONE** — `MarkComplete()` added |
| ~~1.2~~ | Resume deadlock in Screen | **DONE** — context-guarded select |
| ~~1.3~~ | External filter reader goroutine leak | **DONE** — `sync.WaitGroup` added |
| ~~1.4~~ | Query exec timer leak | **DONE** — `stopQueryExecTimer()` on shutdown |
| ~~2.1~~ | `github.com/pkg/errors` deprecated | **DONE** — replaced with `fmt.Errorf` |
| ~~2.3~~ | `runtime.GOMAXPROCS(runtime.NumCPU())` unnecessary | **DONE** — removed |
| ~~4.1~~ | Hub channels used `interface{}` | **DONE** — generic `Payload[T]` types |
| ~~4.2~~ | `OnCancel` was stringly typed | **DONE** — typed constants |
| ~~4.4~~ | Draw messages used magic strings | **DONE** — `DrawOptions` struct |
| ~~5.1~~ | `State` interface unused | **DONE** — deleted |
| ~~6.4~~ | Filter Apply/ApplyCollect duplication | **DONE** — shared base |
| ~~6.5~~ | Selection navigation duplication | **DONE** — unified with direction param |
| ~~6.6~~ | Freeze/unfreeze shared logic | **DONE** — `resetQueryState()` extracted |
| ~~6.7~~ | Batch action pattern duplication | **DONE** — `batchAction()` extracted |
| ~~6.8~~ | Regexp filter constructor duplication | **DONE** — delegated constructors |
| ~~7.1~~ | Goroutine leak in screen suspend handler | **DONE** — `doneCh` added |
| ~~7.4~~ | Missing `signal.Stop()` cleanup | **DONE** |
| ~~8.2~~ | Manual error chain unwrapping | **DONE** — uses `errors.Unwrap()` |
| ~~8.5~~ | `interface{}` throughout pipeline | **DONE** — typed `ChanOutput chan line.Line` |
| ~~9.5~~ | Residual "Termbox" naming | **DONE** — renamed to `TcellScreen` |
| ~~3.3~~ | Ignored `Write()` errors | **DONE** — help checks error; PrintResults explicitly ignores |
| ~~3.4~~ | `maxOf` helper reimplements `max()` | **DONE** — replaced with built-in `max()` |
| ~~4.1~~ | Serial vs parallel filter loop duplication | **DONE** — refactored with shared flusher |
| ~~4.2~~ | Query-exec-draw pattern (7 duplicates) | **DONE** — `execQueryAndDraw()` extracted |
| ~~4.4~~ | Duplicate comments + typo in `query.go` | **DONE** — deduplicated, "in tact" → "intact" |
| ~~4.5~~ | Platform-specific `PostInit` identical no-ops | **DONE** — consolidated, platform files removed |
| ~~5.4~~ | `maxOf` reimplements `max()` built-in | **DONE** — removed, using built-in `max()` |
| ~~7.2~~ | Makefile issues (GO111MODULE, -race, generate, coverage) | **DONE** — all four sub-items fixed |
| ~~8.1~~ | Major test coverage gaps | **DONE** — tests added for query, caret, util, buffer |
| ~~8.2~~ | Excessive `time.Sleep` in tests | **DONE** — replaced with `require.Eventually()` |

---

## 10. Priority Summary

### Fix Now (High Impact, Low Cost)

| # | Issue | Importance | Cost |
|---|-------|-----------|------|
| 1.4 | Hub.Batch swallows panics | High | Small |
| 3.1 | External filter swallows panics | High | Small |
| 3.2 | Screen PollEvent swallows panics | High | Small |
| 2.2 | `context.Background()` should propagate parent | Medium | Small |
| 2.5 | Timer callback context check | Low | Trivial |

### Quick Wins (Low Cost, Low Risk)

| # | Issue | Importance | Cost |
|---|-------|-----------|------|
| *(none remaining)* | | | |

### Medium Effort, High Value

| # | Issue | Importance | Cost |
|---|-------|-----------|------|
| 7.1 | Remove pdebug dependency | Medium | Medium |
| 8.3 | Extract test setup helper | Medium | Small |
| 6.2 | Split Action interface (registration vs execution) | Medium | Medium |
| 6.3 | Split/simplify MessageHub interface | Medium | Medium |

### Major Refactor (Plan Carefully)

| # | Issue | Importance | Cost |
|---|-------|-----------|------|
| 1.1 | Decompose Peco God Object | High | Large |
| 1.3 | Decouple Layout from *Peco | Medium | Medium |
| 1.2 | Move registries into Peco instance | Medium | Medium |
| 1.6 | Simplify Run() orchestration | Medium | Medium |

### Low Priority / Consider

| # | Issue | Importance | Cost |
|---|-------|-----------|------|
| 1.5 | Hub context values for control flow | Low | Medium |
| 2.1 | Hub Payload.waitDone() race | Medium | Small |
| 2.3 | Alt key timer race | Low | Medium |
| 2.4 | Filter work goroutine not waited for | Low | Small |
| 5.2 | Remove deprecated config fields | Low | Small |
| 5.3 | Remove `skipReadConfig` test-only field | Low | Small |
| 5.5 | Remove `extraOffset` global | Low | Trivial |
| 6.1 | Remove Layout interface | Low | Small |
| 6.4 | Simplify StatusBar | Low | Small |
| 6.5 | Buffer unexported method | Low | Small |
| 6.6 | MatchIndexer minimal use | Low | Small |
| 8.4 | Tests checking implementation details | Low | Small |
| 8.5 | Incomplete test TODOs | Low | Medium |
