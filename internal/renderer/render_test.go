package renderer

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/jamestelfer/relic/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	v := m.Run()
	_, _ = snaps.Clean(m)
	os.Exit(v)
}

func TestRenderMarkdown_PlainText(t *testing.T) {
	out := renderMarkdown("Hello, world.", "github")
	snaps.MatchSnapshot(t, string(out))
}

func TestRenderMarkdown_Heading(t *testing.T) {
	out := renderMarkdown("# My Heading\n\nBody text.", "github")
	snaps.MatchSnapshot(t, string(out))
}

func TestRenderMarkdown_CodeFence(t *testing.T) {
	out := renderMarkdown("```go\nfunc Add(a, b int) int { return a + b }\n```", "github")
	snaps.MatchSnapshot(t, string(out))
}

func TestRenderMarkdown_InlineCode(t *testing.T) {
	out := renderMarkdown("Use `fmt.Println` to print output.", "github")
	snaps.MatchSnapshot(t, string(out))
}

// renderBlock renders a single ContentBlock to an HTML string via the templ component.
func renderBlock(t *testing.T, block parser.ContentBlock) string {
	t.Helper()
	var buf bytes.Buffer
	err := contentBlock(block, "github").Render(context.Background(), &buf)
	require.NoError(t, err)
	return buf.String()
}

// TestRenderToolUseBlock verifies that tool_use blocks render as <details> with summary.
func TestRenderToolUseBlock(t *testing.T) {
	block := &parser.ToolUseBlock{
		ID:    "toolu_01",
		Name:  "Bash",
		Input: map[string]any{"command": "echo hello world"},
	}
	out := renderBlock(t, block)
	snaps.MatchSnapshot(t, out)
	assert.Contains(t, out, "<details")
	assert.Contains(t, out, "<summary")
}

// TestRenderThinkingBlock verifies that thinking blocks render as <details> labelled
// "Internal reasoning" with a preview height.
func TestRenderThinkingBlock(t *testing.T) {
	block := &parser.ThinkingBlock{
		Thinking: "Line one\nLine two\nLine three",
	}
	out := renderBlock(t, block)
	snaps.MatchSnapshot(t, out)
	assert.Contains(t, out, "<details")
	assert.Contains(t, out, "Internal reasoning")
}

// TestToolIconFallback verifies that unknown tool names return the default icon.
func TestToolIconFallback(t *testing.T) {
	assert.Equal(t, "⚙", toolIcon("UnknownTool"))
}

// TestRenderToolResultBlock verifies tool_result blocks render as <details> with ANSI converted.
func TestRenderToolResultBlock(t *testing.T) {
	block := &parser.ToolResultBlock{
		ToolUseID: "toolu_01",
		Content:   "\x1b[32mSuccess!\x1b[0m\nOutput line 2",
	}
	out := renderBlock(t, block)
	snaps.MatchSnapshot(t, out)
	assert.Contains(t, out, "<details")
	assert.NotContains(t, out, "\x1b", "ANSI escape byte must be absent in rendered tool_result")
}
