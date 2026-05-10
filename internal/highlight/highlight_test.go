package highlight_test

import (
	"os"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/jamestelfer/relic/internal/highlight"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	v := m.Run()
	_, _ = snaps.Clean(m)
	os.Exit(v)
}

// TestHighlight_KnownLanguage verifies that a known language produces non-empty HTML
// with Chroma token spans and that angle brackets in code are HTML-escaped.
func TestHighlight_KnownLanguage(t *testing.T) {
	code := `fmt.Println("<hello>")`
	out, err := highlight.Highlight(code, "go", "github")
	require.NoError(t, err)
	assert.NotEmpty(t, out)
	// '<' must be escaped as &lt; in the code body.
	assert.NotContains(t, string(out), "<hello>", "angle brackets must be HTML-escaped")
	// Output should contain a <pre> wrapper.
	assert.Contains(t, string(out), "<pre")
}

// TestHighlight_UnknownLanguage verifies that an empty or unknown language
// returns code wrapped in a plain <pre> without error.
func TestHighlight_UnknownLanguage(t *testing.T) {
	code := "some plain text content"
	out, err := highlight.Highlight(code, "", "github")
	require.NoError(t, err)
	assert.NotEmpty(t, out)
	assert.Contains(t, string(out), "<pre")
	assert.Contains(t, string(out), "some plain text content")
}

// TestHighlight_UnknownLanguageName verifies that an unrecognised language name
// falls back gracefully to a plain <pre> without error.
func TestHighlight_UnknownLanguageName(t *testing.T) {
	out, err := highlight.Highlight("hello world", "xyz_totally_unknown_language", "")
	require.NoError(t, err)
	assert.Contains(t, string(out), "<pre")
}

// TestCSS verifies that CSS() returns non-empty CSS containing both light and
// dark theme rules, plus structural overrides.
func TestCSS(t *testing.T) {
	css := highlight.CSS()
	require.NotEmpty(t, css)
	assert.Contains(t, css, ".term .chroma")
	assert.NotContains(t, css, "@media (prefers-color-scheme: dark)")
	assert.Contains(t, css, "background-color: var(--bg-code)")
	snaps.MatchSnapshot(t, css)
}

func TestStyle_LoadedFromEmbed(t *testing.T) {
	s := highlight.Style()
	require.NotNil(t, s, "style must load successfully")
	assert.Equal(t, "github", s.Name)
}

func TestHighlight_UserBashSession(t *testing.T) {
	out, err := highlight.Highlight("! echo hello", "userbashsession", "")
	require.NoError(t, err)
	assert.NotEmpty(t, out)
	assert.Contains(t, string(out), "<pre")
	assert.Contains(t, string(out), "echo")
}

func TestDiff_AdditionsAndDeletions(t *testing.T) {
	old := "hello\nworld\n"
	new := "hello\nearth\n"
	out := highlight.Diff(old, new, "test.txt")
	require.NotEmpty(t, out)
	html := string(out)
	// Should contain Chroma CSS classes (class-based, not inline)
	assert.Contains(t, html, "chroma")
	// Should show the diff content
	assert.Contains(t, html, "world")
	assert.Contains(t, html, "earth")
}

func TestDiff_IdenticalStrings(t *testing.T) {
	out := highlight.Diff("same\n", "same\n", "f.go")
	assert.Empty(t, out, "identical strings should produce empty output")
}

func TestDiff_PureAddition(t *testing.T) {
	out := highlight.Diff("", "new content\n", "f.go")
	require.NotEmpty(t, out)
	assert.Contains(t, string(out), "new content")
}

func TestDiff_PureDeletion(t *testing.T) {
	out := highlight.Diff("old content\n", "", "f.go")
	require.NotEmpty(t, out)
	assert.Contains(t, string(out), "old content")
}
