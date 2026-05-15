# Agent Instructions — relic

## Targeting

AI agent instructions must target models using a Claude Sonnet 4.6 as their
baseline. Write for agents, not humans.

## Project overview

`relic` is a CLI tool that converts Claude Code session JSONL files into
shareable, syntax-highlighted HTML. It can write HTML to a file, stdout, or
publish directly to a GitHub Gist via the `gh` CLI.

The core principle is **self-contained, distributable HTML**: each rendered file
is a single HTML document with all CSS, JS, and images inlined. No external
dependencies, no network requests, works offline. This constraint applies to
(almost) all rendering decisions — never reference external resources.

**Exception — Google Fonts:** typography is loaded via a Google Fonts `@import`
in the CSS. This is an intentional trade-off: the font payload is too large to
inline, and Google Fonts is a reliable, high-availability CDN. The system font
stack provides acceptable fallback when offline. Do not attempt to inline or
bundle these fonts.

## Toolchain

**Language:** Go 1.26 (`GOEXPERIMENT=jsonv2` — `encoding/json/v2` is enabled everywhere)  
**Task runner:** `just` (see `justfile` for all targets)
**Tooling manager:** mise (`mise trust && mise install`)

> [!IMPORTANT]
> Go 1.26 includes new libraries and syntax that may not be in your training
> data. (e.g. `new("constant")` expressions, range functions, `iter.Seq`).
> Follow the linter that indicates new patterns and see to use them
> consistently.

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

## Go style

- write go in the style of the stdlib
- add comments where the code isn't self explanatory
- this is a CLI application not a library, only consider backwards compatibility for CLI commands and flags

## Testing conventions

- Tests use `github.com/stretchr/testify` for assertions.
- Snapshot tests use `github.com/gkampitakis/go-snaps`. To update snapshots run
  `just update-snaps` (sets `UPDATE_SNAPS=true`).
- Integration-style tests in `cmd/relic/main_test.go` inject fakes for the
  `GistPublisher` interface rather than shelling out to `gh`.
- Use data-driven tests whereever possible. Assertions in these tests must not
  be conditional - they should be comprehensively testing a single simple set of
  assertions.

## Library documentation (context7)

When using APIs from these libraries, look up current documentation via the
context7 MCP using the library IDs below:

| Library | context7 ID |
|---|---|
| templ (HTML templating) | `/a-h/templ` |
| Chroma (syntax highlighting) | `/alecthomas/chroma` |
| goldmark (Markdown parsing) | `/yuin/goldmark` |
| urfave/cli v3 (CLI framework) | `/urfave/cli` |

