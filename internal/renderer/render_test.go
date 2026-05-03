package renderer

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/jamestelfer/relic/internal/parser"
)

func TestMain(m *testing.M) {
	v := m.Run()
	_, _ = snaps.Clean(m)
	os.Exit(v)
}

// TestRenderMarkdown_PlainText verifies plain paragraph text is wrapped in <p>.
func TestRenderMarkdown_PlainText(t *testing.T) {
	out := renderMarkdown("Hello, world.")
	snaps.MatchSnapshot(t, string(out))
}

// TestRenderMarkdown_Heading verifies ATX headings produce <h1>–<h6>.
func TestRenderMarkdown_Heading(t *testing.T) {
	out := renderMarkdown("# My Heading\n\nBody text.")
	snaps.MatchSnapshot(t, string(out))
}

// TestRenderMarkdown_CodeFence verifies fenced code blocks produce <pre><code>.
func TestRenderMarkdown_CodeFence(t *testing.T) {
	out := renderMarkdown("```go\nfunc Add(a, b int) int { return a + b }\n```")
	snaps.MatchSnapshot(t, string(out))
}

// TestRenderMarkdown_InlineCode verifies inline code produces <code>.
func TestRenderMarkdown_InlineCode(t *testing.T) {
	out := renderMarkdown("Use `fmt.Println` to print output.")
	snaps.MatchSnapshot(t, string(out))
}

// renderBlock renders a single ContentBlock to an HTML string via the templ component.
func renderBlock(t *testing.T, block parser.ContentBlock) string {
	t.Helper()
	var buf bytes.Buffer
	if err := contentBlock(block).Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
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

	if !contains(out, "<details") {
		t.Error("expected <details> element for tool_use block")
	}
	if !contains(out, "<summary") {
		t.Error("expected <summary> element for tool_use block")
	}
}

// TestRenderThinkingBlock verifies that thinking blocks render as <details> labelled
// "Internal reasoning" with a preview height.
func TestRenderThinkingBlock(t *testing.T) {
	block := &parser.ThinkingBlock{
		Thinking: "Line one\nLine two\nLine three",
	}
	out := renderBlock(t, block)
	snaps.MatchSnapshot(t, out)

	if !contains(out, "<details") {
		t.Error("expected <details> element for thinking block")
	}
	if !contains(out, "Internal reasoning") {
		t.Error("expected 'Internal reasoning' label in thinking block")
	}
}

// TestToolIconFallback verifies that unknown tool names return the default icon.
func TestToolIconFallback(t *testing.T) {
	icon := toolIcon("UnknownTool")
	if icon != "⚙" {
		t.Errorf("toolIcon(%q) = %q, want %q", "UnknownTool", icon, "⚙")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexString(s, substr) >= 0)
}

func indexString(s, substr string) int {
	for i := range s {
		if i+len(substr) <= len(s) && s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
