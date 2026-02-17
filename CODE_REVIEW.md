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

### 1.1 God Object — `Peco` Struct (32+ fields, 40+ methods) **(DONE)**

**Location:** `interface.go:66-125` (type definition), `peco.go` (methods)

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
| Notes       | The original assessment of "32+ fields, 40+ methods" was understated — the actual count was ~50 fields and ~54 methods. Decomposition extracted 4 sub-structs following the existing codebase pattern (`Query`, `Caret`, `Location`, `Selection`). |
| Verdict     | **Done** — problematic state clusters extracted; remaining fields are cohesive |

**Resolution:** Extracted 4 field groups into named sub-structs with accessor methods (all in `state.go`):

| Sub-struct | Fields moved | Methods moved/added | Own mutex? |
|------------|-------------|-------------------|-----------|
| `SingleKeyJumpState` | 4 (`mode`, `showPrefix`, `prefixes`, `prefixMap`) | 5 methods (`Mode`, `SetMode`, `ShowPrefix`, `Prefixes`, `Index`) + 1 accessor | No |
| `ZoomState` | 2 (`buffer`, `lineNo`) | 4 methods (`Buffer`, `LineNo`, `Set`, `Clear`) + 1 accessor | Yes (split from `p.mutex`) |
| `FrozenState` | 1 (`source`) | 3 methods (`Source`, `Set`, `Clear`) + 1 accessor | Yes (split from `p.mutex`) |
| `QueryExecState` | 3 (`delay`, `mutex`, `timer`) | 2 methods (`Delay`, `StopTimer`) + 1 accessor | Yes (already had own mutex) |

**Impact:**
- **Peco field count:** ~50 → 48 (10 fields consolidated into 4 sub-struct fields, net -2 after adding sub-struct fields + `setCurrentLineBufferNoNotify`)
- **Peco method count:** ~54 → 45 (14 methods moved/removed, 5 added: 4 accessors + `setCurrentLineBufferNoNotify`)
- **`p.mutex` scope reduced:** Previously guarded 5+ unrelated field groups (`resultCh`, `frozenSource`, `preZoomBuffer`+`preZoomLineNo`, `keymap`, `currentLineBuffer`). Now guards only 3 coherent groups: `resultCh`, `keymap`, `currentLineBuffer`
- **Direct `state.mutex` access eliminated from `action.go`:** ZoomIn/ZoomOut now use `state.setCurrentLineBufferNoNotify()` instead of directly locking `state.mutex`

**Further decomposition evaluated and declined:** The remaining 48 fields break down as: 8 already-extracted sub-structs, 14 essential infrastructure fields, 4 I/O fields, and 22 config-derived read-only flags (set once in `ApplyConfig`, never mutated after). Extracting config flags would add indirection without improving thread safety (already effectively immutable), testability, or method count. Splitting `p.mutex` into per-field mutexes would add complexity for 3 simple get/set patterns with no contention. The struct is large but cohesive — it is the application orchestrator, and the remaining fields genuinely belong to it.

### 1.2 Global Mutable State — Action and Layout Registries **(DONE)**

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
| Notes       | Assessment overstated. The package-level registry pattern is idiomatic Go (`http.Handle`, `sql.Register`, `image.RegisterFormat`). `init()` is guaranteed by Go to run exactly once — no `sync.Once` needed. Layout registry is immutable after init() (no issue). The ONE real problem was `resolveActionName()` in `keymap.go` caching combined actions back into the global `nameToActions` map, making it mutable post-init. Moving registries into Peco would bloat initialization for no benefit — actions are inherently application-global. |
| Verdict     | **Done** — removed post-init write to global map; registries now truly immutable after init() |

**Resolution:** Removed `nameToActions[name] = v` caching line from `resolveActionName()` in `keymap.go`. Combined actions from user config are now constructed on each resolution during `ApplyKeybinding()` (negligible startup-time cost). Global maps (`nameToActions`, `defaultKeyBinding`, `layoutRegistry`) are now all immutable after `init()`. Moving registries into Peco was declined as over-engineering — the pattern is idiomatic Go and there is only ever one Peco instance per process.

### 1.3 Layout Coupled to Full `*Peco` **(DONE)**

**Location:** `layout.go` — `DrawScreen(*Peco, ...)`, `DrawPrompt(*Peco)`, `MovePage(*Peco, ...)`

Layout code reaches deep into `*Peco` to read styles, selection state, current line,
caret position, screen, filters, etc. This makes the layout untestable without a full
Peco instance and tightly couples rendering to the state machine.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Medium** |
| Fix cost    | **Medium** — define a `LayoutState` interface with just what layout needs |
| Risk of fix | Medium — touches many call sites |
| Notes       | Assessment partially accurate. Layout accesses Peco through 11 well-defined accessor methods (Caret, Query, Location, Selection, etc.) which is clean and appropriate. The real issue was 3 direct field accesses bypassing the public API. A `LayoutState` interface (11+ methods) would violate Go's small-interface idiom and add no real value since Layout and Peco are in the same package. |
| Verdict     | **Done** — direct field accesses fixed; `LayoutState` interface declined as over-engineering |

**Resolution:** Replaced 3 direct field accesses in `layout.go` with getter methods:
- `state.screen.Size()` → `state.Screen().Size()` (2 call sites; method already existed)
- `state.selectionPrefix` → `state.SelectionPrefix()` (new getter added)
- `state.config.SuppressStatusMsg` → `state.SuppressStatusMsg()` (new getter added)

Layout now accesses Peco exclusively through public accessor methods. The suggested `LayoutState` interface was evaluated and declined: it would be an 11-method interface (Go anti-pattern), provide no package-boundary benefit (same package), and tests already work fine with `New()` + dummy screen.

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

### 1.5 Hub Uses Context Values for Out-of-Band Signaling **(PARTIALLY FIXED)**

**Location:** `hub/hub.go:50-51, 70, 89-96`

Batch mode is detected via `ctx.Value(batchPayloadKey{})`. The original assessment
called this a Go anti-pattern, but on closer evaluation the `batchPayloadKey` context
value is actually a **legitimate scope-propagation pattern** — `Hub.Batch()` wraps an
arbitrary callback and needs to signal batch mode to whatever `Send*` methods get called
inside it. This is the same pattern used by `database/sql` for transactions and
OpenTelemetry for trace spans. Eliminating this context value would require either
changing the `Send*` method signatures or creating duplicated batch variants, with no
real benefit.

The `operationNameKey` context value was only used for pdebug logging inside `send()`
and only set by `SendQuery` — it was dead weight.

**What was actually wrong:** The `send()` function redundantly checked `isBatchCtx(ctx)`
when every `Send*` caller already extracted the batch flag from context and passed it
into the payload via `NewPayload(..., isBatchCtx(ctx))`. The `send()` function could
simply use `r.Batch()` instead. Additionally, `send()` used a bare `ch <- r` which
could block forever if the context was cancelled (e.g. during shutdown).

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small — use `r.Batch()` in `send()`, add `ctx.Done()` select |
| Risk of fix | Low |
| Verdict     | **Partially fixed** — removed redundant batch check and dead `operationNameKey`; added context cancellation to `send()`; the `batchPayloadKey` context value in `Batch()` → `Send*()` is kept as it's idiomatic scope propagation |

**Resolution:** Removed `operationNameKey` type (unused debug-only context key). Simplified `send()` to use `r.Batch()` instead of redundantly checking `isBatchCtx(ctx)`. The context parameter is kept in `send()` but now used properly — for cancellation via `select` on `ctx.Done()`, preventing sends from blocking forever during shutdown. The `batchPayloadKey` context value remains as the mechanism for `Batch()` to propagate scope to `Send*` calls, which is an appropriate use of context values.

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

### 2.1 Hub `Payload.waitDone()` Potential Race **(DONE)**

**Location:** `hub/hub.go:78-92`

The original code read `p.done` *after* the blocking receive, making the synchronization
reasoning subtle:

```go
func (p *Payload[T]) waitDone() {
    // MAKE SURE p.done is valid. XXX needs locking?
    <-p.done
    ch := p.done   // reads p.done after receive — safe but non-obvious
    p.done = nil
    defer doneChPool.Put(ch)
}
```

**Evaluation:** There is no actual data race per the Go memory model — channel operations
provide all necessary happens-before guarantees. `send()` sets `p.done` sequentially
before sending the payload on the hub channel; the receiver's `Done()` sends on the
channel before `waitDone()` receives from it; after the receive, the receiver is done
with `p.done`. However, the code was unnecessarily subtle (the author's own XXX comment
acknowledged uncertainty), and the `defer doneChPool.Put(ch)` at the end of the function
was a gratuitous defer.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** (no actual race, but code clarity issue) |
| Fix cost    | Trivial — reorder to save channel reference before blocking |
| Risk of fix | None |
| Notes       | The original assessment called this "Medium" importance, but analysis confirms there is no data race under the Go memory model. The fix is still worthwhile for clarity. |
| Verdict     | **Fixed** — reordered to save channel before receive, removed XXX comment |

**Resolution:** Reordered `waitDone()` to save the `p.done` channel reference into a
local variable *before* blocking on receive. This makes the safety trivially obvious:
the read is on the same goroutine that set the field in `send()`, and the write
(`p.done = nil`) happens after the channel receive which guarantees the receiver is done.
Removed the XXX comment, added clear synchronization comments, and replaced the
unnecessary `defer doneChPool.Put(ch)` with a direct call.

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

### 2.3 Input Handler Alt Key Timer Race **(DONE)**

**Location:** `input.go:56-92`

Timer + mutex state machine for Alt/Esc key detection. The generation counter
(`modGen`) mitigates stale timer callbacks, but the timer callback called
`ExecuteAction()` directly on the timer goroutine (outside the mutex), allowing
concurrent execution with the input loop goroutine's `ExecuteAction()` calls.

The generation counter correctly prevents the most dangerous scenario (stale Esc
action firing after being superseded by Alt+key). The remaining concern was
concurrent `ExecuteAction` calls from the timer goroutine and the input loop
goroutine. While safe in practice (actions communicate through the channel-based
hub), this was unnecessary — the fix was simpler than originally estimated.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | ~~Medium — needs redesign of alt-key detection~~ **Small** — route timer callback through a channel |
| Risk of fix | ~~Medium — timing-sensitive code~~ **Low** — the timing heuristic is unchanged |
| Notes       | The original assessment overestimated fix cost. No redesign of alt-key detection was needed — just routing the timer callback's event through a buffered channel back to the input loop goroutine. |
| Verdict     | **Fix** |

**Resolution:** Added a `pendingEsc chan Event` (buffered, size 1) to the `Input` struct. The timer callback now sends the Esc event to this channel instead of calling `ExecuteAction` directly. The `Loop` method reads from `pendingEsc` in its select, executing the action on the input loop goroutine. This serializes all `ExecuteAction` calls, eliminating concurrent action execution while preserving the 50ms Alt/Esc timing heuristic unchanged.

### 2.4 Filter Loop Doesn't Wait for Previous Work Goroutine **(DONE)**

**Location:** `filter.go:490-525`

Each new query spawns `go f.Work(workctx, q)`. The previous goroutine is cancelled
but not waited for. With rapid typing, multiple goroutines may briefly accumulate.
Context cancellation handles correctness, but `Loop()` could return (on shutdown)
while `Work()` goroutines are still winding down.

**Evaluation:** The assessment is correct. The fix is NOT to wait for previous work
before spawning new work (that would hurt responsiveness during rapid typing). Instead,
a `sync.WaitGroup` tracks all in-flight `Work()` goroutines, and `defer wg.Wait()` at
the top of `Loop()` ensures clean shutdown — `Loop()` waits for all goroutines to
finish before returning. New queries still start immediately without waiting.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Trivial — 4 lines added |
| Risk of fix | None |
| Notes       | Context cancellation still handles correctness. The WaitGroup only ensures `Loop()` doesn't return while goroutines are still running. |
| Verdict     | **Fixed** |

**Resolution:** Added a `sync.WaitGroup` to `Filter.Loop()`. Each `Work()` goroutine increments the counter before starting and decrements it via `defer wg.Done()`. `defer wg.Wait()` at the top of `Loop()` ensures all goroutines finish before `Loop()` returns. No impact on responsiveness — new queries are never blocked on previous ones completing.

### 2.5 Query Exec Timer Callback Can Fire After Shutdown **(DONE)**

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
| Notes       | In practice, the 50ms delay and Go's scheduler make this extremely unlikely to cause issues. ~~Adding `if ctx.Err() != nil { return }` at the callback start is sufficient.~~ The callback closure does not capture a parent context (it uses `context.Background()`), so checking `ctx.Err()` is not possible. Instead, the fix moves the mutex acquisition to the top of the callback and checks if `queryExecTimer` is nil (set by `stopQueryExecTimer` during shutdown). |
| Verdict     | **Fix** — cheap safety check |

**Resolution:** The timer callback now acquires `queryExecMutex` at the start and checks whether `queryExecTimer` is nil. If `stopQueryExecTimer` already cleared it during shutdown, the callback returns immediately without calling `sendQuery`.

---

## 3. Error Handling

### 3.1 External Filter Swallows Panics Silently **(DONE)**

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

**Resolution:** Panics are now converted to proper error returns via `fmt.Errorf("panic in external filter %q: %v", ...)`. The reader goroutine panic is also captured into `readerPanicErr` and propagated.

### 3.2 Screen PollEvent Goroutine Swallows Panics **(DONE)**

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

**Resolution:** Panics are now logged with full stack trace via `fmt.Fprintf(t.errWriter, "peco: panic in PollEvent goroutine: %v\n%s", r, debug.Stack())`.

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

### 5.2 Deprecated Config Fields Still in Struct **(DONE)**

**Location:** `interface.go:314-315, 322`

`Matcher` (deprecated in favor of `InitialFilter`), `InitialMatcher` (deprecated in
favor of `InitialFilter`), and `CustomMatcher` (deprecated in favor of `CustomFilter`)
were still defined in the `Config` struct and handled in `config.go`.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small |
| Risk of fix | ~~**Medium** — breaks backward compat for users with old configs~~ **Low** — all three fields were already inert or silently ignored by Go's JSON/YAML decoders after removal |
| Notes       | These have been deprecated since **2017** (9 years). `Config.Matcher` was never read anywhere. `Config.InitialMatcher` was set as a default in `Init()` but the runtime fallback chain in `peco.go` skipped it entirely (only checked CLI `--initial-filter`, then config `InitialFilter`, then CLI `--initial-matcher`). Only `CustomMatcher` had active conversion logic. The `RotateMatcher` deprecated action alias remains in `action.go` as a separate concern. |
| Verdict     | **Fix** |

**Resolution:** Removed all three deprecated fields (`Config.Matcher`, `Config.InitialMatcher`, `Config.CustomMatcher`) and the `CustomMatcher` conversion logic. Removed `CLIOptions.OptInitialMatcher` (`--initial-matcher` flag) and its fallback in `peco.go`. Removed `Config.Init()` setting `InitialMatcher` default. Old config files with these keys are silently ignored by Go's JSON/YAML decoders, consistent behavior across all three removed fields.

### 5.3 `skipReadConfig` — Test-Only Field in Production Struct **(DONE)**

**Location:** `interface.go:112`, `peco.go:331`

Production code contained a test-only boolean escape hatch. The field was necessary
because `parseCommandLine` auto-detects config files via `LocateRcfile()` — without
the flag, tests would pick up the developer's personal `~/.config/peco/config.json`.

**Evaluation:** The assessment is correct. The boolean flag is a real need (not just
paranoia) but the wrong abstraction. Replacing it with a function field is proper
dependency injection — more idiomatic Go. As a bonus, the 5 layout tests that set
`skipReadConfig = true` defensively never called `Run()`, so those lines were dead code.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small |
| Risk of fix | None |
| Verdict     | **Fixed** |

**Resolution:** Replaced `skipReadConfig bool` with `readConfigFn func(*Config, string) error`. `New()` defaults to the real `readConfig`. Tests inject a no-op function. The `issues_test.go` test that needed config reading re-enables it by setting `readConfigFn = readConfig`. Removed 5 dead `skipReadConfig = true` assignments from layout tests that never called `Run()`.

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

### 5.5 `extraOffset` Global Variable **(DONE)**

**Location:** `layout.go:17`, `layout_windows.go`

```go
var extraOffset int = 0
```

Package-level mutable variable. Used in layout calculations but only ever 0 in the
current code. It appears to be a leftover from a feature that was removed or never
completed. It's used in `linesPerPage()`, `NewBottomUpLayout()`, and
`NewTopDownQueryBottomLayout()`.

**Evaluation:** The original assessment was **incomplete**. `extraOffset` is NOT always 0 —
`layout_windows.go` sets it to 1 via `init()` to reserve an extra line at the bottom for
Windows terminal rendering. `screen_inline.go` does NOT modify it. The variable is set
once during init and never changed at runtime, making it effectively a per-platform constant
masquerading as a mutable global.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Trivial |
| Risk of fix | None |
| Notes       | The original assessment missed `layout_windows.go` which sets `extraOffset = 1`. `screen_inline.go` does not modify it. The variable is effectively a per-platform constant. |
| Verdict     | **Fixed** — converted to platform-specific constants |

**Resolution:** Replaced the mutable global `var extraOffset int = 0` with platform-specific
constants: `const extraOffset = 0` in `layout_notwindows.go` (with `//go:build !windows`) and
`const extraOffset = 1` in `layout_windows.go`. The `init()` function in `layout_windows.go`
was removed. This makes the platform-specific behavior explicit and eliminates the mutable
global while preserving the Windows layout offset.

---

## 6. Interface Design

### 6.1 `Layout` Interface — Only One Implementation **(Won't fix)**

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

**Evaluation:** While technically correct that there's only one implementation, the interface
and registry are well-designed and provide real value: (1) clean separation between `View`
and layout implementation details, (2) a standard extensibility point for adding new layouts,
(3) the registry is minimal overhead and idiomatic Go. Removing the interface would reduce
testability (no mock possible) and future extensibility for negligible simplification.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small |
| Risk of fix | Low |
| Notes       | The interface provides clean separation between View and layout. The registry is a standard Go pattern. Removing it would reduce testability and extensibility. |
| Verdict     | **Won't fix** — the interface and registry are well-designed as-is |

### 6.2 `Action` Interface Mixes Registration and Execution **(DONE)**

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

**Resolution:** The `Action` interface now only requires `Execute(context.Context, *Peco, Event)`. `Register` and `RegisterKeySequence` are methods on the concrete `ActionFunc` type, not part of the interface contract.

### 6.3 `MessageHub` Interface — 11 Methods **(DONE)**

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

**Resolution:** Split into `HubSender` (6 methods) and `HubReceiver` (4 methods) interfaces. `MessageHub` composes both. `SendStatusMsgAndClear` merged into `SendStatusMsg` which now takes a duration parameter.

### 6.4 `StatusBar` Interface — Trivial **(Won't fix)**

**Location:** `interface.go:232-234`

Single-method interface with only two implementations: `screenStatusBar` and
`nullStatusBar`. The null variant is a no-op. This could be a simple nil check.

**Evaluation:** The null object pattern is the **correct design** here. `screenStatusBar`
has real internal state (`clearTimer`, `timerMutex`) and complex behavior (message
truncation, timer lifecycle management, screen flushing). Replacing the interface with
nil checks would scatter conditional logic throughout callers and lose encapsulation of
the timer management. This is a textbook case where the null object pattern (a standard
Go idiom) is better than nil checks.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small |
| Risk of fix | Low |
| Notes       | The null object pattern correctly encapsulates the "suppress status messages" behavior. `screenStatusBar` has real state that would require nil-check duplication. |
| Verdict     | **Won't fix** — null object pattern is the correct design here |

### 6.5 `Buffer` Interface Has Unexported Method **(DONE)**

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

**Evaluation:** The unexported method is **intentional** package-level sealing — a standard
Go pattern. All four implementations (`Source`, `MemoryBuffer`, `FilteredBuffer`,
`ContextBuffer`) are internal to the peco package. `linesInRange` is an internal
optimization for efficient pagination in `NewFilteredBuffer`, used in exactly one place
(`buffer.go:38`). Peco is a CLI tool, not a library — no external packages implement
`Buffer`. The fix is documentation, not code restructuring.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Trivial |
| Risk of fix | None |
| Notes       | Intentional package-level sealing pattern. All implementations are internal. |
| Verdict     | **Fixed** — added documentation clarifying the intentional sealing |

**Resolution:** Added documentation to the `Buffer` interface comment explaining that the
unexported `linesInRange` method intentionally seals the interface to the peco package,
and that it is an internal optimization for efficient pagination.

### 6.6 `MatchIndexer` Interface — Barely Used **(DONE)**

**Location:** `interface.go:145-150`

Used via type assertion in a single place (`layout.go:592`). The `line.Matched` type
has an `Indices()` method but the interface adds little value for a single-use type
assertion.

**Evaluation:** The assessment is correct. `MatchIndexer` has exactly one implementing type
(`line.Matched`), is used in exactly one place via type assertion, and provides no
abstraction benefit. Replacing `target.(MatchIndexer)` with `target.(*line.Matched)` is
more explicit and eliminates unnecessary indirection.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Trivial |
| Risk of fix | None |
| Notes       | Single implementation, single usage point. Direct type assertion is clearer. |
| Verdict     | **Fixed** — removed interface, using direct type assertion |

**Resolution:** Removed the `MatchIndexer` interface from `interface.go`. Replaced the
type assertion in `layout.go:592` from `target.(MatchIndexer)` to `target.(*line.Matched)`,
using the concrete type directly. The `line` import was aliased to `linepkg` to avoid
conflict with the local `line` variable in the draw loop.

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

### 8.3 Duplicated Test Setup **(DONE)**

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

**Resolution:** Extracted `setupPecoTest(t)` helper in `peco_test.go` returning `(*Peco, context.Context)`. Uses `t.Cleanup(cancel)` for automatic teardown. Tests now use `state, ctx := setupPecoTest(t)`.

### 8.4 Tests Checking Implementation Details **(Won't fix)**

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

**Evaluation:** The assessment overstates the concern. The action names checked by this test
(e.g. `peco.ForwardChar`, `peco.Cancel`) are **user-facing identifiers** used in config files
and key binding customization, not purely internal implementation details. The `nameToActions`
map is the backing store for `resolveActionName()` which is the public config API. While
testing through `resolveActionName()` would be marginally more abstract, the test is in the
same package and serves as a valid smoke test for action registration. The test is also
incomplete (only checks 26 of 40+ actions) but still catches registration regressions.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small |
| Risk of fix | Low |
| Notes       | Action names are user-facing (config file API), not purely internal. Test is valid as a registration smoke test. |
| Verdict     | **Won't fix** — test is valid; action names are the user-facing public API |

### 8.5 Incomplete Test Cases **(DONE)**

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

**Evaluation:** Both tests had valid incomplete coverage.

- **TestRotateFilter**: The TODO asked for verification that `ExecQuery` is called after
  filter rotation. Extended the test to type a query ("func"), wait for filtering to produce
  results (buffer smaller than source), then rotate the filter and verify the buffer is
  re-filtered — proving `execQueryAndDraw` → `ExecQuery` was called with the new filter.

- **TestGHIssue331**: The comment noted that verifying `DisableCache` in draw options was
  hard. Restructured the test to use `setupPecoTest(t)` for proper lifecycle management.
  Added verification that `ToggleSingleKeyJumpMode()` correctly toggles the mode (on/off).
  Full draw cache verification would require hub interception infrastructure (injecting a
  `recordingHub` before `Run()` creates the real hub), which is out of scope for this item.

| Attribute   | Value |
|-------------|-------|
| Importance  | **Low** |
| Fix cost    | Small (TestRotateFilter), Small (TestGHIssue331) |
| Risk of fix | Low |
| Verdict     | **Fixed** |

**Resolution:** `TestRotateFilter` now types a query, verifies filtering occurs, rotates the
filter, and verifies re-filtering — completing the ExecQuery TODO. `TestGHIssue331` was
restructured to use `setupPecoTest(t)` and now verifies SingleKeyJumpMode toggle behavior
in addition to field population.

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
| ~~3.1~~ | External filter swallows panics | **DONE** — panics converted to error returns |
| ~~3.2~~ | Screen PollEvent swallows panics | **DONE** — panics logged with stack trace |
| ~~6.2~~ | Action interface mixes registration and execution | **DONE** — interface now only requires `Execute` |
| ~~6.3~~ | MessageHub interface (11 methods) | **DONE** — split into `HubSender` + `HubReceiver`, merged StatusMsg methods |
| ~~8.3~~ | Duplicated test setup | **DONE** — extracted `setupPecoTest(t)` helper |
| ~~1.5~~ | Hub context values for control flow | **PARTIALLY FIXED** — removed redundant ctx from `send()` and dead `operationNameKey`; context value for batch scope propagation is idiomatic and retained |
| ~~2.5~~ | Query exec timer callback can fire after shutdown | **DONE** — callback now checks `queryExecTimer == nil` under mutex before proceeding |
| ~~2.1~~ | Hub `Payload.waitDone()` clarity | **DONE** — reordered to save channel before receive; no actual race but code was needlessly subtle |
| ~~2.3~~ | Input handler Alt key timer race | **DONE** — timer callback now sends to `pendingEsc` channel; input loop executes the action, serializing all `ExecuteAction` calls |
| ~~2.4~~ | Filter work goroutine not waited for | **DONE** — added `sync.WaitGroup` to `Filter.Loop()` for clean shutdown; new queries still start immediately |
| ~~5.2~~ | Deprecated config fields still in struct | **DONE** — removed `Matcher`, `InitialMatcher`, `CustomMatcher` fields and `--initial-matcher` CLI option |
| ~~5.3~~ | `skipReadConfig` test-only field | **DONE** — replaced with `readConfigFn` function field for proper dependency injection |
| ~~5.5~~ | `extraOffset` global variable | **DONE** — converted to platform-specific constants; original assessment missed `layout_windows.go` (sets to 1) |
| ~~6.1~~ | Layout interface only one implementation | **Won't fix** — interface provides clean separation and extensibility; registry is idiomatic Go |
| ~~6.4~~ | StatusBar interface trivial | **Won't fix** — null object pattern is correct design; `screenStatusBar` has real state (timers, mutex) |
| ~~6.5~~ | Buffer unexported method | **DONE** — documented intentional package-level sealing pattern |
| ~~6.6~~ | MatchIndexer minimal use | **DONE** — removed interface; replaced with direct `*line.Matched` type assertion |
| ~~8.4~~ | Tests checking implementation details | **Won't fix** — action names are user-facing config API, not purely internal; test is valid |
| ~~8.5~~ | Incomplete test TODOs | **DONE** — TestRotateFilter now verifies ExecQuery after rotation; TestGHIssue331 restructured with toggle verification |
| ~~1.1~~ | God Object — Peco struct | **DONE** — extracted 4 sub-structs (`SingleKeyJumpState`, `ZoomState`, `FrozenState`, `QueryExecState`) into `state.go`; `p.mutex` scope reduced from 5+ groups to 3; further decomposition evaluated and declined (remaining fields cohesive) |
| ~~1.3~~ | Layout coupled to full `*Peco` | **DONE** — fixed 3 direct field accesses (`state.screen`, `state.selectionPrefix`, `state.config.SuppressStatusMsg`) to use getter methods; `LayoutState` interface declined (11-method interface is Go anti-pattern, same-package code) |
| ~~1.2~~ | Global mutable state — registries | **DONE** — removed post-init write to `nameToActions` in `resolveActionName()`; global maps now immutable after `init()`; moving registries to Peco declined (idiomatic Go pattern) |

---

## 10. Priority Summary

### Fix Now (High Impact, Low Cost)

| # | Issue | Importance | Cost |
|---|-------|-----------|------|
| 1.4 | Hub.Batch swallows panics | High | Small |
| ~~3.1~~ | ~~External filter swallows panics~~ | ~~High~~ | ~~Small~~ |
| ~~3.2~~ | ~~Screen PollEvent swallows panics~~ | ~~High~~ | ~~Small~~ |
| 2.2 | `context.Background()` should propagate parent | Medium | Small |
| ~~2.5~~ | ~~Timer callback context check~~ | ~~Low~~ | ~~Trivial~~ |

### Quick Wins (Low Cost, Low Risk)

| # | Issue | Importance | Cost |
|---|-------|-----------|------|
| *(none remaining)* | | | |

### Medium Effort, High Value

| # | Issue | Importance | Cost |
|---|-------|-----------|------|
| 7.1 | Remove pdebug dependency | Medium | Medium |
| ~~8.3~~ | ~~Extract test setup helper~~ | ~~Medium~~ | ~~Small~~ |
| ~~6.2~~ | ~~Split Action interface (registration vs execution)~~ | ~~Medium~~ | ~~Medium~~ |
| ~~6.3~~ | ~~Split/simplify MessageHub interface~~ | ~~Medium~~ | ~~Medium~~ |

### Major Refactor (Plan Carefully)

| # | Issue | Importance | Cost | Status |
|---|-------|-----------|------|--------|
| ~~1.1~~ | ~~Decompose Peco God Object~~ | ~~High~~ | ~~Large~~ | **DONE** |
| ~~1.3~~ | ~~Decouple Layout from *Peco~~ | ~~Medium~~ | ~~Medium~~ | **DONE** |
| ~~1.2~~ | ~~Move registries into Peco instance~~ | ~~Medium~~ | ~~Medium~~ | **DONE** |
| 1.6 | Simplify Run() orchestration | Medium | Medium | |

### Low Priority / Consider

| # | Issue | Importance | Cost |
|---|-------|-----------|------|
| ~~1.5~~ | ~~Hub context values for control flow~~ | ~~Low~~ | ~~Small~~ |
| ~~2.1~~ | ~~Hub Payload.waitDone() clarity~~ | ~~Low~~ | ~~Trivial~~ |
| ~~2.3~~ | ~~Alt key timer race~~ | ~~Low~~ | ~~Small~~ |
| ~~2.4~~ | ~~Filter work goroutine not waited for~~ | ~~Low~~ | ~~Trivial~~ |
| ~~5.2~~ | ~~Remove deprecated config fields~~ | ~~Low~~ | ~~Small~~ |
| ~~5.3~~ | ~~Replace `skipReadConfig` with function field~~ | ~~Low~~ | ~~Small~~ |
| ~~5.5~~ | ~~Remove `extraOffset` global~~ | ~~Low~~ | ~~Trivial~~ |
| ~~6.1~~ | ~~Remove Layout interface~~ | ~~Low~~ | ~~Won't fix~~ |
| ~~6.4~~ | ~~Simplify StatusBar~~ | ~~Low~~ | ~~Won't fix~~ |
| ~~6.5~~ | ~~Buffer unexported method~~ | ~~Low~~ | ~~Trivial~~ |
| ~~6.6~~ | ~~MatchIndexer minimal use~~ | ~~Low~~ | ~~Trivial~~ |
| ~~8.4~~ | ~~Tests checking implementation details~~ | ~~Low~~ | ~~Won't fix~~ |
| ~~8.5~~ | ~~Incomplete test TODOs~~ | ~~Low~~ | ~~Small~~ |
