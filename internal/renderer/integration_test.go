package renderer_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jamestelfer/relic/internal/renderer"
	"github.com/jamestelfer/relic/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_AllBlockTypes renders a session containing every block type
// and verifies the output is structurally complete: no block is silently
// dropped, the visual hierarchy is legible (conversation > tools > thinking > meta),
// and the HTML is self-contained.
func TestIntegration_AllBlockTypes(t *testing.T) {
	start := time.Date(2026, 5, 9, 14, 0, 0, 0, time.UTC)
	callID := "toolu_01INTEG"
	linkedID := callID

	s := session.Session{
		Model:         "claude-sonnet-4-6",
		EmbeddedName:  "Integration test session",
		Start:         &start,
		DurationNanos: 42 * time.Minute,
		FilesTouched:  []string{"a.go", "b.go", "c.go"},
		Turns: []session.Turn{
			{
				Index: 1,
				Title: "Initial prompt",
				Start: &start,
				Blocks: []session.Block{
					&session.UserText{Text: "Refactor the **parser** into typed content blocks."},
					&session.AssistantText{Text: "I'll define `ContentBlock` as an interface with concrete types."},
					&session.Thinking{Text: "The cleanest approach uses a type switch..."},
					&session.RedactedThinking{Data: "secret"},
					&session.ToolCall{
						ID: callID, Name: "Bash",
						Input:          map[string]any{"command": "just test ./..."},
						LinkedResultID: &linkedID,
					},
					&session.ToolResult{
						ToolUseID: callID, Content: "PASS\nok github.com/jamestelfer/relic 0.4s",
						LinkedCallID: &linkedID, LinkedCallName: "Bash",
					},
					&session.ToolCall{
						ID: "toolu_02EDIT", Name: "Edit",
						Input: map[string]any{
							"file_path":  "parser.go",
							"old_string": "func old() {}\n",
							"new_string": "func new() {}\n",
						},
					},
					&session.ToolCall{
						ID: "toolu_03READ", Name: "Read",
						Input: map[string]any{"file_path": "main.go"},
					},
					&session.ToolCall{
						ID: "toolu_04WRITE", Name: "Write",
						Input: map[string]any{"file_path": "new.go", "content": "package main\n"},
					},
					&session.ToolCall{
						ID: "toolu_05WS", Name: "WebSearch",
						Input: map[string]any{"query": "go type switch best practices"},
					},
					&session.ToolCall{
						ID: "toolu_06TASK", Name: "Task",
						Input: map[string]any{"prompt": "Audit all block types", "subagent_type": "code-reviewer"},
					},
					&session.ToolCall{
						ID: "toolu_07TODO", Name: "TodoWrite",
						Input: map[string]any{"todos": []any{
							map[string]any{"content": "refactor parser", "status": "completed", "priority": "high"},
							map[string]any{"content": "add tests", "status": "in-progress", "priority": "medium"},
						}},
					},
					&session.ToolCall{
						ID: "toolu_08PLAN", Name: "exit_plan_mode",
						Input: map[string]any{"plan": "## Plan\n1. Add types\n2. Add tests"},
					},
					&session.ToolCall{
						ID: "toolu_09ASK", Name: "AskUserQuestion",
						Input: map[string]any{"questions": []any{
							map[string]any{
								"question":    "Which approach should we take?",
								"header":      "Strategy",
								"options":     []any{map[string]any{"label": "Option A", "description": "First"}, map[string]any{"label": "Option B", "description": "Second"}},
								"multiSelect": false,
							},
						}},
						LinkedResultID: new("toolu_09ASK"),
					},
					&session.ToolResult{
						ToolUseID: "toolu_09ASK", Content: "User has answered your questions.",
						LinkedCallID: new("toolu_09ASK"), LinkedCallName: "AskUserQuestion",
						Enrichment: &session.AskUserQuestionEnrichment{
							Questions: []session.AskQuestionResult{
								{Header: "Strategy", Question: "Which approach should we take?", Options: []string{"Option A", "Option B"}, Selected: []string{"Option B"}},
							},
						},
					},
					&session.ToolCall{
						ID: "toolu_10MCP", Name: "mcp__github__create_issue",
						Input: map[string]any{"title": "test issue", "repo": "relic"},
					},
					&session.Image{MediaType: "image/png", Base64: "iVBORw0KGgo="},
					&session.Error{LineNum: 99, Msg: "unexpected EOF", Raw: []byte(`{"broken":true}`)},
					&session.Raw{RawType: "server_tool_use", Data: []byte(`{"type":"server_tool_use","id":"srv_01"}`)},
				},
			},
			{
				Index: 2,
				Title: "Commands and meta",
				Blocks: []session.Block{
					&session.LocalCommand{Name: "compact", Args: ""},
					&session.CompactionBoundary{Label: "Context compacted · auto"},
					&session.CompactionSummary{Content: "Session covers parser refactor."},
					&session.ApiError{ErrorType: "rate_limit", Message: "Rate limit exceeded"},
					&session.HookInjection{Content: "PreToolUse · Bash · allowed"},
					&session.System{Subtype: "init", Content: "system init"},
					&session.SessionResult{Subtype: "success", DurationMS: 2520000, NumTurns: 2, CostUSD: 0.84},
				},
			},
		},
	}

	out := render(t, s, renderer.Options{Name: "Integration Test"})

	// === Self-contained: no external resources ===
	assert.NotContains(t, out, `<link rel="stylesheet" href=`)
	assert.NotContains(t, out, `<script src=`)
	// Google Fonts @import is the allowed exception
	assert.Contains(t, out, "@import")

	// === Page structure ===
	assert.Contains(t, out, `class="layout"`)
	assert.Contains(t, out, `<aside class="rail"`)
	assert.Contains(t, out, `<main>`)
	assert.Contains(t, out, `class="session-head"`)
	assert.Contains(t, out, `class="session-foot"`)

	// === No block silently dropped ===
	assert.Contains(t, out, `class="block conversation user"`, "user text present")
	assert.Contains(t, out, `class="block conversation assistant"`, "assistant text present")
	assert.Contains(t, out, `class="block thinking"`, "thinking present")
	assert.Contains(t, out, `class="block redacted-thinking"`, "redacted thinking present")
	assert.Contains(t, out, `class="block tool"`, "tool call present")
	assert.Contains(t, out, `class="block tool-result"`, "tool result present")
	assert.Contains(t, out, `class="block image-block"`, "image present")
	assert.Contains(t, out, `class="block error"`, "error present")
	assert.Contains(t, out, `class="block slash-cmd"`, "slash command present")
	assert.Contains(t, out, `class="block meta"`, "meta block present")
	assert.Contains(t, out, `class="meta-separator"`, "compaction boundary present")
	assert.Contains(t, out, `class="block api-error"`, "api error present")
	assert.Contains(t, out, `unknown-block`, "unknown/raw block present")
	assert.Contains(t, out, `class="session-result`, "session result present")

	// === Design system component structures ===
	// Conversation tier: cards with role rows
	assert.Contains(t, out, `<span class="tag">user</span>`)
	assert.Contains(t, out, `<span class="tag">assistant</span>`)

	// Tool tier: role row with pair-id
	assert.Contains(t, out, `<span class="tag">tool_use</span>`)
	assert.Contains(t, out, `<span class="tag">tool_result</span>`)

	// Tool card structure
	assert.Contains(t, out, `data-tool="Bash"`)
	assert.Contains(t, out, `data-tool="Edit"`)
	assert.Contains(t, out, `data-tool="Read"`)
	assert.Contains(t, out, `data-tool="Write"`)
	assert.Contains(t, out, `data-tool="WebSearch"`)
	assert.Contains(t, out, `data-tool="Task"`)
	assert.Contains(t, out, `data-tool="TodoWrite"`)
	assert.Contains(t, out, `data-tool="exit_plan_mode"`)
	assert.Contains(t, out, `data-tool="AskUserQuestion"`)
	assert.Contains(t, out, `data-tool="mcp__github__create_issue"`)

	// WebSearch body structure
	assert.Contains(t, out, `class="websearch-body"`)
	assert.Contains(t, out, `class="ws-label"`)
	assert.Contains(t, out, `class="ws-text"`)

	// Task body structure
	assert.Contains(t, out, `class="task-delegation"`)
	assert.Contains(t, out, `class="task-badge"`)

	// TodoWrite body structure
	assert.Contains(t, out, `class="todo-list"`)
	assert.Contains(t, out, `class="todo-check"`)
	assert.Contains(t, out, `class="todo-text"`)

	// AskUserQuestion tool_use body structure
	assert.Contains(t, out, `class="ask-list"`)
	assert.Contains(t, out, `class="ask-header"`)
	assert.Contains(t, out, `class="ask-question"`)
	assert.Contains(t, out, `class="ask-option"`)

	// AskUserQuestion tool_result body structure
	assert.Contains(t, out, `class="ask-result"`)
	assert.Contains(t, out, `class="ask-result-item"`)
	assert.Contains(t, out, `class="ask-option selected"`)

	// MCP body structure
	assert.Contains(t, out, `class="mcp-server"`)
	assert.Contains(t, out, `class="chroma"`)

	// Image structure
	assert.Contains(t, out, `class="image-card"`)
	assert.Contains(t, out, "data:image/png;base64,")

	// Error structure
	assert.Contains(t, out, `class="err-callout"`)
	assert.Contains(t, out, "Parse error")

	// Unknown block structure
	assert.Contains(t, out, `class="ub-tag"`)
	assert.Contains(t, out, `class="ub-type"`)
	assert.Contains(t, out, "server_tool_use")

	// Session result
	assert.Contains(t, out, `class="sr-icon"`)
	assert.Contains(t, out, `class="sr-stats"`)

	// === CSS present for all component types ===
	styleStart := strings.Index(out, "<style>")
	styleEnd := strings.Index(out, "</style>")
	require.Greater(t, styleStart, 0)
	require.Greater(t, styleEnd, styleStart)
	css := out[styleStart:styleEnd]

	assert.Contains(t, css, ".block.conversation")
	assert.Contains(t, css, ".block.user")
	assert.Contains(t, css, ".block.assistant")
	assert.Contains(t, css, ".block.tool")
	assert.Contains(t, css, ".block.thinking")
	assert.Contains(t, css, ".pair-id")
	assert.Contains(t, css, ".tool-card")
	assert.Contains(t, css, ".term")
	assert.Contains(t, css, ".websearch-body")
	assert.Contains(t, css, ".task-delegation")
	assert.Contains(t, css, ".todo-list")
	assert.Contains(t, css, ".plan-body")
	assert.Contains(t, css, ".ask-list")
	assert.Contains(t, css, ".ask-result")
	assert.Contains(t, css, ".mcp-server")
	assert.Contains(t, css, ".diff-body")
	assert.Contains(t, css, ".image-card")
	assert.Contains(t, css, "details.unknown-block")
	assert.Contains(t, css, ".meta-separator")
	assert.Contains(t, css, ".api-error-card")
	assert.Contains(t, css, ".session-result")
	assert.Contains(t, css, ".err-callout")
	assert.Contains(t, css, ".slash-card")
	assert.Contains(t, css, ".cmd-output-body")
	assert.Contains(t, css, ".redacted-note")
}
