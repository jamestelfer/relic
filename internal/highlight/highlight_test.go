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

// TestValidateTheme verifies known and unknown theme names.
func TestValidateTheme(t *testing.T) {
	assert.True(t, highlight.ValidateTheme("github"), "github should be a valid theme")
	assert.True(t, highlight.ValidateTheme("monokai"), "monokai should be a valid theme")
	assert.False(t, highlight.ValidateTheme("totally_unknown_xyz"), "unknown theme should be invalid")
}

// TestCSS verifies that CSS() returns non-empty CSS containing both github
// (default) and github-dark (media query) rule sets.
func TestCSS(t *testing.T) {
	css := highlight.CSS()
	require.NotEmpty(t, css)
	assert.Contains(t, css, "prefers-color-scheme")
	snaps.MatchSnapshot(t, css)
}
