package renderer_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jamestelfer/relic/internal/renderer"
	"github.com/jamestelfer/relic/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func render(t *testing.T, s session.Session, opts renderer.Options) string {
	t.Helper()
	var buf bytes.Buffer
	err := renderer.Render(&buf, s, opts)
	require.NoError(t, err)
	return buf.String()
}

func TestRender_OutputStartsWithDoctype(t *testing.T) {
	out := render(t, session.Session{}, renderer.Options{Name: "test"})
	assert.Contains(t, out, "<!doctype html")
}

func TestRender_EmptySession_ValidHTML(t *testing.T) {
	out := render(t, session.Session{}, renderer.Options{Name: "empty"})
	assert.Contains(t, out, "<!doctype html")
	assert.Contains(t, out, "</html>")
}

func TestRender_Title_ExplicitName(t *testing.T) {
	s := session.Session{EmbeddedName: "embedded-title"}
	out := render(t, s, renderer.Options{Name: "explicit", FilenameFallback: "fallback"})
	assert.Contains(t, out, "<title>explicit")
}

func TestRender_Title_EmbeddedFallback(t *testing.T) {
	s := session.Session{EmbeddedName: "embedded-title"}
	out := render(t, s, renderer.Options{FilenameFallback: "fallback"})
	assert.Contains(t, out, "<title>embedded-title")
}

func TestRender_Title_FilenameFallback(t *testing.T) {
	s := session.Session{}
	out := render(t, s, renderer.Options{FilenameFallback: "fixture"})
	assert.Contains(t, out, "<title>fixture")
}

func TestRender_Title_UntitledSession(t *testing.T) {
	out := render(t, session.Session{}, renderer.Options{})
	assert.Contains(t, out, "<title>untitled session")
}

func TestRender_SingleStyleBlock(t *testing.T) {
	out := render(t, session.Session{}, renderer.Options{Name: "test"})
	count := strings.Count(out, "<style>")
	assert.Equal(t, 1, count, "expected exactly one <style> block, got %d", count)
}

func TestRender_PreservesGoogleFontsImport(t *testing.T) {
	out := render(t, session.Session{}, renderer.Options{Name: "test"})
	assert.Contains(t, out, "@import", "Google Fonts @import should be preserved in output")
}

func TestRender_ChromaCSSInStyleBlock(t *testing.T) {
	out := render(t, session.Session{}, renderer.Options{Name: "test"})
	assert.Contains(t, out, ".chroma", "Chroma CSS classes should be in the style block")
}

func TestRender_UserTextBlockStructure(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.UserText{Text: "hello world"},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, `class="block conversation user"`)
	assert.Contains(t, out, "hello world")

	// Design system: .role row with .tag "user"
	assert.Contains(t, out, `<div class="role"><span class="tag">user</span></div>`)
}

func TestRender_AssistantTextBlockStructure(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "I can help with that."},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, `class="block conversation assistant"`)
	assert.Contains(t, out, "I can help with that.")

	// Design system: .role row with .tag "assistant"
	assert.Contains(t, out, `<div class="role"><span class="tag">assistant</span></div>`)
}

func TestRender_ThinkingBlockCollapsible(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.Thinking{Text: "Let me reason about this problem carefully"},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, `class="block thinking"`)
	assert.Contains(t, out, "<details")
	assert.Contains(t, out, `class="thinking-label"`)
	assert.Contains(t, out, `class="thinking-peek"`)
	assert.Contains(t, out, `class="thinking-chevron"`)
	assert.Contains(t, out, `class="thinking-body"`)
	assert.Contains(t, out, "Let me reason about this problem carefully")
}

func TestRender_ThinkingPeekTruncation(t *testing.T) {
	longText := "This is a very long thinking text that should be truncated when displayed as a peek preview in the collapsed thinking block summary row"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.Thinking{Text: longText},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	// Peek should contain truncated text with ellipsis
	assert.Contains(t, out, `class="thinking-peek"`)
	assert.Contains(t, out, "…")
}

func TestRender_ToolCallHasIDAttribute(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolCall{ID: "toolu_01ABC", Name: "Bash", Input: map[string]any{"command": "ls"}},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, `id="toolu_01ABC"`)
}

func TestRender_ToolResultHasIDAttribute(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{ToolUseID: "toolu_01ABC", Content: "output", LinkedCallName: "Bash"},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	// Result uses "result-" prefix to distinguish from the call's anchor
	assert.Contains(t, out, `id="result-toolu_01ABC"`)
}

func TestRender_PairIDBadgeLinks(t *testing.T) {
	callID := "toolu_01ABCDEF"
	linkedID := callID
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolCall{ID: callID, Name: "Bash", Input: map[string]any{"command": "ls"}, LinkedResultID: &linkedID},
			&session.ToolResult{ToolUseID: callID, Content: "file.txt", LinkedCallID: &linkedID, LinkedCallName: "Bash"},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	// Call's badge links to the result
	assert.Contains(t, out, `href="#result-toolu_01ABCDEF"`)
	// Result's badge links back to the call
	assert.Contains(t, out, `href="#toolu_01ABCDEF"`)
}

func TestRender_ToolCallStructure_RoleRowWithPairID(t *testing.T) {
	callID := "toolu_01XYZ789"
	linkedID := callID
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolCall{ID: callID, Name: "Read", Input: map[string]any{"file_path": "/tmp/x.go"}, LinkedResultID: &linkedID},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	// Design system: tool call has .role row containing .tag "tool_use" and .pair-id badge
	assert.Contains(t, out, `<span class="tag">tool_use</span>`)
	assert.Contains(t, out, `class="pair-id"`)
	// pair-id badge must be in the .role row, NOT in .tool-card .head
	roleStart := strings.Index(out, `<span class="tag">tool_use</span>`)
	require.Greater(t, roleStart, 0)
	roleEnd := strings.Index(out[roleStart:], `</div>`)
	roleRow := out[roleStart : roleStart+roleEnd]
	assert.Contains(t, roleRow, `class="pair-id"`, "pair-id badge should be in .role row")
}

func TestRender_ToolResultStructure_RoleRowWithPairID(t *testing.T) {
	callID := "toolu_01XYZ789"
	linkedID := callID
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{ToolUseID: callID, Content: "output", LinkedCallID: &linkedID, LinkedCallName: "Bash"},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	// Design system: tool result has .role row containing .tag "tool_result" and .pair-id badge
	assert.Contains(t, out, `<span class="tag">tool_result</span>`)
	assert.Contains(t, out, `class="pair-id"`)
	// pair-id badge in role row, NOT in .term .chrome
	roleStart := strings.Index(out, `<span class="tag">tool_result</span>`)
	require.Greater(t, roleStart, 0)
	roleEnd := strings.Index(out[roleStart:], `</div>`)
	roleRow := out[roleStart : roleStart+roleEnd]
	assert.Contains(t, roleRow, `class="pair-id"`, "pair-id badge should be in .role row")
}

func TestRender_EditToolShowsDiff(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolCall{ID: "toolu_edit1", Name: "Edit", Input: map[string]any{
				"file_path":  "/tmp/test.go",
				"old_string": "hello\n",
				"new_string": "world\n",
			}},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, `data-tool="Edit"`)
	assert.Contains(t, out, "chroma", "diff should use Chroma CSS classes")
}

func TestRender_MultiEditToolShowsDiffs(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolCall{ID: "toolu_me1", Name: "MultiEdit", Input: map[string]any{
				"file_path": "/tmp/test.go",
				"edits": []any{
					map[string]any{"old_string": "a\n", "new_string": "b\n"},
					map[string]any{"old_string": "x\n", "new_string": "y\n"},
				},
			}},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, `data-tool="MultiEdit"`)
	assert.Contains(t, out, "chroma")
}

func TestRender_WebSearchTool(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolCall{ID: "toolu_ws1", Name: "WebSearch", Input: map[string]any{"query": "golang concurrency"}},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "ws-query")
	assert.Contains(t, out, "golang concurrency")
}

func TestRender_TaskTool(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolCall{ID: "toolu_t1", Name: "Task", Input: map[string]any{
				"prompt":        "Research this topic",
				"subagent_type": "code-reviewer",
			}},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "task-badge")
	assert.Contains(t, out, "code-reviewer")
	assert.Contains(t, out, "Research this topic")
}

func TestRender_TodoWriteTool(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolCall{ID: "toolu_td1", Name: "TodoWrite", Input: map[string]any{
				"todos": []any{
					map[string]any{"content": "write tests", "status": "completed", "priority": "high"},
					map[string]any{"content": "deploy", "status": "in-progress", "priority": "medium"},
				},
			}},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "todo-list")
	assert.Contains(t, out, "write tests")
	assert.Contains(t, out, "deploy")
}

func TestRender_MCPTool(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolCall{ID: "toolu_mcp1", Name: "mcp__myserver__mytool", Input: map[string]any{"key": "value"}},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "mcp-server")
	assert.Contains(t, out, "myserver")
	assert.Contains(t, out, "mytool")
}

func TestRender_UnknownTool(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolCall{ID: "toolu_unk", Name: "FancyNewTool", Input: map[string]any{"arg1": "val1"}},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, `data-tool="FancyNewTool"`)
}

func TestRender_BashToolHighlighted(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolCall{ID: "toolu_bash1", Name: "Bash", Input: map[string]any{"command": "echo hello"}},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, `data-tool="Bash"`)
	assert.Contains(t, out, "$", "Bash icon should be $")
}

func TestRender_OrphanedToolResult(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{ToolUseID: "toolu_orphan", Content: "some output"},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "tool_result", "orphaned result should show 'tool_result' label")
	// Orphaned: LinkedCallID is nil so no pair-id link renders
	assert.NotContains(t, out, `pair-id" href=`, "orphaned result should not have pair-id link")
}

func TestRender_RedactedThinkingNotExpandable(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.RedactedThinking{Data: "secret"},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, `class="block redacted-thinking"`)
	assert.Contains(t, out, `class="redacted-note"`)
	assert.NotContains(t, out, "secret", "redacted data must not appear in output")

	// Design system: redacted-note contains 🔒 icon
	start := strings.Index(out, `class="block redacted-thinking"`)
	require.Greater(t, start, 0)
	end := strings.Index(out[start:], "</article>")
	require.Greater(t, end, 0)
	article := out[start : start+end]
	assert.NotContains(t, article, "<details", "redacted thinking must not be expandable")
	assert.Contains(t, article, "🔒", "redacted thinking should use 🔒 icon per design system")
}

func TestRender_WebSearchStructure(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolCall{ID: "toolu_ws1", Name: "WebSearch", Input: map[string]any{"query": "golang testing"}},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	// Design system: .websearch-body > .ws-query > .ws-label + .ws-text
	assert.Contains(t, out, `class="websearch-body"`)
	assert.Contains(t, out, `class="ws-query"`)
	assert.Contains(t, out, `class="ws-label"`)
	assert.Contains(t, out, `class="ws-text"`)
	assert.Contains(t, out, "golang testing")
}

func TestRender_TaskStructure(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolCall{ID: "toolu_t1", Name: "Task", Input: map[string]any{
				"prompt":        "Research this topic",
				"subagent_type": "code-reviewer",
			}},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	// Design system: .task-delegation > .task-meta > .task-badge + .task-type
	assert.Contains(t, out, `class="task-delegation"`)
	assert.Contains(t, out, `class="task-meta"`)
	assert.Contains(t, out, `class="task-badge"`)
	assert.Contains(t, out, `class="task-prompt"`)
}

func TestRender_TodoWriteStructure(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolCall{ID: "toolu_td1", Name: "TodoWrite", Input: map[string]any{
				"todos": []any{
					map[string]any{"content": "write tests", "status": "completed", "priority": "high"},
				},
			}},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	// Design system: .todo-list > .todo-item > .todo-check + .todo-text + .todo-priority
	assert.Contains(t, out, `class="todo-list"`)
	assert.Contains(t, out, `class="todo-check"`)
	assert.Contains(t, out, `class="todo-text"`)
	assert.Contains(t, out, `class="todo-priority`)
}

func TestRender_MCPToolStructure(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolCall{ID: "toolu_mcp1", Name: "mcp__github__create_issue", Input: map[string]any{"title": "test"}},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	// Design system: .mcp-server breadcrumb + Chroma-highlighted JSON body
	assert.Contains(t, out, `class="mcp-server"`)
	assert.Contains(t, out, `class="chroma"`)
	assert.Contains(t, out, "github")
	assert.Contains(t, out, "create_issue")
}

func TestRender_ImageBlock(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.Image{
				MediaType: "image/png",
				Base64:    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	// Design system: .block.image-block > .role + .image-card > .image-head + img.image-preview
	assert.Contains(t, out, `class="block image-block"`)
	assert.Contains(t, out, `class="image-card"`)
	assert.Contains(t, out, `class="image-head"`)
	assert.Contains(t, out, `class="image-preview"`)
	assert.Contains(t, out, "data:image/png;base64,")
	assert.Contains(t, out, "click to expand")
}

func TestRender_UnknownBlock(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.Raw{RawType: "server_tool_use", Data: []byte(`{"type":"server_tool_use","id":"srvtool_01"}`)},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	// Design system: .block.meta > details.unknown-block > summary(.ub-tag + .ub-type + .ub-chevron) + chroma JSON
	assert.Contains(t, out, `class="block meta"`)
	assert.Contains(t, out, `unknown-block`)
	assert.Contains(t, out, `class="ub-tag"`)
	assert.Contains(t, out, `class="ub-type"`)
	assert.Contains(t, out, `class="ub-chevron"`)
	assert.Contains(t, out, "server_tool_use")
}

func TestRender_CompactSummaryBlock(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.CompactionSummary{Content: "Context was compacted."},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	assert.Contains(t, out, `class="block meta"`)
	assert.Contains(t, out, `compact-summary`)
	assert.Contains(t, out, `class="cs-label"`)
	assert.Contains(t, out, `class="cs-meta"`)
	assert.Contains(t, out, `class="cs-chevron"`)
	assert.Contains(t, out, "Context was compacted.")
}

func TestRender_CSSContainsDesignSystemComponents(t *testing.T) {
	out := render(t, session.Session{}, renderer.Options{Name: "test"})

	// Key design system CSS selectors that must be present
	assert.Contains(t, out, ".block.conversation", "conversation card styles")
	assert.Contains(t, out, ".block.user", "user color styles")
	assert.Contains(t, out, ".block.assistant", "assistant color styles")
	assert.Contains(t, out, ".block.tool", "tool color styles")
	assert.Contains(t, out, ".pair-id", "pair-id badge styles")
	assert.Contains(t, out, ".websearch-body", "websearch body styles")
	assert.Contains(t, out, ".task-delegation", "task delegation styles")
	assert.Contains(t, out, ".todo-list", "todo list styles")
	assert.Contains(t, out, ".mcp-server", "mcp server styles")
	assert.Contains(t, out, ".diff-body", "diff body styles")
	assert.Contains(t, out, ".image-card", "image card styles")
	assert.Contains(t, out, "details.unknown-block", "unknown block styles")
}

func TestRender_HookInjection_UTF8NotCorrupted(t *testing.T) {
	// 79 ASCII chars + a 3-byte UTF-8 rune (ä) at position 80.
	// Byte-slicing at 80 would split the rune; rune-aware truncation must not.
	content := strings.Repeat("a", 79) + "äbc"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.HookInjection{Content: content},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.True(t, strings.ContainsRune(out, 'ä'),
		"multi-byte rune at truncation boundary must not be corrupted")
}

func TestRender_OutlineSingularTurn(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.UserText{Text: "hello"},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "1 turn", "single turn should use singular form")
	assert.NotContains(t, out, "1 turns", "single turn must not use plural form")
}

func TestRender_OutlinePluralTurns(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{
			{Index: 1, Blocks: []session.Block{&session.UserText{Text: "first"}}},
			{Index: 2, Blocks: []session.Block{&session.UserText{Text: "second"}}},
		},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "2 turns", "multiple turns should use plural form")
}

func TestRender_ThemeToggleButton(t *testing.T) {
	out := render(t, session.Session{}, renderer.Options{Name: "test"})
	assert.Contains(t, out, `class="theme-toggle"`, "toggle button must be present")
	assert.Contains(t, out, `aria-label=`, "toggle button must have accessible label")
}

func TestRender_ThemeToggleScript(t *testing.T) {
	out := render(t, session.Session{}, renderer.Options{Name: "test"})
	assert.Contains(t, out, `data-theme`, "script must reference data-theme attribute")
	assert.NotContains(t, out, "localStorage", "toggle must not persist state")
}

func TestRender_ThemeOverrideCSS(t *testing.T) {
	out := render(t, session.Session{}, renderer.Options{Name: "test"})
	assert.Contains(t, out, `[data-theme="dark"]`, "CSS must include dark override selector")
	assert.Contains(t, out, `[data-theme="light"]`, "CSS must include light override selector")
}

func TestRender_BashEnrichment_StderrSeparator(t *testing.T) {
	linkedID := "toolu_bash_enr"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID:      linkedID,
				Content:        "raw content ignored when enriched",
				LinkedCallID:   &linkedID,
				LinkedCallName: "Bash",
				Enrichment: &session.BashEnrichment{
					Stdout: "hello world",
					Stderr: "warning: something",
				},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "hello world")
	assert.Contains(t, out, "warning: something")
	assert.Contains(t, out, "stderr shown below", "stderr separator must be present")
}

func TestRender_BashEnrichment_NoStderr(t *testing.T) {
	linkedID := "toolu_bash_enr2"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID:      linkedID,
				Content:        "raw content",
				LinkedCallID:   &linkedID,
				LinkedCallName: "Bash",
				Enrichment: &session.BashEnrichment{
					Stdout: "only stdout here",
					Stderr: "",
				},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "only stdout here")
	assert.NotContains(t, out, "stderr shown below", "no stderr separator when stderr is empty")
}

func TestRender_BashEnrichment_InterruptedLabel(t *testing.T) {
	linkedID := "toolu_bash_int"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID:      linkedID,
				Content:        "partial output",
				LinkedCallID:   &linkedID,
				LinkedCallName: "Bash",
				Enrichment: &session.BashEnrichment{
					Stdout:      "partial output",
					Interrupted: true,
				},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "interrupted", "chrome label should include 'interrupted' for interrupted Bash")
}

func TestRender_BashEnrichment_NormalLabel(t *testing.T) {
	linkedID := "toolu_bash_norm"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID:      linkedID,
				Content:        "ok",
				LinkedCallID:   &linkedID,
				LinkedCallName: "Bash",
				Enrichment: &session.BashEnrichment{
					Stdout: "ok",
				},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.NotContains(t, out, "interrupted", "non-interrupted Bash should not show 'interrupted'")
	// Chrome label should still show "Bash"
	assert.Contains(t, out, `<span class="label">Bash</span>`)
}

func TestRender_ReadEnrichment_WithLineRange(t *testing.T) {
	linkedID := "toolu_read_enr"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID:      linkedID,
				Content:        "file content",
				LinkedCallID:   &linkedID,
				LinkedCallName: "Read",
				Enrichment: &session.ReadEnrichment{
					FilePath:  "/src/internal/parser/parser.go",
					LineStart: 10,
					LineEnd:   59,
				},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "Read · parser.go:10-59", "chrome label should show tool · basename:start-end")
}

func TestRender_ReadEnrichment_NoLineRange(t *testing.T) {
	linkedID := "toolu_read_enr2"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID:      linkedID,
				Content:        "file content",
				LinkedCallID:   &linkedID,
				LinkedCallName: "Read",
				Enrichment: &session.ReadEnrichment{
					FilePath: "/src/internal/parser/parser.go",
				},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "Read · parser.go", "chrome label should show tool · basename")
	assert.NotContains(t, out, "parser.go:", "no line range when absent")
}

func TestRender_WriteEnrichment_Created(t *testing.T) {
	linkedID := "toolu_write_c"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID: linkedID, Content: "ok", LinkedCallID: &linkedID, LinkedCallName: "Write",
				Enrichment: &session.WriteEnrichment{FilePath: "/src/new_file.go", Action: "created"},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "Write · new_file.go · created", "chrome label: tool · basename · created")
}

func TestRender_WriteEnrichment_Updated(t *testing.T) {
	linkedID := "toolu_write_u"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID: linkedID, Content: "ok", LinkedCallID: &linkedID, LinkedCallName: "Write",
				Enrichment: &session.WriteEnrichment{FilePath: "/src/existing.go", Action: "updated"},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "Write · existing.go · updated", "chrome label: tool · basename · updated")
}

func TestRender_EditEnrichment(t *testing.T) {
	linkedID := "toolu_edit_enr"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID: linkedID, Content: "ok", LinkedCallID: &linkedID, LinkedCallName: "Edit",
				Enrichment: &session.EditEnrichment{FilePath: "/src/session.go", Action: "updated"},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "Edit · session.go · updated", "chrome label: tool · basename · updated")
}

func TestRender_GrepEnrichment_ContentMode(t *testing.T) {
	linkedID := "toolu_grep_c"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID: linkedID, Content: "file.go:42:match", LinkedCallID: &linkedID, LinkedCallName: "Grep",
				Enrichment: &session.GrepEnrichment{Mode: "content", NumFiles: 3, NumLines: 25},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "Grep · 25 lines in 3 files", "chrome label: tool · content mode summary")
}

func TestRender_GrepEnrichment_FilesMode(t *testing.T) {
	linkedID := "toolu_grep_f"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID: linkedID, Content: "a.go\nb.go", LinkedCallID: &linkedID, LinkedCallName: "Grep",
				Enrichment: &session.GrepEnrichment{Mode: "files_with_matches", NumFiles: 7},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "Grep · 7 files", "chrome label: tool · files_with_matches mode")
}

func TestRender_GrepEnrichment_CountMode(t *testing.T) {
	linkedID := "toolu_grep_n"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID: linkedID, Content: "25", LinkedCallID: &linkedID, LinkedCallName: "Grep",
				Enrichment: &session.GrepEnrichment{Mode: "count", NumMatches: 25},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "Grep · 25 matches", "chrome label: tool · count mode shows match count")
}

func TestRender_GrepEnrichment_CountMode_NoData(t *testing.T) {
	linkedID := "toolu_grep_n0"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID: linkedID, Content: "0", LinkedCallID: &linkedID, LinkedCallName: "Grep",
				Enrichment: &session.GrepEnrichment{Mode: "count"},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, `<span class="label">Grep</span>`, "chrome label: falls back to tool name when no match count")
}

func TestRender_GlobEnrichment(t *testing.T) {
	linkedID := "toolu_glob"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID: linkedID, Content: "a.go\nb.go", LinkedCallID: &linkedID, LinkedCallName: "Glob",
				Enrichment: &session.GlobEnrichment{NumFiles: 8, Truncated: false},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "Glob · 8 files", "chrome label: tool · file count")
	assert.NotContains(t, out, "truncated", "no truncated marker when not truncated")
}

func TestRender_GlobEnrichment_Truncated(t *testing.T) {
	linkedID := "toolu_glob_t"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID: linkedID, Content: "a.go", LinkedCallID: &linkedID, LinkedCallName: "Glob",
				Enrichment: &session.GlobEnrichment{NumFiles: 100, Truncated: true},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "Glob · 100 files (truncated)", "chrome label: tool · truncated glob")
}

func TestRender_AgentEnrichment(t *testing.T) {
	linkedID := "toolu_agent"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID: linkedID, Content: "done", LinkedCallID: &linkedID, LinkedCallName: "Agent",
				Enrichment: &session.AgentEnrichment{AgentType: "general-purpose", Status: "completed", DurationMs: 45000, TokenCount: 12500, ToolUseCount: 8},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "Agent · general-purpose · 45s · 12.5k tokens", "chrome label: tool · agent type · duration · tokens")
}

func TestRender_AgentEnrichment_SubSecond(t *testing.T) {
	linkedID := "toolu_agent_s"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID: linkedID, Content: "done", LinkedCallID: &linkedID, LinkedCallName: "Agent",
				Enrichment: &session.AgentEnrichment{AgentType: "Explore", Status: "completed", DurationMs: 500, TokenCount: 800},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "&lt;1s", "chrome label: sub-second duration")
	assert.Contains(t, out, "800 tokens", "chrome label: exact token count under 1000")
}

func TestRender_AgentEnrichment_Minutes(t *testing.T) {
	linkedID := "toolu_agent_m"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID: linkedID, Content: "done", LinkedCallID: &linkedID, LinkedCallName: "Agent",
				Enrichment: &session.AgentEnrichment{AgentType: "code-reviewer", Status: "completed", DurationMs: 125000, TokenCount: 1000},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "2m 5s", "chrome label: minutes+seconds")
	assert.Contains(t, out, "1.0k tokens", "chrome label: exactly 1000 tokens")
}

func TestRender_ToolResult_NoEnrichment_Unchanged(t *testing.T) {
	linkedID := "toolu_no_enr"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID:      linkedID,
				Content:        "normal output",
				LinkedCallID:   &linkedID,
				LinkedCallName: "Read",
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "normal output")
	assert.Contains(t, out, `<span class="label">Read</span>`)
}

func TestRender_ToolResult_NilTypedEnrichment_NoPanic(t *testing.T) {
	linkedID := "toolu_nil_enr"
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID:      linkedID,
				Content:        "file content",
				LinkedCallID:   &linkedID,
				LinkedCallName: "Read",
				Enrichment:     (*session.ReadEnrichment)(nil),
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, `<span class="label">Read</span>`, "should fall back to tool name when enrichment is typed nil")
}

func TestRender_AskUserQuestion_ResultWithEnrichment(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID: "toolu_ask", Content: "User has answered your questions.",
				LinkedCallID: new("toolu_ask"), LinkedCallName: "AskUserQuestion",
				Enrichment: &session.AskUserQuestionEnrichment{
					Questions: []session.AskQuestionResult{
						{Header: "Scope", Question: "Which approach?", Options: []string{"Option A", "Option B"}, Selected: []string{"Option B"}},
					},
				},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	assert.Contains(t, out, `class="ask-result"`, "should render ask-result container")
	assert.Contains(t, out, `class="ask-result-item"`, "should render ask-result-item")
	assert.Contains(t, out, `class="ask-header"`, "should render header")
	assert.Contains(t, out, `class="ask-question"`, "should render question")
	assert.Contains(t, out, `class="ask-option selected"`, "selected option has selected class")
	assert.Contains(t, out, `class="ask-option"`, "unselected option has no selected class")
	assert.NotContains(t, out, `class="term"`, "ask-result should not use terminal chrome")
}

func TestRender_AskUserQuestion_ResultWithFreetext(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID: "toolu_ask_ft", Content: "User has answered.",
				LinkedCallID: new("toolu_ask_ft"), LinkedCallName: "AskUserQuestion",
				Enrichment: &session.AskUserQuestionEnrichment{
					Questions: []session.AskQuestionResult{
						{Header: "Output", Question: "Where to save?", Options: []string{"Desktop", "Temp"}, Selected: []string{"Desktop"}, Freetext: "Put it on ~/Desktop/out.html"},
					},
				},
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	assert.Contains(t, out, `class="ask-freetext"`, "should render freetext block")
	assert.Contains(t, out, "Put it on ~/Desktop/out.html", "freetext content present")
}

func TestRender_SystemXML_Highlighted(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.SystemXML{
				TagName: "system-reminder",
				Label:   "system-reminder",
				Content: "<system-reminder>\nYou are a helpful assistant.\n</system-reminder>",
				LineNum: 1,
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	assert.Contains(t, out, `class="block meta"`, "rendered as meta block")
	assert.Contains(t, out, "system-reminder", "label shown")
	assert.Contains(t, out, `class="chroma"`, "Chroma highlighting applied")
}

func TestRender_SystemXML_OriginKindLabel(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.SystemXML{
				TagName: "unknown-tag",
				Label:   "custom-origin",
				Content: "<unknown-tag>content</unknown-tag>",
				LineNum: 1,
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	assert.Contains(t, out, "custom-origin", "origin.kind label shown")
}

func TestRender_FormatXML_Broken(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.SystemXML{
				TagName: "broken",
				Label:   "broken",
				Content: "<broken",
				LineNum: 1,
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	assert.Contains(t, out, "&lt;broken", "broken XML escaped in output")
}

func TestRender_TaskNotification(t *testing.T) {
	cases := []struct {
		name        string
		notif       session.TaskNotification
		contains    []string
		notContains []string
	}{
		{
			name: "completed card",
			notif: session.TaskNotification{
				TaskID:     "abc123",
				ToolUseID:  "toolu_01",
				OutputFile: "/tmp/out.txt",
				Status:     "completed",
				Summary:    "Agent finished",
				Result:     "All done",
				Usage:      session.TaskNotificationUsage{TotalTokens: 5000, ToolUses: 3, DurationMs: 12000},
				LineNum:    1,
			},
			contains: []string{
				`class="block tool-result"`,
				`class="result-card"`,
				`class="rc-badge completed"`,
				"Agent finished",
				`class="rc-body"`,
				"<p>All done</p>",
				"abc123",
				"/tmp/out.txt",
				"5.0k tokens",
			},
		},
		{
			name: "failed card",
			notif: session.TaskNotification{
				TaskID:  "def456",
				Status:  "failed",
				Summary: "Task crashed",
				Result:  "Error: something went wrong",
				LineNum: 1,
			},
			contains: []string{
				`class="rc-badge failed"`,
				"Task crashed",
				"something went wrong",
			},
		},
		{
			name: "running with no result omits section",
			notif: session.TaskNotification{
				TaskID:  "ghi789",
				Status:  "running",
				Summary: "Still working",
				LineNum: 1,
			},
			contains:    []string{`class="rc-badge running"`},
			notContains: []string{`class="rc-body"`},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := session.Session{
				Turns: []session.Turn{{Index: 1, Blocks: []session.Block{&c.notif}}},
			}
			out := render(t, s, renderer.Options{Name: "test"})

			for _, want := range c.contains {
				assert.Contains(t, out, want)
			}
			for _, unwanted := range c.notContains {
				assert.NotContains(t, out, unwanted)
			}
		})
	}
}

func TestRender_TeammateMessage_MarkdownBody(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.TeammateMessage{
				TeammateID: "agent-1",
				From:       "parent",
				To:         "main",
				Content:    "## Summary\n\n- Item 1\n- Item 2",
				LineNum:    1,
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	assert.Contains(t, out, `class="block meta"`, "rendered as meta block")
	assert.Contains(t, out, "teammate", "label shown")
	assert.Contains(t, out, "agent-1", "teammate ID shown")
	assert.Contains(t, out, "Summary</h2>", "markdown heading rendered to HTML")
	assert.Contains(t, out, "<li>Item 1", "markdown list rendered")
}

func TestRender_AskUserQuestion_NilEnrichment_FallsBack(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.ToolResult{
				ToolUseID: "toolu_ask_nil", Content: "User has answered your questions.",
				LinkedCallID: new("toolu_ask_nil"), LinkedCallName: "AskUserQuestion",
			},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})

	assert.NotContains(t, out, `class="ask-result"`, "no ask-result when enrichment is nil")
	assert.Contains(t, out, `class="term"`, "falls back to terminal chrome")
	assert.Contains(t, out, "User has answered your questions.", "raw content shown in fallback")
}
