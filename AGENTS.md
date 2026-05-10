# Agent Instructions — relic

> These instructions target AI coding agents (Claude Sonnet 4.6 and above).
> They are not intended for human readers.

## Project overview

`relic` is a CLI tool that converts Claude Code session JSONL files into
shareable, syntax-highlighted HTML. It can write HTML to a file, stdout, or
publish directly to a GitHub Gist via the `gh` CLI.

The core principle is **self-contained, distributable HTML**: each rendered file
is a single HTML document with all CSS, JS, and images inlined. No external
dependencies, no network requests, works offline. This constraint applies to
all rendering decisions — never reference external resources.

**Exception — Google Fonts:** typography is loaded via a Google Fonts `@import`
in the CSS. This is an intentional trade-off: the font payload is too large to
inline, and Google Fonts is a reliable, high-availability CDN. The system font
stack provides acceptable fallback when offline. Do not attempt to inline or
bundle these fonts.

Module: `github.com/jamestelfer/relic`  
Language: Go 1.26 (`GOEXPERIMENT=jsonv2` — encoding/json/v2 is enabled everywhere)  
Task runner: `just` (see `justfile` for all targets)

## Key commands

**Always use `just` for building and testing.** Do not use `go build`, `go test`,
or `go run` directly — the `justfile` sets `GOEXPERIMENT=jsonv2` and runs
prerequisite steps (like templ generation) that are required for correct builds.
Running `go` commands directly will fail with missing import errors.

To run the `relic` CLI, build first with `just build` then use `dist/relic`.
Do not use `go run`.

| Intent | Command |
|---|---|
| Run all checks (canonical verify) | `just verify` |
| Build | `just build` |
| Test | `just test` |
| Test a single package | `just test ./internal/parser/...` |
| Run a named test | `just test -run TestName -v ./...` |
| Lint | `just lint` |
| Regenerate templ templates | `just generate` |
| Update golden snapshots | `just update-snaps` |
| Format | `just fmt` |
| Tidy modules | `just tidy` |

> **`just generate` must be run before build, lint, or test** — it invokes
> `go tool templ generate` to produce `renderer_templ.go` from the templ
> templates. The `justfile` targets handle this automatically; run it manually
> if you edit a `.templ` file.

## Architecture

The data pipeline flows in three stages:

1. **parser** (`internal/parser/`) — decodes Claude Code JSONL into a flat
   sequence of typed Go structs (`TextBlock`, `ToolUseBlock`,
   `ToolResultBlock`, etc). This layer is purely structural: it maps JSON
   shapes to Go types without interpreting semantics.

2. **session** (`internal/session/`) — transforms parser output into
   render-ready blocks. This is where domain logic lives: a detection cascade
   in `classifyUserMessage()` routes synthetic user messages (hook injections,
   bash I/O, local commands, request interruptions) into specialised block
   types. All block types implement the sealed `Block` interface. Messages are
   grouped into turns (human-initiated conversation units).

3. **renderer** (`internal/renderer/`) — maps each `Block` to a templ
   component and produces the final self-contained HTML document. CSS, JS,
   fonts, and images are all inlined. `blockComponent()` is the central
   dispatch from block type to templ template.

Supporting packages: `internal/highlight/` (Chroma syntax highlighting),
`internal/gist/` (GitHub Gist publishing), `internal/picker/` (interactive
file selection), `internal/ansi/` (ANSI escape handling).

## Testing conventions

- Tests use `github.com/stretchr/testify` for assertions.
- Snapshot tests use `github.com/gkampitakis/go-snaps`. To update snapshots run
  `just update-snaps` (sets `UPDATE_SNAPS=true`).
- Integration-style tests in `cmd/relic/main_test.go` inject fakes for the
  `GistPublisher` interface rather than shelling out to `gh`.

## Tools (managed by mise)

- Go 1.26.0
- golangci-lint 2.11.4
- just 1.49.0
- templ is a **Go tool dependency** (`go.mod` `tool` directive); no separate
  install needed — use `go tool templ`.

## Library documentation (context7)

When using APIs from these libraries, look up current documentation via the
context7 MCP using the library IDs below:

| Library | context7 ID |
|---|---|
| templ (HTML templating) | `/a-h/templ` |
| Chroma (syntax highlighting) | `/alecthomas/chroma` |
| goldmark (Markdown parsing) | `/yuin/goldmark` |
| urfave/cli v3 (CLI framework) | `/urfave/cli` |

