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

	// Fixture has 3 user messages → 3 turns.
	if !strings.Contains(html, `id="turn-1"`) {
		t.Error(`expected id="turn-1" in output`)
	}
	if !strings.Contains(html, `id="turn-2"`) {
		t.Error(`expected id="turn-2" in output`)
	}
	if !strings.Contains(html, `id="turn-3"`) {
		t.Error(`expected id="turn-3" in output`)
	}
	if strings.Contains(html, `id="turn-4"`) {
		t.Error(`unexpected id="turn-4" in output (only 3 user turns in fixture)`)
	}
}

// TestTOC verifies a table of contents is rendered with one entry per user turn.
func TestTOC(t *testing.T) {
	html := runFixture(t, options{})

	// TOC must contain fragment links to each turn.
	if !strings.Contains(html, `href="#turn-1"`) {
		t.Error(`expected href="#turn-1" in TOC`)
	}
	if !strings.Contains(html, `href="#turn-2"`) {
		t.Error(`expected href="#turn-2" in TOC`)
	}
	if !strings.Contains(html, `href="#turn-3"`) {
		t.Error(`expected href="#turn-3" in TOC`)
	}

	// TOC entries must include the first line of the user message (truncated).
	if !strings.Contains(html, "Hello, can you help me write a Go function?") {
		t.Error("expected first user message text in TOC entry")
	}
}

// TestPrevNext verifies prev/next navigation links on turn sections.
func TestPrevNext(t *testing.T) {
	html := runFixture(t, options{})

	// First turn: no prev, has next.
	if strings.Contains(html, `href="#turn-0"`) {
		t.Error(`unexpected href="#turn-0": first turn must have no prev`)
	}
	if !strings.Contains(html, `href="#turn-2"`) {
		t.Error(`expected href="#turn-2" next link on turn 1`)
	}

	// Middle turn (turn 2): has both prev and next.
	if !strings.Contains(html, `href="#turn-1"`) {
		t.Error(`expected href="#turn-1" prev link on turn 2`)
	}
	if !strings.Contains(html, `href="#turn-3"`) {
		t.Error(`expected href="#turn-3" next link on turn 2`)
	}

	// Last turn: has prev, no next.
	if strings.Contains(html, `href="#turn-4"`) {
		t.Error(`unexpected href="#turn-4": last turn must have no next`)
	}
}

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
