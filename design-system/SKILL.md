---
description: Use this skill to design interfaces and assets for Relic, an open-source Go CLI that renders Claude Code session JSONL files as single-file HTML. Each Relic file is one self-contained session — no browse, no app shell. Engineer-focused, scannable, terminal-heavy. Deep-blue (lapis + cobalt) palette with citron accent for matches. Type: Instrument Serif (display) + Geist (UI) + JetBrains Mono (code/terminal).
---

# Relic

## What Relic is

Relic is a Go CLI (`github.com/jamestelfer/relic`) that converts Claude Code session JSONL files into shareable, syntax-highlighted HTML. It outputs one self-contained HTML file per session, optionally publishing to a GitHub Gist via the `gh` CLI. There is no multi-session viewer, no browse — each file is the artifact.

## Data model

A session is a list of `Turn`s. A turn is one user message followed by its assistant messages. Messages are made of typed `ContentBlock`s:

- `TextBlock` — Markdown body
- `ToolUseBlock` — tool name + JSON input (Bash, Read, Edit, Write, Grep, WebSearch, TodoWrite, …)
- `ToolResultBlock` — tool output (may contain ANSI escapes)
- `ThinkingBlock` — internal reasoning (collapsible)
- `ErrorBlock` — JSONL parse failure callout
- `RawBlock` — unknown block type, preserved as raw JSON

Code goes through Chroma; tool_result goes through an ANSI-to-HTML converter.

## Design principles

1. **Readability first.** Long monospace passages must feel good to read.
2. **Scannability within one document.** Sticky outline rail, numbered turn markers.
3. **The file is the artifact.** Works offline, prints well.
4. **Quiet color.** Blue + ink everywhere; citron only for findings.

## Tokens

Always import `colors_and_type.css`. Key tokens:

- `--font-display` Instrument Serif · `--font-ui` Geist · `--font-mono` JetBrains Mono
- `--lapis-600` (#1B3A8C) brand · `--cobalt-500` (#2E5BFF) highlight
- `--citron-500` (#D9C44A) the find — sparingly
- `--ink-50…950` cool bluish-gray neutrals
- `--role-user-*` (teal) · `--role-tool-*` (lapis) · `--role-assistant-*` (cobalt) · `--role-thinking-*` (violet)

## Component vocabulary

All components live in `preview/render.html` (full session) and `preview/components.html` (spec sheet).

### Conversation tier (full visual weight)
- `.block.conversation.user` — white card, teal left rule, Geist UI body ~16px
- `.block.conversation.assistant` — white card, cobalt left rule, reading body 70ch

### Process tier (internal reasoning)
- `.block.thinking` — `<details class="thinking">` with violet bg, italic preview, collapses by default
- `.block.redacted-thinking` — `.redacted-note` dashed box, non-expandable

### Action tier — tool_use
- `.block.tool` → `.tool-card[data-tool="NAME"]` — head with icon/name/arg, optional body
  - **Bash** `$` icon, ink-700; body: Chroma-highlighted bash or empty
  - **Read** `R` icon, lapis-600; body: empty (content arrives in tool_result)
  - **Write** `W` icon, lapis-600; body: `<pre class="code">` Chroma-highlighted file content
  - **Edit / MultiEdit** `E` icon, lapis-700; body: `.diff-body` with `.diff-line.add/.del/.ctx`
  - **Grep / Glob** `G` icon, citron-600 (dark text); body: empty
  - **LS** `LS` icon, ink-500; body: empty
  - **WebSearch** `↗` icon, ink-600; body: `.websearch-body` with `.ws-query` rows (label + text)
  - **WebFetch** `↗` icon, ink-600; body: `.websearch-body` with url + prompt rows
  - **Task** `▶▶` icon, ink-800; body: `.task-delegation` (`.task-badge` sub-agent label + `.task-prompt`)
  - **TodoWrite** `✓` icon, green; body: `.todo-list` with `.todo-item.completed/.in-progress` + `.todo-priority.high/.medium/.low`
  - **exit_plan_mode** `▶` icon, lapis-700; body: `.plan-body` Markdown plan
  - **MCP** `MCP` icon, lapis-700; `.mcp-server` breadcrumb + `.mcp-json` JSON body with `.jk .js .jn .jb`
  - **Unknown** `⚙` icon; body: raw JSON

### Action tier — tool_result
- `.block.tool-result` → `.term` — dark terminal with `.chrome` (traffic-light dots + label) + `<pre>` with ANSI classes `.ok .err .warn .dim .path .prompt`
- **is_error**: red chrome (`background:#1A0000`), first dot `#FF5F57`, label in `#FF8080`
- **Multimodal**: `.term` followed by `.multimodal-images > .mm-image-wrap > img`

### Action tier — other
- `.block.slash-cmd` → `.slash-card` — monospace command invocation (`.slash` `/` + `.cmd` + `.args`)
- `.block.cmd-output` → `.cmd-output-body` — preformatted muted text
- `.block.image-block` → `.image-card` with `.image-head` (meta + hint) + `.image-preview` (click-to-expand via `.expanded`)

### Meta tier (lowest visual presence)
- `.block.meta` — dashed left rule, `.meta-bar` with icon + label + `.detail`
- `.meta-separator` — dashed rule line with centered text label
- `<details class="compact-summary">` — `.cs-label` + `.cs-meta` summary; `.cs-body` Markdown content
- `.doc-placeholder` — PDF/doc icon badge + title + note
- `<details class="unknown-block">` — `.ub-tag` + `.ub-type` summary; `.mcp-json` body

### Error blocks (meta/action tier)
- `.block.api-error` → `.api-error-card` (icon square + `.etype` + `.msg`)
- `.block.error` → `.err-callout` (parse errors, JSONL failures)

### Banners and primitives
- `.session-result.success/.error` — outcome banner, `.sr-icon` + `.sr-label` + `.sr-stats`
- `.find` — citron pill (`.find::before` dot + uppercase label) — use sparingly for matches/hits
- `.pair-id` — mono badge linking tool_use ↔ tool_result

### Clamp (progressive disclosure)
Wrap any long `<pre>` or `.preview` in `.clamp > .clamp-body + <button class="clamp-toggle">`.  
JS toggles `.is-open` on the `.clamp` element; CSS transitions `max-height` and fades the mask.

## Token layering

- **Palette tokens** (`--ink-*`, `--lapis-*`, `--teal-*`, etc.) — raw color values, never change between modes
- **Semantic tokens** (`--bg`, `--fg`, `--role-*-rule`, etc.) — purpose-driven, use `light-dark()` when mode-dependent
- **Component-scoped variables** (`--rule-color`) — set on a component root, overridden by modifiers
- **Direct palette in components** is acceptable for: per-tool icon branding, one-off decorative colors, `color-mix()` expressions

## Don'ts

- No autumn tones (no orange, brown, amber backgrounds)
- No emoji as iconography — use single letters or initials in tinted squares
- Don't use citron as a background or a body color — it is a *find* signal
