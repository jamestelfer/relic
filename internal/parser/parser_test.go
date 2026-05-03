package parser_test

import (
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

// TestParseFixture is the tracer bullet: parse the fixture and snapshot the result.
func TestParseFixture(t *testing.T) {
	f, err := os.Open("testdata/fixture.jsonl")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	msgs, _, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Only user and assistant records are returned; system/queue-operation are filtered.
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}

	if msgs[0].Role != "user" {
		t.Errorf("msgs[0].Role = %q, want %q", msgs[0].Role, "user")
	}
	if msgs[1].Role != "assistant" {
		t.Errorf("msgs[1].Role = %q, want %q", msgs[1].Role, "assistant")
	}
	if msgs[1].Model != "claude-opus-4-5" {
		t.Errorf("msgs[1].Model = %q, want %q", msgs[1].Model, "claude-opus-4-5")
	}
	if msgs[1].Timestamp == nil {
		t.Error("msgs[1].Timestamp is nil, want non-nil")
	}

	snaps.MatchSnapshot(t, msgs)
}

// TestParseTextBlock verifies that assistant text blocks are decoded as TextBlock.
func TestParseTextBlock(t *testing.T) {
	f, err := os.Open("testdata/fixture.jsonl")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	msgs, _, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// msgs[1] is the first assistant reply — content is [{"type":"text","text":"Sure!..."}]
	if len(msgs[1].Content) != 1 {
		t.Fatalf("msgs[1].Content len = %d, want 1", len(msgs[1].Content))
	}
	tb, ok := msgs[1].Content[0].(*parser.TextBlock)
	if !ok {
		t.Fatalf("msgs[1].Content[0] type = %T, want *parser.TextBlock", msgs[1].Content[0])
	}
	if tb.Text == "" {
		t.Error("TextBlock.Text is empty")
	}

	snaps.MatchSnapshot(t, msgs[1].Content)
}

// TestParseStringContent verifies that a plain-string user content becomes a TextBlock.
func TestParseStringContent(t *testing.T) {
	f, err := os.Open("testdata/fixture.jsonl")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	msgs, _, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// msgs[0] is the first user message — content is a plain string in the fixture.
	if len(msgs[0].Content) != 1 {
		t.Fatalf("msgs[0].Content len = %d, want 1", len(msgs[0].Content))
	}
	tb, ok := msgs[0].Content[0].(*parser.TextBlock)
	if !ok {
		t.Fatalf("msgs[0].Content[0] type = %T, want *parser.TextBlock", msgs[0].Content[0])
	}
	if tb.Text != "Hello, can you help me write a Go function?" {
		t.Errorf("TextBlock.Text = %q, want plain string text", tb.Text)
	}
}

// TestParseRawBlock verifies that unknown content block types become RawBlock.
func TestParseRawBlock(t *testing.T) {
	f, err := os.Open("testdata/unknown_block.jsonl")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	msgs, _, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if len(msgs[0].Content) != 1 {
		t.Fatalf("Content len = %d, want 1", len(msgs[0].Content))
	}
	rb, ok := msgs[0].Content[0].(*parser.RawBlock)
	if !ok {
		t.Fatalf("Content[0] type = %T, want *parser.RawBlock", msgs[0].Content[0])
	}
	if rb.RawType != "custom_widget" {
		t.Errorf("RawBlock.RawType = %q, want %q", rb.RawType, "custom_widget")
	}

	snaps.MatchSnapshot(t, msgs[0].Content)
}

// TestParseMalformedLine verifies that a malformed JSONL line produces a ParseError
// with the correct line number while remaining valid messages are still returned.
func TestParseMalformedLine(t *testing.T) {
	f, err := os.Open("testdata/malformed.jsonl")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	msgs, parseErrs, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("Parse returned terminal error: %v", err)
	}
	if len(parseErrs) != 1 {
		t.Fatalf("expected 1 ParseError, got %d", len(parseErrs))
	}
	if parseErrs[0].Line != 2 {
		t.Errorf("ParseError.Line = %d, want 2", parseErrs[0].Line)
	}
	if len(parseErrs[0].Raw) == 0 {
		t.Error("ParseError.Raw is empty, want the malformed line bytes")
	}
	if parseErrs[0].Err == nil {
		t.Error("ParseError.Err is nil, want underlying error")
	}
	// Valid messages on other lines are still returned.
	if len(msgs) == 0 {
		t.Error("expected at least one valid message from non-malformed lines")
	}

	snaps.MatchSnapshot(t, parseErrs[0].Line)
}

// TestParseToolUseBlock verifies that tool_use content blocks are decoded as ToolUseBlock.
func TestParseToolUseBlock(t *testing.T) {
	f, err := os.Open("testdata/tool_thinking.jsonl")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	msgs, _, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if len(msgs[0].Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(msgs[0].Content))
	}

	tub, ok := msgs[0].Content[0].(*parser.ToolUseBlock)
	if !ok {
		t.Fatalf("Content[0] = %T, want *parser.ToolUseBlock", msgs[0].Content[0])
	}
	if tub.Name != "Bash" {
		t.Errorf("ToolUseBlock.Name = %q, want %q", tub.Name, "Bash")
	}

	tb, ok := msgs[0].Content[1].(*parser.ThinkingBlock)
	if !ok {
		t.Fatalf("Content[1] = %T, want *parser.ThinkingBlock", msgs[0].Content[1])
	}
	if tb.Thinking == "" {
		t.Error("ThinkingBlock.Thinking is empty")
	}

	snaps.MatchSnapshot(t, msgs[0].Content)
}

// TestParseNoContent verifies that a message with empty content array is handled.
func TestParseNoContent(t *testing.T) {
	f, err := os.Open("testdata/empty_content.jsonl")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	msgs, _, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if len(msgs[0].Content) != 0 {
		t.Fatalf("Content len = %d, want 0", len(msgs[0].Content))
	}
}
