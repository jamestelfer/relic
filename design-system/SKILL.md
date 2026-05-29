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

Always import `tokens.css`. Key tokens:

- `--font-display` Instrument Serif · `--font-ui` Geist · `--font-mono` JetBrains Mono
- `--lapis-600` (#1B3A8C) brand · `--cobalt-500` (#2E5BFF) highlight
- `--citron-500` (#D9C44A) the find — sparingly
- `--ink-50…950` cool bluish-gray neutrals
- `--role-user-*` (teal) · `--role-tool-*` (lapis) · `--role-assistant-*` (cobalt) · `--role-thinking-*` (violet)

## CSS architecture — four authored layers

Relic styles are organised into four authored layers plus two verbatim pipeline stylesheets. Six files total, no subdirectory:

| Layer | File | Purpose |
|---|---|---|
| 0 | `tokens.css` | Design tokens. Only file that declares raw scale values. |
| 1 | `primitives.css` | Structural building blocks. No block-specific selectors. |
| 2 | `scaffold.css` | `article.block` wrapper, `.role` strip, turn layout. |
| 3 | `compositions.css` | Per-role colours, icon tints, block-specific overrides. |
| — | `highlight.css` | Verbatim Chroma palette. Do not edit. |
| — | `terminal.css` | Verbatim terminal-to-html palette. Do not edit. |

`compositions.css` reaches palette tokens (`--ink-*`, `--lapis-*`, `--success-*`, `--role-*-*`, `--bg-*`, `--fg-*`, `--border*`) and primitive classes only. It does **not** redeclare generic scale tokens (`--font-*`, `--color-*`, `--radius-*`, `--shadow-*`, `--leading-*`, `--tracking-*`) — those are reached through the primitives that already use them.

The four authored layers are flat in the directory — no subdirectory. The v1 dark-surface stylesheet does not exist in v2; its rules are absorbed into the `.card` + `.card-chrome` + `.window-dots` + `.ct-terminal` primitive vocabulary under the `.card--dark` modifier.

## Primitive vocabulary (Layer 1)

The catalogue of every primitive lives in `design-system/upd/components.html`. That file is the source of truth — when in doubt, copy structure from there, not from this skill.

### Surface

- `.card` — elevated paper surface. Every non-conversation block that needs an enclosing surface uses this.
- `.card--dark` — colour-scheme-pinning modifier on `.card`. Sets `color-scheme: dark` and locks the surface, border, chrome, and window-dot tokens to their dark values regardless of page theme. Used by every terminal-shell block. The structure is identical to a light card; only the colour scheme is pinned. Bash and terminal results compose as `.card.card--dark`.
- `.card-chrome` — sunk flex header bar. Carries the eyebrow voice (mono, uppercase, tracked). Optional — many cards have no chrome.
- `.card-body` — content section. Has its own padding and a top-border separator. The first `.card-body` in a card (no preceding chrome) drops its top border automatically.
- `.window-dots` — three-dot decoration. Lives inside `.card-chrome`. Dot colour is `--dot-color`; the dark-card override supplies the dark variant.

### Clamp

- `.card-body.clamp` — collapsibility modifier on the body section. Always on the body, **never** wraps the card from outside.
- Structure: `div.card-body.clamp > div.clamp-body + button.clamp-toggle`. The toggle is always in the DOM; JS adds `.is-fit` when the content fits and the toggle disappears.
- One JS selector covers every clamp in the system: `btn.closest(".card-body.clamp")`. Light and dark cards share the same clamp structure.
- Clamp and disclosure are orthogonal — never put `.card-body.clamp` inside a `<details class="disclosure">`.

### Disclosure

- `.disclosure` — modifier on a `<details>` that supplies marker suppression, mechanics, and the right-aligned chevron rotation. The chevron is a `::after` pseudo-element on `summary`; **never** add a chevron span (`sm-chevron`, `cs-chevron`, `ub-chevron`, `thinking-chevron` are all retired).
- Disclosure card pattern: `details.card.disclosure > summary.card-chrome + .card-body`.

### Content types (applied alongside `.card-body`)

- `.ct-prose` — markdown body. Paragraphs, headings, lists, tables, inline `code`. The single definition is the only prose ruleset in the system.
- `.ct-code` — Chroma syntax-highlighted output. The `<pre class="chroma">` supplies its own padding; the body is a transparent host.
- `.ct-terminal` — terminal-to-html output. Peer of `.ct-code`. Establishes the context the verbatim `terminal.css` palette expects. **Every `.ct-terminal` body lives inside a `.card--dark` card** — that is what supplies `color-scheme: dark` to the palette.
- **Custom structured** — todo lists, ask question/answer rows, websearch queries, task delegations, session results. No shared content-type class; each is a Layer 3 composition.

### Flat surfaces

- `.meta-bar` — thin ruled strip. Not a content container. Variants: `.meta-bar--interrupt`, `.meta-bar--system`, `.meta-bar--teammate`. May be followed by a `.card-body` (or `.card-body.clamp`) for the rare meta blocks that carry content (`teammateMessage`, `systemXML`).
- `.callout` — flat non-elevated panel. Modifiers: `.callout--sunk` (dashed, muted), `.callout--error` (coloured), `.callout--cmd-output` (sunk + monospace `<pre>` host).
- `.compaction-boundary` — horizontal divider rule with a centred eyebrow label between turns.

## The five structural families

Every block type belongs to exactly one family. Exemplars and specialisations live in `components.html` in family order: card (light), card (dark), conversation, disclosure, callout, meta-bar.

| Family | Skeleton | Members |
|---|---|---|
| Card (light) | `article.block.<role>` → `div.card` → `.card-chrome?` + `.card-body[.clamp]?` | `toolCall`, all `toolBody` variants, non-terminal `toolResult`, `userBashInput`, `image`, `taskNotification`, `sessionResult`, `toolResult (ask enrichment)` |
| Card (dark) | `article.block.<role>` → `div.card.card--dark` → `.card-chrome` + `.card-body.clamp.ct-terminal` | terminal-shell `toolResult`, `userBashResult` |
| Conversation | `article.block.conversation.<role>` (is the card) → `.role` + `.card-body[.clamp]?.ct-prose`; `slashMerged` is a specialised conversation-card variant that swaps the plain body for a slash-specific disclosure treatment | `userText`, `assistantText`, `slashMerged` |
| Disclosure | `article.block.<role>` → `details.card.disclosure` → `summary.card-chrome` + `.card-body` | `thinking`, `compactionSummary`, `rawBlock` |
| Callout | `article.block.<role>` → `.callout[.<modifier>]?` | `redactedThinking`, `error`, `apiError`, `system (cmd_output)` |
| Meta-bar | `article.block.meta` → `.meta-bar[.<variant>]?` + optional body | `hookInjection`, `requestInterrupted`, `teammateMessage`, `systemXML`, `system (meta)`, `compactionBoundary` |

## Tool icons — semantic classes only

Tool icons use semantic classes on the `.icon` element. Tools that share a tint share a class:

| Class | Tools |
|---|---|
| `icon--shell` | Bash |
| `icon--user-bash` | userBashInput |
| `icon--file-op` | Read, Write |
| `icon--file-edit` | Edit, MultiEdit |
| `icon--search` | Grep, Glob |
| `icon--list` | LS |
| `icon--web` | WebSearch, WebFetch |
| `icon--task` | Task |
| `icon--todo` | TodoWrite |
| `icon--plan` | exit_plan_mode |
| `icon--mcp` | MCP tools |
| `icon--ask` | AskUserQuestion |

## Conversation has its own pattern

The user and assistant blocks are an intentional exception. `article.block.conversation.<role>` **is** the card surface — there is no inner `.card` div. The role tag sits inside the card as an in-card header. When the body is clamped, a composition rule (`.block.conversation > .card-body.clamp`) lets it escape the card's inset padding so the toggle strip is edge-to-edge; `.block.conversation` has `overflow: hidden` so the card's rounded corners clip the clamp.

## Slash commands

`slashMerged` is the **sole** slash-command representative. It is a specialised **conversation-card** form, not a disclosure-family card: it inherits conversation-card surface ownership and then adds slash-specific disclosure behaviour and styling. Its summary carries the slash/cmd/args triplet (`.sm-cmd > .slash + .cmd + .args`) plus a `.sm-out-peek` of the first output line; the body is `.sm-out-body` with the full output.

## Don'ts

- Tool icons use semantic classes (`icon--shell`, `icon--file-op`, …), never attribute selectors.
- The dark surface is always `.card.card--dark`. There is no separate dark-surface class.
- The clamp is a modifier on `.card-body` only. It never wraps a card from outside.
- No `:has()` selectors used to compensate for clamp/chrome interaction. The clamp lives on `.card-body`, so the chrome is structurally outside the clamped region.
- No clamp inside a disclosure. Disclosure and clamp are orthogonal.
- No per-composition chevron spans inside a `.disclosure` summary — the chevron is supplied by `.disclosure > summary::after`.
- No autumn tones (no orange, brown, amber backgrounds).
- No emoji as iconography — use single letters or initials in tinted squares.
- Don't use citron as a background or a body color — it is a *find* signal.
- Don't reach generic scale tokens (`--font-*`, `--color-*`, `--radius-*`, `--shadow-*`, `--leading-*`, `--tracking-*`) from `compositions.css`. Use them through primitive classes that already carry them.