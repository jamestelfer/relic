# Relic: Renderable Items Taxonomy

This document catalogs every distinct renderable item in a Claude Code session transcript, ranked by relevance to a human reader, with intermediate TypeScript schemas showing where each field originates. The goal is to fully specify the design surface for Relic's HTML renderer.

Sub-agent and sidechain rendering is out of scope until DAG parsing is implemented. Records with `isSidechain: true` are filtered from the main chain. Task tool calls render as a delegation boundary, not as a call–response pair.

---

## Cross-Cutting Design Primitives

Four structural patterns recur across multiple item types. The design language needs a shared vocabulary for these.

### Primitive 1: Visual Hierarchy

The heroes of the transcript are **what the user typed** and **what the agent said back**. Their rendering must reflect their importance. Everything else uses a progressively more muted visual language.

| Tier | Items | Visual weight |
|------|-------|---------------|
| **Conversation** | Human prompt, assistant text | Full weight — primary typeface, full contrast, prominent placement. A reader skimming the document should be able to follow the human↔agent dialogue without reading anything else. |
| **Action** | Tool use, tool result, slash command, command output, images | Muted — reduced contrast, smaller type or indented, collapsible by default. Explains *how* work got done, but secondary to the conversation. |
| **Process** | Thinking, redacted thinking | Further muted — Claude's internal reasoning, not part of the conversation. Collapsed by default, visually distinct from actions. |
| **Meta** | Compaction boundary/summary, session header, API errors, system notes, parse errors | Lowest visual presence — informational chrome about the transcript itself, not about the work. |

This hierarchy is the single most important constraint on the design language. Every renderable item below belongs to exactly one tier.

Note that `assistant` records contain blocks from multiple tiers: text blocks are **Conversation**, tool_use blocks are **Action**, thinking blocks are **Process**. The assistant record is not a single tier — its content blocks render at their respective tiers within the message.

### Primitive 2: Call–Response Pairing

An **initiating action** appears at one point in the timeline and its **response** appears later. Both halves render in transcript order (the document is temporally organized), but the design must provide:

- A **hypertext link** from the call to the response and vice versa (anchor + fragment)
- An **optional visible affordance** (e.g. matching badge, border color, or ID label) so a reader scanning can visually associate the two halves

Two keyed pairing mechanisms exist:

| Call | Response | Pairing key |
|------|----------|-------------|
| `tool_use` block (in assistant message) | `tool_result` block (in subsequent user message) | `tool_use.id` ↔ `tool_result.tool_use_id` |
| Slash command (`<command-name>` in user record) | Command output (`<local-command-stdout>` in subsequent user record) | Positional: matching command name tag |

Task (sub-agent) tool calls are **excluded** — they are delegation boundaries whose response is an entire nested transcript, not a single result. Task rendering is deferred pending DAG work.

Call–response pairs are usually adjacent in the transcript but this is not guaranteed. The design must not assume adjacency.

### Primitive 3: Collapsible Section

Many items have a compact **summary line** visible when collapsed and a full **body** visible when expanded:

- Summary template: icon + label + primary argument (one line)
- Body template: full content, syntax-highlighted or ANSI-converted as appropriate
- Named preview line constants controlling initial visible height (`TOOL_USE_PREVIEW_LINES`, `TOOL_RESULT_PREVIEW_LINES`, `THINKING_PREVIEW_LINES`)
- CSS-only expand/collapse via `<details>`/`<summary>`

### Primitive 4: Progressive Enhancement for Structured Data

Tool results exist in two forms: the **inline** `tool_result` content (freeform text, always present) and the **structured** `toolUseResult` envelope field (typed JSON, sometimes present). The design uses the same container for both — the structured form enriches the summary line and body content when available, but the generic inline rendering is always the baseline. One template, richer content extraction when the data permits.

---

## Part 1: Record-Level Filtering

| Record `type` | Relevance | Disposition |
|---|---|---|
| `user` (real prompt) | 10 | Render — **Conversation** tier |
| `user` (tool_result feedback) | 9 | Render — **Action** tier, as response half of tool pair |
| `user` (slash command) | 5 | Render — **Action** tier, as call half of command pair |
| `user` (command stdout) | 5 | Render — **Action** tier, as response half of command pair |
| `user` (`isCompactSummary: true`) | 6 | Render — **Meta** tier |
| `user` (`isMeta: true`, hook injection) | 3 | Render — **Meta** tier, faded system note |
| `user` (`isApiErrorMessage: true`) | 4 | Render — **Meta** tier, error callout |
| `assistant` | 10 | Render — **Conversation** tier (text blocks) / **Action** tier (tool_use blocks) / **Process** tier (thinking blocks) |
| `system` / `subtype: "init"` | 5 | Extract for session header — **Meta** tier |
| `system` / `subtype: "compact_boundary"` | 5 | Render — **Meta** tier |
| `system` / `subtype: "error"` | 4 | Render — **Meta** tier |
| `summary` | 3 | Use as `<title>` / TOC heading; not rendered inline |
| `result` | 2 | Rare (headless only); **Meta** tier if present |
| `file-history-snapshot` | 0 | Skip |
| `progress` | 0 | Skip |
| `custom-title` | 3 | Use as `<title>` override; not rendered inline |
| `attachment` | 2 | Render — **Meta** tier, note that attachment was present |

### Records filtered from the main chain

| Flag | Detection | Disposition |
|---|---|---|
| `isSidechain: true` | Envelope field | Skip entirely (deferred pending DAG work) |
| `isApiErrorMessage: true` | Envelope field, `message.model === "<synthetic>"` | Render as error callout, not as assistant message |
| `isCompactSummary: true` | Envelope field | Render as compaction summary, not as human prompt |
| `isMeta: true` | Envelope field | Render as system note, not as human prompt |

---

## Part 2: Content Block Renderable Items

Within `user` and `assistant` records, `message.content` is either a plain string or an array of typed blocks.

### 2.1 Text Block — Conversation tier

**Relevance: 10**

```typescript
// Source: message.content[] where type === "text"
interface RenderableText {
  kind: "text";
  text: string;           // from content.text — may contain Markdown
  // Rendering: goldmark → HTML, with Chroma for fenced code blocks.
  // Full visual weight — primary typeface, full contrast.
}
```

### 2.2 Thinking Block — Process tier

**Relevance: 7**

```typescript
// Source: message.content[] where type === "thinking"
interface RenderableThinking {
  kind: "thinking";
  thinking: string;       // from content.thinking — raw scratchpad text
  // content.signature: NOT rendered (opaque verification token)
  // Rendering: collapsible section, label "Internal reasoning",
  // THINKING_PREVIEW_LINES visible when collapsed, Markdown-rendered when expanded.
  // Visually muted — not part of the conversation.
}
```

### 2.3 Redacted Thinking Block — Process tier

**Relevance: 4**

```typescript
// Source: message.content[] where type === "redacted_thinking"
interface RenderableRedactedThinking {
  kind: "redacted_thinking";
  // content.data: opaque encrypted bytes — NOT renderable as text
  // Rendering: non-expandable label "Redacted reasoning".
  // Visually echoes thinking block but signals content is unavailable.
}
```

### 2.4 Tool Use Block — Action tier (call half of pair)

All tool_use blocks share the `tool_use` content block type but render differently based on tool name. Every tool_use carries an `id` that pairs it with a subsequent `tool_result` (except Task — see §2.4.10).

Summary lines use a **per-tool icon** plus the **primary argument**. The design system is encouraged to provide a distinct icon for each tool.

#### 2.4.1 Bash

**Relevance: 10** — Most frequent tool.

```typescript
// Source: message.content[] where type === "tool_use" && name === "Bash"
interface RenderableToolUseBash {
  kind: "tool_use";
  tool: "Bash";
  id: string;             // from content.id — pairing key
  command: string;        // from content.input.command
  description?: string;   // from content.input.description — 5-10 word summary
  timeout?: number;       // from content.input.timeout
  // Summary: icon + (description ?? first line of command)
  // Body: Chroma-highlighted bash of full command
}
```

#### 2.4.2 Read

**Relevance: 9**

```typescript
// Source: message.content[] where type === "tool_use" && name === "Read"
interface RenderableToolUseRead {
  kind: "tool_use";
  tool: "Read";
  id: string;
  filePath: string;       // from content.input.file_path
  offset?: number;        // from content.input.offset
  limit?: number;         // from content.input.limit
  // Summary: icon + filePath (with line range if offset/limit present)
  // Body: path and range parameters (file content arrives in paired tool_result)
}
```

#### 2.4.3 Write

**Relevance: 9**

```typescript
// Source: message.content[] where type === "tool_use" && name === "Write"
interface RenderableToolUseWrite {
  kind: "tool_use";
  tool: "Write";
  id: string;
  filePath: string;       // from content.input.file_path
  content: string;        // from content.input.content — full file content
  // Summary: icon + filePath
  // Body: Chroma-highlighted content (language inferred from file extension)
}
```

#### 2.4.4 Edit

**Relevance: 9**

```typescript
// Source: message.content[] where type === "tool_use" && name === "Edit"
interface RenderableToolUseEdit {
  kind: "tool_use";
  tool: "Edit";
  id: string;
  filePath: string;       // from content.input.file_path
  oldString: string;      // from content.input.old_string
  newString: string;      // from content.input.new_string
  replaceAll?: boolean;   // from content.input.replace_all
  // Summary: icon + filePath
  // Body: Chroma diff rendering (old → new)
}
```

#### 2.4.5 MultiEdit

**Relevance: 8**

```typescript
// Source: message.content[] where type === "tool_use" && name === "MultiEdit"
interface RenderableToolUseMultiEdit {
  kind: "tool_use";
  tool: "MultiEdit";
  id: string;
  filePath: string;       // from content.input.file_path
  edits: Array<{
    oldString: string;    // from content.input.edits[].old_string
    newString: string;    // from content.input.edits[].new_string
    replaceAll?: boolean; // from content.input.edits[].replace_all
  }>;
  // Summary: icon + filePath + " (N edits)"
  // Body: sequence of Chroma diff renderings per edit
}
```

#### 2.4.6 Glob

**Relevance: 6**

```typescript
// Source: message.content[] where type === "tool_use" && name === "Glob"
interface RenderableToolUseGlob {
  kind: "tool_use";
  tool: "Glob";
  id: string;
  pattern: string;        // from content.input.pattern
  path?: string;          // from content.input.path
  // Summary: icon + pattern
  // Body: pattern + path (the result listing is the interesting part)
}
```

#### 2.4.7 Grep

**Relevance: 6**

```typescript
// Source: message.content[] where type === "tool_use" && name === "Grep"
interface RenderableToolUseGrep {
  kind: "tool_use";
  tool: "Grep";
  id: string;
  pattern: string;        // from content.input.pattern
  path?: string;          // from content.input.path
  glob?: string;          // from content.input.glob
  // Summary: icon + pattern
  // Body: search parameters
}
```

#### 2.4.8 LS

**Relevance: 5**

```typescript
// Source: message.content[] where type === "tool_use" && name === "LS"
interface RenderableToolUseLS {
  kind: "tool_use";
  tool: "LS";
  id: string;
  path: string;           // from content.input.path
  // Summary: icon + path
  // Body: just the path (result has the listing)
}
```

#### 2.4.9 TodoRead / TodoWrite

**Relevance: 5**

```typescript
// Source: message.content[] where type === "tool_use" && name === "TodoRead" | "TodoWrite"
interface RenderableToolUseTodo {
  kind: "tool_use";
  tool: "TodoRead" | "TodoWrite";
  id: string;
  todos?: Array<{         // from content.input.todos (TodoWrite only)
    content: string;
    status: "pending" | "in_progress" | "completed";
    priority: "high" | "medium" | "low";
  }>;
  // Summary: icon + "Read todos" or "Update todos (N items)"
  // Body: todo list rendered as checklist for TodoWrite, empty for TodoRead
}
```

#### 2.4.10 Task (Sub-agent) — Delegation boundary, NOT a call–response pair

**Relevance: 7**

```typescript
// Source: message.content[] where type === "tool_use" && name === "Task"
interface RenderableToolUseTask {
  kind: "tool_use";
  tool: "Task";
  id: string;
  description: string;    // from content.input.description — 3-5 word label
  prompt: string;         // from content.input.prompt — full sub-agent instructions
  subagentType: string;   // from content.input.subagent_type
  // Summary: icon + description
  // Body: prompt text
  // NOTE: The tool_result for Task contains a sub-agent transcript summary,
  // rendered generically. Full sub-agent transcript rendering requires DAG
  // parsing and is a separate design problem — effectively a nested document.
}
```

#### 2.4.11 WebFetch

**Relevance: 6**

```typescript
// Source: message.content[] where type === "tool_use" && name === "WebFetch"
interface RenderableToolUseWebFetch {
  kind: "tool_use";
  tool: "WebFetch";
  id: string;
  url: string;            // from content.input.url
  prompt: string;         // from content.input.prompt
  // Summary: icon + url
  // Body: url + prompt
}
```

#### 2.4.12 WebSearch

**Relevance: 6**

```typescript
// Source: message.content[] where type === "tool_use" && name === "WebSearch"
interface RenderableToolUseWebSearch {
  kind: "tool_use";
  tool: "WebSearch";
  id: string;
  query: string;          // from content.input.query
  // Summary: icon + query
  // Body: query text
}
```

#### 2.4.13 NotebookEdit / NotebookRead

**Relevance: 4**

```typescript
// Source: message.content[] where type === "tool_use" && name === "NotebookEdit" | "NotebookRead"
interface RenderableToolUseNotebook {
  kind: "tool_use";
  tool: "NotebookEdit" | "NotebookRead";
  id: string;
  notebookPath: string;   // from content.input.notebook_path
  // Summary: icon + notebookPath
  // Body: cell source if NotebookEdit
}
```

#### 2.4.14 BashOutput / KillShell

**Relevance: 3**

```typescript
// Source: message.content[] where type === "tool_use" && name === "BashOutput" | "KillShell"
interface RenderableToolUseShellMgmt {
  kind: "tool_use";
  tool: "BashOutput" | "KillShell";
  id: string;
  shellId: string;        // from content.input.bash_id or content.input.shell_id
  // Summary: icon + "Check background shell" or "Kill shell"
  // Body: shell ID
}
```

#### 2.4.15 exit_plan_mode

**Relevance: 5**

```typescript
// Source: message.content[] where type === "tool_use" && name === "exit_plan_mode"
interface RenderableToolUsePlanExit {
  kind: "tool_use";
  tool: "exit_plan_mode";
  id: string;
  plan: string;           // from content.input.plan — full plan Markdown
  // Summary: icon + "Submit plan"
  // Body: plan rendered as Markdown
}
```

#### 2.4.16 MCP Tool (`mcp__<server>__<tool>`)

**Relevance: 5**

```typescript
// Source: message.content[] where type === "tool_use" && name starts with "mcp__"
interface RenderableToolUseMCP {
  kind: "tool_use";
  tool: string;           // full name e.g. "mcp__github__create_issue"
  server: string;         // extracted: segment between first and second "__"
  toolName: string;       // extracted: segment after second "__"
  id: string;
  input: Record<string, unknown>; // opaque — schema varies per MCP server
  // Summary: icon + server + "/" + toolName
  // Body: JSON-formatted input
}
```

#### 2.4.17 Unknown Tool

**Relevance: 4** — Forward-compatibility for tools added after Relic's release (LSP, Monitor, PowerShell, future tools).

```typescript
// Source: message.content[] where type === "tool_use" && name not matched above
interface RenderableToolUseUnknown {
  kind: "tool_use";
  tool: string;           // the unrecognised tool name
  id: string;
  input: Record<string, unknown>;
  // Summary: icon "⚙" + tool name + first string value in input
  // Body: JSON-formatted input
}
```

### 2.5 Tool Result Block — Action tier (response half of pair)

Tool results arrive as `type: "tool_result"` content blocks inside `user` records. The renderer correlates them with the preceding `tool_use` block via `tool_use_id` (call–response pairing).

#### 2.5.1 Tool Result — Generic (inline string content)

**Relevance: 9**

```typescript
// Source: message.content[] where type === "tool_result"
// where content is a plain string
interface RenderableToolResult {
  kind: "tool_result";
  toolUseId: string;      // from content.tool_use_id — pairing key back to tool_use.id
  content: string;        // from content.content — freeform text, may contain ANSI escapes
  isError: boolean;       // from content.is_error ?? false
  // Rendering: collapsible section, Action tier.
  // Summary: first line of content (ANSI-stripped).
  // Body: ANSI-converted content via terminal-to-html.
  // If isError: error styling (distinct border/icon).
  // Hypertext link back to the originating tool_use block.
}
```

#### 2.5.2 Tool Result — Multimodal (array content)

**Relevance: 5** — Prevalence uncertain but structurally distinct from the string form.

```typescript
// Source: message.content[] where type === "tool_result"
// where content is an array of typed blocks (text + image)
interface RenderableToolResultMultimodal {
  kind: "tool_result_multimodal";
  toolUseId: string;      // from content.tool_use_id — pairing key
  blocks: Array<          // from content.content (array form)
    | { type: "text"; text: string }
    | { type: "image"; source: { type: "base64" | "url"; media_type: string; data?: string; url?: string } }
  >;
  isError: boolean;       // from content.is_error ?? false
  // Rendering: same container as generic tool_result, but body renders
  // text blocks as ANSI-converted text and image blocks as inline <img>.
  // Summary: first line of first text block.
}
```

#### 2.5.3 Tool Result — Progressive Enhancement via `toolUseResult`

When the `toolUseResult` envelope field is present and well-typed on the parent `user` record, the generic tool result rendering is enriched. Same container, better content.

**Bash** — Relevance: 8
```typescript
// Source: envelope.toolUseResult where correlated tool is Bash
// Enriches the generic tool_result for this tool_use_id
interface ToolResultEnrichmentBash {
  tool: "Bash";
  stdout: string;         // from toolUseResult.stdout
  stderr: string;         // from toolUseResult.stderr
  returnCode?: number;    // from toolUseResult.returnCode (newer versions)
  interrupted: boolean;   // from toolUseResult.interrupted
  isImage: boolean;       // from toolUseResult.isImage
  // Enrichment: separate stdout/stderr rendering, return code in summary,
  // "interrupted" badge if true, image placeholder if isImage
}
```

**Read** — Relevance: 7
```typescript
// Source: envelope.toolUseResult where correlated tool is Read
interface ToolResultEnrichmentRead {
  tool: "Read";
  filePath: string;       // from toolUseResult.file.filePath
  content: string;        // from toolUseResult.file.content
  numLines: number;       // from toolUseResult.file.numLines
  startLine: number;      // from toolUseResult.file.startLine
  totalLines: number;     // from toolUseResult.file.totalLines
  // Enrichment: summary = "filePath (lines N–M of T)",
  // body = Chroma-highlighted content (language from file extension)
}
```

**Write / Edit** — Relevance: 6
```typescript
// Source: envelope.toolUseResult where correlated tool is Write or Edit
interface ToolResultEnrichmentWriteEdit {
  tool: "Write" | "Edit";
  filePath: string;       // from toolUseResult.filePath
  resultType?: "create" | "update"; // from toolUseResult.type (Write only)
  // Enrichment: summary = "Created filePath" or "Updated filePath"
}
```

**Grep** — Relevance: 5
```typescript
// Source: envelope.toolUseResult where correlated tool is Grep
interface ToolResultEnrichmentGrep {
  tool: "Grep";
  numFiles: number;       // from toolUseResult.numFiles
  numLines: number;       // from toolUseResult.numLines
  // Enrichment: summary = "N matches across M files"
}
```

**Glob** — Relevance: 5
```typescript
// Source: envelope.toolUseResult where correlated tool is Glob
interface ToolResultEnrichmentGlob {
  tool: "Glob";
  numFiles: number;       // from toolUseResult.numFiles
  truncated: boolean;     // from toolUseResult.truncated
  // Enrichment: summary = "N files found" (+ "truncated" badge)
}
```

**Task** — Relevance: 7
```typescript
// Source: envelope.toolUseResult where correlated tool is Task
interface ToolResultEnrichmentTask {
  tool: "Task";
  totalDurationMs: number;  // from toolUseResult.totalDurationMs
  totalTokens: number;      // from toolUseResult.totalTokens
  totalToolUseCount: number; // from toolUseResult.totalToolUseCount
  wasInterrupted: boolean;  // from toolUseResult.wasInterrupted
  // Enrichment: summary = "Sub-agent completed in Xs, N tool calls"
  // or "Sub-agent interrupted"
}
```

### 2.6 Image Block — Action tier

**Relevance: 5** — Pasted screenshots in some sessions.

```typescript
// Source: message.content[] where type === "image"
interface RenderableImage {
  kind: "image";
  mediaType: string;      // from content.source.media_type
  sourceType: "base64" | "url";
  data?: string;          // from content.source.data (base64)
  url?: string;           // from content.source.url
  // Rendering: inline <img> with data URI or external URL.
  // Self-contained HTML (core Relic value) means base64 is inlined.
  // Max-width constraint, click-to-expand.
}
```

### 2.7 Document Block — Action tier

**Relevance: 3** — PDFs attached to conversations. Rare.

```typescript
// Source: message.content[] where type === "document"
interface RenderableDocument {
  kind: "document";
  title?: string;         // from content.title
  // Rendering: placeholder note showing the document title.
  // Binary content is not useful in HTML.
}
```

### 2.8 Server Tool Blocks — Action tier

**Relevance: 2** — Rare content types from MCP/built-in server contexts.

```typescript
// Source: message.content[] where type is one of:
// "server_tool_use", "web_search_tool_result", "web_fetch_tool_result",
// "code_execution_tool_result", etc.
interface RenderableServerToolBlock {
  kind: "server_tool";
  blockType: string;      // the specific server tool type
  data: unknown;          // raw JSON — no consistent schema
  // Rendering: labelled collapsible with JSON dump. RawBlock fallback.
}
```

### 2.9 Unknown Block — fallback

**Relevance: 4** — Forward-compatibility for block types added after Relic's release.

```typescript
// Source: message.content[] where type is not matched by any of the above
interface RenderableUnknownBlock {
  kind: "unknown";
  blockType: string;      // the unrecognised type value
  data: unknown;          // raw JSON preserved verbatim
  // Rendering: labelled collapsible showing block type + raw JSON.
  // Never silently dropped — unknown blocks are forward-compatibility hooks.
}
```

---

## Part 3: User Record Subtypes

A `type: "user"` record can represent several fundamentally different things. Detection order matters — check flags before content patterns.

### 3.1 Human Prompt — Conversation tier

**Relevance: 10**

Detection: `type === "user"` AND none of the flags below AND `message.content` is a string or contains `text` blocks (not exclusively `tool_result` blocks) AND content does not match slash command or command stdout XML patterns.

```typescript
interface RenderableHumanPrompt {
  kind: "human_prompt";
  text: string;           // from message.content (string form) or text blocks
  timestamp?: string;     // from envelope.timestamp
  // Rendering: Conversation tier — full visual weight. Markdown-rendered.
}
```

### 3.2 Tool Feedback (tool_result carrier) — Action tier

**Relevance: 9**

Detection: `type === "user"` AND `message.content` is an array where every element has `type === "tool_result"`.

```typescript
interface RenderableToolFeedback {
  kind: "tool_feedback";
  results: RenderableToolResult[]; // one per tool_result block (generic or multimodal)
  toolUseResult?: unknown;        // from envelope.toolUseResult — structured enrichment data
  timestamp?: string;
  // Rendering: NOT a new turn. Action tier.
  // Each result rendered as collapsible section with pairing link to its tool_use.
}
```

### 3.3 Slash Command — Action tier (call half of command pair)

**Relevance: 5**

Detection: `type === "user"` AND `message.content` (string) matches `<command-name>...</command-name>` XML pattern.

```typescript
interface RenderableSlashCommand {
  kind: "slash_command";
  command: string;        // extracted from XML tag name (e.g. "compact", "plan", "clear", "init")
  args: string;           // extracted from XML tag content
  // Rendering: Action tier. Styled as a monospace command invocation.
  // Per-command icon — the design system should provide distinct icons for each known command.
  // Hypertext link to paired command output if present.
}
```

Known commands (non-exhaustive, for icon assignment): `compact`, `plan`, `clear`, `init`, `review`, `config`, `mcp`, `memory`, `model`, `title`, `vim`, `terminal-setup`.

### 3.4 Command Output — Action tier (response half of command pair)

**Relevance: 5**

Detection: `type === "user"` AND `message.content` (string) matches `<local-command-stdout>...</local-command-stdout>` XML pattern.

```typescript
interface RenderableCommandOutput {
  kind: "command_output";
  content: string;        // extracted from XML tag content
  // Rendering: Action tier. Collapsible section.
  // Summary: first line of content.
  // Body: preformatted text (may contain ANSI — run through terminal-to-html).
  // Hypertext link back to paired slash command.
}
```

### 3.5 Compaction Summary — Meta tier

**Relevance: 6**

Detection: `type === "user"` AND `isCompactSummary === true`.

```typescript
interface RenderableCompactionSummary {
  kind: "compaction_summary";
  text: string;           // from message.content — summary of pre-compaction context
  trigger?: "manual" | "auto"; // from preceding compact_boundary's compactMetadata.trigger, if available
  preTokens?: number;     // from preceding compact_boundary's compactMetadata.preTokens, if available
  // Rendering: Meta tier. Collapsible section.
  // Summary: "Context summary" (with trigger and token count if available).
  // Body: the summary text, Markdown-rendered.
}
```

### 3.6 API Error (synthetic) — Meta tier

**Relevance: 4**

Detection: `isApiErrorMessage === true` OR `message.model === "<synthetic>"`.

```typescript
interface RenderableApiError {
  kind: "api_error";
  errorType: string;      // from envelope.error — "rate_limit", "invalid_request", etc.
  message: string;        // from message.content
  // Rendering: Meta tier. Error callout (distinct styling).
  // NOT rendered as an assistant message.
}
```

### 3.7 Hook Injection — Meta tier

**Relevance: 3**

Detection: `type === "user"` AND `isMeta === true` AND not matching other flag checks.

```typescript
interface RenderableHookInjection {
  kind: "hook_injection";
  content: string;        // from message.content
  // Rendering: Meta tier. Faded system note.
}
```

---

## Part 4: System / Meta Record Renderable Items — Meta tier

### 4.1 Session Header

**Relevance: 7**

```typescript
// Source: system/init record, or inferred from first record's envelope fields
interface RenderableSessionHeader {
  kind: "session_header";
  model?: string;         // from init.model or first assistant message.model
  cwd: string;            // from envelope.cwd
  gitBranch?: string;     // from envelope.gitBranch
  version: string;        // from envelope.version (Claude Code version)
  slug?: string;          // from envelope.slug
  startTime?: string;     // from first record timestamp
  title?: string;         // from summary record, custom-title record, or CLI --name flag
  // Rendering: Meta tier, but positioned as a banner at top of document.
  // Despite being Meta tier, this is structurally prominent — it contextualises everything.
}
```

### 4.2 Compaction Boundary

**Relevance: 5**

```typescript
// Source: system record with subtype === "compact_boundary"
interface RenderableCompactionBoundary {
  kind: "compaction_boundary";
  trigger: "manual" | "auto"; // from compactMetadata.trigger
  preTokens?: number;    // from compactMetadata.preTokens
  timestamp?: string;    // from envelope.timestamp
  // Rendering: Meta tier. Visual separator/rule.
  // Label: "Context compacted" (+ trigger type and token count).
  // Distinct from the compaction summary that follows — this is the event marker,
  // the summary is the content.
}
```

### 4.3 System Error

**Relevance: 4**

```typescript
// Source: system record with subtype === "error"
interface RenderableSystemError {
  kind: "system_error";
  content: string;        // from record content
  level: string;          // from envelope.level — "error", "warning", "info"
  // Rendering: Meta tier. Error/warning callout.
}
```

### 4.4 Parse Error

**Relevance: 3**

```typescript
// Source: generated by parser when a JSONL line fails to decode
interface RenderableParseError {
  kind: "parse_error";
  lineNum: number;
  message: string;        // the error description
  // Rendering: Meta tier. Small warning callout at the correct position in the stream.
}
```

### 4.5 Session Result (headless mode)

**Relevance: 2**

```typescript
// Source: type === "result" record (headless --output-format stream-json only)
interface RenderableSessionResult {
  kind: "session_result";
  subtype: "success" | "error_max_turns" | "error_during_execution" | "error_max_budget_usd" | "error_max_structured_output_retries";
  durationMs?: number;    // from result.duration_ms (success only)
  numTurns?: number;      // from result.num_turns (success only)
  totalCostUsd?: number;  // from result.total_cost_usd (success only)
  result?: string;        // from result.result (success only)
  errors?: string[];      // from result.errors (error subtypes)
  // Rendering: Meta tier. Session outcome banner at end of document.
}
```

### 4.6 Attachment Placeholder

**Relevance: 2**

```typescript
// Source: type === "attachment" record
interface RenderableAttachment {
  kind: "attachment";
  // Rendering: Meta tier. Note that an attachment was present.
  // Content is not available in the JSONL — placeholder only.
}
```

---

## Part 5: Relevance Summary

| Rank | Item | Tier |
|------|------|------|
| **10** | Human prompt | Conversation |
| **10** | Assistant text | Conversation |
| **10** | Tool use: Bash | Action |
| **9** | Tool use: Read, Write, Edit | Action |
| **9** | Tool result (generic) | Action |
| **8** | Tool use: MultiEdit | Action |
| **8** | Enrichment: Bash (stdout/stderr/returnCode) | Action |
| **7** | Thinking block | Process |
| **7** | Tool use: Task (delegation boundary) | Action |
| **7** | Enrichment: Task (duration/tokens) | Action |
| **7** | Enrichment: Read (filePath/lines) | Action |
| **7** | Session header | Meta |
| **6** | Tool use: Glob, Grep, WebFetch, WebSearch | Action |
| **6** | Compaction summary | Meta |
| **6** | Enrichment: Write/Edit (create/update) | Action |
| **5** | Tool use: LS, exit_plan_mode, MCP tools | Action |
| **5** | Tool use: TodoRead/TodoWrite | Action |
| **5** | Tool result: multimodal (array content) | Action |
| **5** | Enrichment: Grep, Glob (counts) | Action |
| **5** | Compaction boundary | Meta |
| **5** | Slash command | Action |
| **5** | Command output | Action |
| **5** | Image block | Action |
| **4** | Redacted thinking | Process |
| **4** | API error (synthetic) | Meta |
| **4** | Tool use: NotebookEdit/Read | Action |
| **4** | Tool use: Unknown | Action |
| **4** | Unknown content block | Action |
| **4** | System error | Meta |
| **3** | Parse error | Meta |
| **3** | Summary record | (title extraction only) |
| **3** | Custom title | (title extraction only) |
| **3** | Document block | Action |
| **3** | Hook injection | Meta |
| **3** | Tool use: BashOutput/KillShell | Action |
| **2** | Server tool blocks | Action |
| **2** | Session result (headless) | Meta |
| **2** | Attachment placeholder | Meta |
| **0** | `file-history-snapshot` | (skip) |
| **0** | `progress` | (skip) |

---

## Part 6: Parser Surface Requirements

The current Relic parser extracts `type`, `message.role`, `message.content`, `timestamp`, and `model`. To support this full taxonomy it also needs:

1. **Envelope flags**: `isSidechain`, `isCompactSummary`, `isApiErrorMessage`, `isMeta`, `version`, `cwd`, `gitBranch`, `slug`
2. **Envelope data**: `toolUseResult` (for progressive enhancement of tool results)
3. **Record-level types beyond user/assistant**: `system` (for init, compact_boundary, error), `summary`, `custom-title`
4. **User record subtype detection**: slash command XML pattern (`<command-name>`), command stdout XML pattern (`<local-command-stdout>`), all-tool-result detection, flag-based classification
5. **Content block additions**: `redacted_thinking` (currently falls through to RawBlock), `image`, `document`, `tool_result` array-form content
6. **Pairing infrastructure**: `tool_use.id` preserved on tool use blocks, `tool_result.tool_use_id` preserved on tool result blocks, index or map for the renderer to link them
