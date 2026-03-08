<!-- Agent-consumed file. Keep terse, unambiguous, machine-parseable. -->

# CLI

## Entry Point

`cmd/peco/peco.go` — parses flags, creates `peco.New()`, calls `Run(ctx)`

## Flags (CLIOptions)

| Flag | Type | Description |
|------|------|-------------|
| `--help` | bool | Show help |
| `--version` | bool | Show version |
| `--query` | string | Initial query string |
| `--rcfile` | string | Config file path |
| `--buffer-size` | int | Max lines to read (0=unlimited) |
| `--null` | bool | Use NUL as line separator |
| `--initial-index` | int | Initial cursor position |
| `--initial-filter` | string | Initial filter name |
| `--prompt` | string | Prompt string |
| `--layout` | string | Layout type: top-down, bottom-up |
| `--select-1` | bool | Auto-select if single match |
| `--exit-zero` | bool | Exit 0 even on cancel |
| `--select-all` | bool | Select all lines initially |
| `--on-cancel` | string | Cancel behavior: success/error |
| `--selection-prefix` | string | Prefix for selected lines |
| `--exec` | string | Command to execute with selection |
| `--print-query` | bool | Print query as first output line |
| `--color` | ColorMode | Color mode: auto, none |
| `--height` | string | Terminal height spec |

## Exit Codes

- 0 — success (lines selected)
- 0 — cancel with `--on-cancel success` or `--exit-zero`
- 1 — cancel (default)
- Custom — from `--exec` command exit status

## Input

- Reads from stdin by default
- Positional arg → read from file
- Supports streaming (infinite) input

## Output

- Selected lines to stdout, one per line
- With `--print-query`: query string as first line
- With `--exec`: pipes selection to command
