package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
)

func TestMain(m *testing.M) {
	v := m.Run()
	_, _ = snaps.Clean(m)
	os.Exit(v)
}

// runFixture executes relic against testdata/fixture.jsonl and returns the HTML.
func runFixture(t *testing.T, opts options) string {
	t.Helper()
	tmp := t.TempDir()
	if opts.outputPath == "" {
		opts.outputPath = filepath.Join(tmp, "out.html")
	}
	if opts.inputPath == "" {
		opts.inputPath = "testdata/fixture.jsonl"
	}
	var logBuf bytes.Buffer
	if err := execute(opts, &logBuf); err != nil {
		t.Fatalf("execute: %v", err)
	}
	html, err := os.ReadFile(opts.outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	return string(html)
}

// TestExecuteE2E is the tracer bullet: fixture JSONL in, snapshot the HTML out.
func TestExecuteE2E(t *testing.T) {
	html := runFixture(t, options{})
	snaps.MatchSnapshot(t, html)
}

// TestTurnGrouping verifies that user turns are wrapped in sections with id="turn-N".
func TestTurnGrouping(t *testing.T) {
	html := runFixture(t, options{})

	// Fixture has 2 user messages → 2 turns.
	if !strings.Contains(html, `id="turn-1"`) {
		t.Error(`expected id="turn-1" in output`)
	}
	if !strings.Contains(html, `id="turn-2"`) {
		t.Error(`expected id="turn-2" in output`)
	}
	if strings.Contains(html, `id="turn-3"`) {
		t.Error(`unexpected id="turn-3" in output (only 2 user turns in fixture)`)
	}
}

// TestSelfContained verifies no external resource URLs appear in the output.
func TestSelfContained(t *testing.T) {
	html := runFixture(t, options{})
	for _, prefix := range []string{"http://", "https://"} {
		if strings.Contains(html, prefix) {
			t.Errorf("output contains external URL with prefix %q", prefix)
		}
	}
}

// TestTimestamps verifies that the first message shows an absolute timestamp
// and subsequent messages show relative time with an absolute title attribute.
func TestTimestamps(t *testing.T) {
	html := runFixture(t, options{})

	// First message: absolute timestamp visible (no "later" text).
	if !strings.Contains(html, "2025") {
		t.Error("expected year 2025 in first message absolute timestamp")
	}
	// Subsequent messages: relative time with title attribute.
	if !strings.Contains(html, "title=\"") {
		t.Error("expected title= attribute for relative timestamp")
	}
}
