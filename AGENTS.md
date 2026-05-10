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

All `go` commands run under `GOEXPERIMENT=jsonv2` (set by the `justfile`).

## Package structure

```
cmd/relic/          CLI entrypoint (urfave/cli/v3), options wiring, execute()
internal/parser/    Decodes Claude Code JSONL → typed Go structs
internal/renderer/  Renders parsed messages to HTML (templ + goldmark + chroma)
internal/highlight/ Syntax highlighting via Chroma
internal/picker/    Interactive JSONL file picker (charmbracelet/huh)
internal/gist/      Publishes HTML to GitHub Gist via gh CLI
internal/ansi/      ANSI escape handling utilities
```

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

