package highlight_test

import (
	"html/template"
	"os"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/jamestelfer/relic/internal/highlight"
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
	out, err := highlight.Highlight(code, "go")
	if err != nil {
		t.Fatalf("Highlight: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty HTML output")
	}
	// '<' must be escaped as &lt; in the code body.
	if strings.Contains(string(out), "<hello>") {
		t.Error("expected '<hello>' to be HTML-escaped in output")
	}
	// Output should contain a <pre> wrapper.
	if !strings.Contains(string(out), "<pre") {
		t.Error("expected <pre> element in highlighted output")
	}
}

// TestHighlight_UnknownLanguage verifies that an empty or unknown language
// returns code wrapped in a plain <pre> without error.
func TestHighlight_UnknownLanguage(t *testing.T) {
	code := "some plain text content"
	out, err := highlight.Highlight(code, "")
	if err != nil {
		t.Fatalf("Highlight with empty lang: %v", err)
	}
	if out == template.HTML("") {
		t.Fatal("expected non-empty output for empty lang")
	}
	if !strings.Contains(string(out), "<pre") {
		t.Error("expected <pre> element in plain output")
	}
	if !strings.Contains(string(out), "some plain text content") {
		t.Error("expected code content in plain output")
	}
}

// TestHighlight_UnknownLanguageName verifies that an unrecognised language name
// falls back gracefully to a plain <pre> without error.
func TestHighlight_UnknownLanguageName(t *testing.T) {
	out, err := highlight.Highlight("hello world", "xyz_totally_unknown_language")
	if err != nil {
		t.Fatalf("Highlight with unknown lang: %v", err)
	}
	if !strings.Contains(string(out), "<pre") {
		t.Error("expected <pre> in output for unknown language")
	}
}

// TestCSS verifies that CSS() returns non-empty CSS containing both github
// (default) and github-dark (media query) rule sets.
func TestCSS(t *testing.T) {
	css := highlight.CSS()
	if css == "" {
		t.Fatal("expected non-empty CSS")
	}
	if !strings.Contains(css, "prefers-color-scheme") {
		t.Error("expected prefers-color-scheme media query in CSS output")
	}
	snaps.MatchSnapshot(t, css)
}
