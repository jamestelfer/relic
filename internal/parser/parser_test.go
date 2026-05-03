package parser_test

import (
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

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	require.NoError(t, err, "open %s", name)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// TestParseFixture is the tracer bullet: parse the fixture and snapshot the result.
func TestParseFixture(t *testing.T) {
	msgs, _, err := parser.Parse(openFixture(t, "fixture.jsonl"))
	require.NoError(t, err)

	// Only user and assistant records are returned; system/queue-operation are filtered.
	require.Len(t, msgs, 4, "expected 4 messages")
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)
	assert.Equal(t, "claude-opus-4-5", msgs[1].Model)
	assert.NotNil(t, msgs[1].Timestamp)

	snaps.MatchSnapshot(t, msgs)
}

// TestParseTextBlock verifies that assistant text blocks are decoded as TextBlock.
func TestParseTextBlock(t *testing.T) {
	msgs, _, err := parser.Parse(openFixture(t, "fixture.jsonl"))
	require.NoError(t, err)

	// msgs[1] is the first assistant reply — content is [{"type":"text","text":"Sure!..."}]
	require.Len(t, msgs[1].Content, 1)
	tb, ok := msgs[1].Content[0].(*parser.TextBlock)
	require.True(t, ok, "Content[0] should be *parser.TextBlock, got %T", msgs[1].Content[0])
	assert.NotEmpty(t, tb.Text)

	snaps.MatchSnapshot(t, msgs[1].Content)
}

// TestParseStringContent verifies that a plain-string user content becomes a TextBlock.
func TestParseStringContent(t *testing.T) {
	msgs, _, err := parser.Parse(openFixture(t, "fixture.jsonl"))
	require.NoError(t, err)

	// msgs[0] is the first user message — content is a plain string in the fixture.
	require.Len(t, msgs[0].Content, 1)
	tb, ok := msgs[0].Content[0].(*parser.TextBlock)
	require.True(t, ok, "Content[0] should be *parser.TextBlock, got %T", msgs[0].Content[0])
	assert.Equal(t, "Hello, can you help me write a Go function?", tb.Text)
}

// TestParseRawBlock verifies that unknown content block types become RawBlock.
func TestParseRawBlock(t *testing.T) {
	msgs, _, err := parser.Parse(openFixture(t, "unknown_block.jsonl"))
	require.NoError(t, err)

	require.Len(t, msgs, 1)
	require.Len(t, msgs[0].Content, 1)
	rb, ok := msgs[0].Content[0].(*parser.RawBlock)
	require.True(t, ok, "Content[0] should be *parser.RawBlock, got %T", msgs[0].Content[0])
	assert.Equal(t, "custom_widget", rb.RawType)

	snaps.MatchSnapshot(t, msgs[0].Content)
}

// TestParseMalformedLine verifies that a malformed JSONL line produces a ParseError
// with the correct line number while remaining valid messages are still returned.
func TestParseMalformedLine(t *testing.T) {
	msgs, parseErrs, err := parser.Parse(openFixture(t, "malformed.jsonl"))
	require.NoError(t, err, "Parse should not return a terminal error")
	require.Len(t, parseErrs, 1)
	assert.Equal(t, 2, parseErrs[0].Line)
	assert.NotEmpty(t, parseErrs[0].Raw)
	assert.Error(t, parseErrs[0].Err)
	assert.NotEmpty(t, msgs, "valid messages from non-malformed lines should still be returned")

	snaps.MatchSnapshot(t, parseErrs[0].Line)
}

// TestParseToolResultBlock verifies that tool_result content blocks are decoded correctly.
func TestParseToolResultBlock(t *testing.T) {
	msgs, _, err := parser.Parse(openFixture(t, "tool_result.jsonl"))
	require.NoError(t, err)

	require.Len(t, msgs, 1)
	require.Len(t, msgs[0].Content, 1)
	trb, ok := msgs[0].Content[0].(*parser.ToolResultBlock)
	require.True(t, ok, "Content[0] should be *parser.ToolResultBlock, got %T", msgs[0].Content[0])
	assert.Equal(t, "toolu_01", trb.ToolUseID)
	assert.NotEmpty(t, trb.Content)

	snaps.MatchSnapshot(t, msgs[0].Content)
}

// TestParseToolUseBlock verifies that tool_use content blocks are decoded as ToolUseBlock.
func TestParseToolUseBlock(t *testing.T) {
	msgs, _, err := parser.Parse(openFixture(t, "tool_thinking.jsonl"))
	require.NoError(t, err)

	require.Len(t, msgs, 1)
	require.Len(t, msgs[0].Content, 2)

	tub, ok := msgs[0].Content[0].(*parser.ToolUseBlock)
	require.True(t, ok, "Content[0] should be *parser.ToolUseBlock, got %T", msgs[0].Content[0])
	assert.Equal(t, "Bash", tub.Name)

	tb, ok := msgs[0].Content[1].(*parser.ThinkingBlock)
	require.True(t, ok, "Content[1] should be *parser.ThinkingBlock, got %T", msgs[0].Content[1])
	assert.NotEmpty(t, tb.Thinking)

	snaps.MatchSnapshot(t, msgs[0].Content)
}

// TestParseNoContent verifies that a message with empty content array is handled.
func TestParseNoContent(t *testing.T) {
	msgs, _, err := parser.Parse(openFixture(t, "empty_content.jsonl"))
	require.NoError(t, err)

	require.Len(t, msgs, 1)
	assert.Empty(t, msgs[0].Content)
}
