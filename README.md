<picture>
  <source media="(prefers-color-scheme: dark)" srcset="design-system/logo-dark.png">
  <img alt="Relic" src="design-system/logo-light.png" width="240">
</picture>

Convert [Claude Code](https://docs.anthropic.com/en/docs/claude-code) session
logs into shareable, self-contained HTML documents.

Claude Code stores every session as a JSONL file under `~/.claude/projects/`.
Relic parses these files and renders them as a single HTML page with syntax
highlighting, collapsible tool calls, and a navigable turn-by-turn outline — no
external dependencies, no network requests, works offline.

## Features

- **Self-contained HTML** — all CSS, JS, and images are inlined into a single
  file. Open it in any browser, email it, drop it in a wiki, host it on a
  static server. The only external resource is a Google Fonts import for
  typography (system fonts provide the fallback).

- **Secret redaction** — API keys, tokens, and credentials are automatically
  detected and replaced with `[REDACTED]` markers before rendering. Powered by
  [gitleaks](https://github.com/gitleaks/gitleaks) rules. Disable with
  `--no-redact` if you need the raw content.

- **Interactive session picker** — run `relic` with no arguments to browse
  recent sessions from `~/.claude/projects/` in a two-step terminal menu:
  pick a project, then pick a session.

- **GitHub Gist publishing** — render and publish in one step with `--mode
  gist`. Requires the [GitHub CLI](https://cli.github.com/) (`gh`). Returns
  both the gist URL and a preview URL for immediate sharing.

- **Syntax highlighting** — fenced code blocks are highlighted via
  [Chroma](https://github.com/alecthomas/chroma) with language detection.

- **Full session fidelity** — renders user prompts, assistant responses,
  thinking blocks, tool calls (Bash, Read, Edit, Write, and more), tool
  results, errors, compaction boundaries, hook injections, and system messages.

- **Navigable outline** — a sidebar lists every turn with keyboard navigation
  (`[` / `]` to move between turns).

- **Light/dark theme** — toggle between themes with the button in the header.
  Respects `prefers-color-scheme` by default.

## Install

Requires [Go](https://go.dev/) 1.26+ and
[just](https://github.com/casey/just):

```sh
git clone https://github.com/jamestelfer/relic.git
cd relic
just build
```

The binary is written to `dist/relic`. Copy it somewhere on your `$PATH`.

## Usage

```
relic                              # pick a session interactively
relic session.jsonl                # render to session.html in CWD
relic -o out.html session.jsonl    # render to a specific file
relic -o - session.jsonl           # render to stdout
relic -m gist session.jsonl        # publish as a secret GitHub Gist
relic -m public-gist session.jsonl # publish as a public GitHub Gist
```

### Options

| Flag | Description |
|---|---|
| `-m`, `--mode` | Output destination: `html` (default), `gist`, or `public-gist` |
| `-o`, `--output` | Write HTML to a file; defaults to `<session>.html` in CWD; use `-` for stdout |
| `-n`, `--name` | Override the session name in the rendered banner and page title |
| `--no-redact` | Disable automatic secret redaction |
| `--debug` | Enable debug-level logging on stderr |

## What the output looks like

The rendered HTML is a single-page document structured around conversation
turns. Each turn starts with a user prompt and contains the assistant's
response, including any tool calls and their results.

```
┌─────────────────────────────────────────────────────┐
│  Outline · 12 turns                     ◑ theme     │
│  ┌────────────────┐ ┌─────────────────────────────┐ │
│  │ 1. Fix parser  │ │ ■ User                      │ │
│  │ 2. Add tests   │ │   Fix the parser to handle  │ │
│  │ 3. Refactor    │ │   empty lines gracefully.   │ │
│  │ ...            │ │                             │ │
│  │                │ │ ■ Assistant                  │ │
│  │ Roles          │ │   I'll look at the parser.  │ │
│  │  user  prompt  │ │                             │ │
│  │  asst  reply   │ │ ▸ Tool: Read parser.go      │ │
│  │  tool  calls   │ │ ▸ Tool: Edit parser.go      │ │
│  │  think reason  │ │                             │ │
│  │                │ │   Fixed. The parser now      │ │
│  │ [ prev ] next  │ │   skips blank lines...      │ │
│  └────────────────┘ └─────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

Key visual elements:

- **Sidebar outline** — lists each turn with a title derived from the user's
  prompt. Collapses to a disclosure on narrow viewports.
- **Role-colored blocks** — user prompts, assistant text, tool calls, and
  thinking blocks each have a distinct visual treatment.
- **Collapsible tool calls** — Bash commands, file reads, edits, and other
  tool interactions are shown with their inputs and results, collapsed by
  default for long outputs.
- **Code highlighting** — fenced code blocks and tool output are
  syntax-highlighted with a theme matched to the light/dark mode.
- **Copy buttons** — code blocks include a one-click copy button.
- **Image zoom** — inline images (e.g. screenshots read by Claude) can be
  clicked to view full-size.

## Built with

Relic leans on a number of high-quality open source libraries:

| Library | Role |
|---|---|
| [goldmark](https://github.com/yuin/goldmark) | Markdown rendering for assistant responses |
| [Chroma](https://github.com/alecthomas/chroma) | Syntax highlighting for fenced code blocks and tool output |
| [bluemonday](https://github.com/microcosm-cc/bluemonday) | HTML sanitization — allowlists safe markup in Markdown to prevent XSS |
| [gitleaks](https://github.com/gitleaks/gitleaks) | Secret detection rules powering automatic redaction |
| [terminal-to-html](https://github.com/buildkite/terminal-to-html) | ANSI escape sequence rendering for terminal output |
| [templ](https://github.com/a-h/templ) | Type-safe HTML templating |
| [huh](https://github.com/charmbracelet/huh) | Interactive terminal UI for the session picker |
| [urfave/cli](https://github.com/urfave/cli) | CLI framework |

## License

Apache 2.0 — see [LICENSE](LICENSE).
