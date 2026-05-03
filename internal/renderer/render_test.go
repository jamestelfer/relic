package renderer

import (
	"os"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
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
