# Relic Design System

> A design system for **Relic** — an open-source Go CLI that converts Claude Code session JSONL files into shareable, syntax-highlighted, single-file HTML.

Each Relic file is one session: self-contained, offline-capable, gist-publishable. The whole document *is* the artifact — there is no app shell, no browse, no list.

The brand reflects that: quiet, document-like, terminal-heavy, deep-blue, with a single warm citron accent for "find / match / value uncovered."

---

## Source

Built from the actual repo: [`github.com/jamestelfer/relic`](https://github.com/jamestelfer/relic).

The data shape is real:

- `Turn` — a user message followed by its assistant messages
- `parser.ContentBlock` cases: `TextBlock`, `ToolUseBlock`, `ToolResultBlock`, `ThinkingBlock`, `ErrorBlock`, `RawBlock`
- Tool output passes through ANSI conversion; code passes through Chroma syntax highlighting
- Session has: name, file path, start time, turn count

The freedom: the existing repo styling is placeholder. The HTML structure and CSS in this system are a **clean redesign** intended to replace `internal/renderer/renderer.templ`'s output, not a recreation of it.

---

## Operating principles

1. **Readability first.** Long monospace passages need to feel *good* to read. Generous line height, JetBrains Mono ligatures, calm contrast.
2. **Scannability within one document.** A sticky outline rail, numbered turn markers, a fixed metadata catalog.
3. **The file is the artifact.** Works offline, prints well, looks as good in a tab as in a PDF.
4. **Quiet color.** Blue + ink, with citron used only for findings.

---

## Files

| Path | What |
|---|---|
| `colors_and_type.css` | All tokens — color, type, scale, dark mode |
| `preview/logo.html` | Stratigraphy mark + serif wordmark |
| `preview/colors.html` | Lapis / Cobalt / Citron / Ink ramps + role tints |
| `preview/type-display.html` | Display + UI typography |
| `preview/type-mono.html` | JetBrains Mono, terminal, ligatures |
| `preview/render.html` | Full 7-turn session render — outline rail + reading column, all block types |
| `preview/components.html` | Component reference — every block type in one spec sheet |

---

## Type

- **Display** — Instrument Serif. Quiet, scholarly. Used for the session title and user prompts (the human voice).
- **UI** — Geist. Engineer-friendly neutral.
- **Mono** — JetBrains Mono. Terminal output, code, cataloged metadata, role tags.

---

## Color

- **Lapis** (`#1B3A8C` anchor) — primary brand. Used for links, user-block rule, the brand mark.
- **Cobalt** (`#2E5BFF` anchor) — bright counterpart for selection/highlight.
- **Citron** (`#D9C44A` anchor) — *the find*. Reserved for matches, hits, "value uncovered." Never a background.
- **Ink** — twelve-step cool, bluish-gray neutral ramp. All surfaces and text live here.
- **Role tints** — teal for tool, violet for thinking, lapis for user, ink for assistant. Stays in the blue family except for `--error-500`.

---

## Render structure

The redesign drops the original fixture's plain `<dl>` banner + bullet TOC + nested `<details>` shape and replaces it with:

```
.layout
├── aside.rail              ← sticky outline (numbered turns, role legend, kbd hints)
└── main
    ├── header.session-head ← cataloged eyebrow, large serif title, summary, metadata grid
    ├── section.turn        × N
    │   ├── .turn-meta      ← turn number + timestamp + relative
    │   ├── article.block.user        ← serif, large, the human prompt
    │   ├── article.block.thinking    ← collapsible, violet, italic preview
    │   ├── article.block.assistant   ← reading body, ~70ch
    │   ├── article.block.tool        ← tool_card with per-tool icon tint
    │   ├── article.block.tool-result ← terminal chrome with traffic-light dots
    │   └── .turn-foot                ← prev/next jump links
    └── footer.session-foot
```

Every block sits on a 2px hairline rule colored by its role, so the eye can track who is speaking down a long scroll.

---

## What's still open

- **Print CSS.** The render needs an explicit `@media print` block: hide the rail, collapse tool cards to one line, break pages at turns.
- **Keyboard navigation.** The `J/K//` hints in the rail are decorative; wiring them requires a small JS handler.
- **Snapshots / tests for the redesign.** If this design becomes the actual renderer output, all `go-snaps` snapshots need regenerating (`UPDATE_SNAPS=true go test ./...`).
- **Compaction summary body rendering.** The body is plain Markdown in the data; the design uses static HTML for now — goldmark rendering should be wired in Go.
