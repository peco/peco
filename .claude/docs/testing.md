<!-- Agent-consumed file. Keep terse, unambiguous, machine-parseable. -->

# Testing

## Commands

```bash
make test              # go test -v -race ./...
go test -v -run TestFoo ./...          # single test, all packages
go test -v -run TestFoo ./filter/      # single test, specific package
go test -race -coverprofile=coverage.out ./...  # coverage
```

## Test Package Convention

- Tests use same package name (not `_test` suffix) — white-box testing
- Exception: some packages use `_test` suffix for external testing

## Key Test Helpers

- `newPeco() → *Peco` — creates test instance with SimScreen, default config
- `NewDummyScreen() → *SimScreen` — mock terminal; supports `SendEvent(Event)` for input injection
- SimScreen has fixed size, no-op rendering, collects events

## Test Patterns

- Table-driven tests with `t.Run()` subtests
- Regression tests for GitHub issues in `issues_test.go`
- Filter tests in `filter/filter_test.go`, `filter/base_test.go`
- Hub tests in `hub/hub_test.go`
- Key sequence tests in `internal/keyseq/trie_test.go`, `ahocorasick_test.go`, `ternary_test.go`
- Pipeline tests in `pipeline/pipeline_test.go`
- Selection tests in `selection/selection_test.go`
- Query tests in `query/query_test.go`

## Benchmark Tests

- `filter/bench_test.go` — filter algorithm benchmarks
- `hub/bench_test.go` — hub message passing benchmarks
- `line/bench_test.go` — line allocation benchmarks
- `internal/ansi/bench_test.go` — ANSI parsing benchmarks
- `internal/util/bench_test.go` — utility benchmarks
- `cmd/filterbench/` — standalone filter benchmark CLI

## No Test Data Directory

- No `testdata/` or golden files
- Tests use inline data and programmatic setup

## Build Tags

- Platform-specific files: `_posix.go`, `_bsd.go`, `_windows.go`, `_darwin.go`
- No custom build tags for testing

## Code Generation

- `go generate ./...` — runs `stringer` for enum types
- Generated files: `vertical_anchor_gen.go`, `hub/paging_request_type_gen.go`
