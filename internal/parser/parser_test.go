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
	res, _, err := parser.Parse(openFixture(t, "fixture.jsonl"))
	msgs := res.Messages
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
	res, _, err := parser.Parse(openFixture(t, "fixture.jsonl"))
	msgs := res.Messages
	require.NoError(t, err)

	// msgs[1] is the first assistant reply — content is [{"type":"text","text":"Sure!..."}]
	require.Len(t, msgs[1].Content, 1)
	require.IsType(t, (*parser.TextBlock)(nil), msgs[1].Content[0])
	tb := msgs[1].Content[0].(*parser.TextBlock)
	assert.NotEmpty(t, tb.Text)

	snaps.MatchSnapshot(t, msgs[1].Content)
}

// TestParseStringContent verifies that a plain-string user content becomes a TextBlock.
func TestParseStringContent(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "fixture.jsonl"))
	msgs := res.Messages
	require.NoError(t, err)

	// msgs[0] is the first user message — content is a plain string in the fixture.
	require.Len(t, msgs[0].Content, 1)
	require.IsType(t, (*parser.TextBlock)(nil), msgs[0].Content[0])
	tb := msgs[0].Content[0].(*parser.TextBlock)
	assert.Equal(t, "Hello, can you help me write a Go function?", tb.Text)
}

// TestParseRawBlock verifies that unknown content block types become RawBlock.
func TestParseRawBlock(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "unknown_block.jsonl"))
	msgs := res.Messages
	require.NoError(t, err)

	require.Len(t, msgs, 1)
	require.Len(t, msgs[0].Content, 1)
	require.IsType(t, (*parser.RawBlock)(nil), msgs[0].Content[0])
	rb := msgs[0].Content[0].(*parser.RawBlock)
	assert.Equal(t, "custom_widget", rb.RawType)

	snaps.MatchSnapshot(t, msgs[0].Content)
}

// TestParseMalformedLine verifies that a malformed JSONL line produces a ParseError
// with the correct line number while remaining valid messages are still returned.
func TestParseMalformedLine(t *testing.T) {
	res, parseErrs, err := parser.Parse(openFixture(t, "malformed.jsonl"))
	require.NoError(t, err, "Parse should not return a terminal error")
	require.Len(t, parseErrs, 1)
	assert.Equal(t, 2, parseErrs[0].Line)
	assert.NotEmpty(t, parseErrs[0].Raw)
	assert.Error(t, parseErrs[0].Err)
	assert.NotEmpty(t, res.Messages, "valid messages from non-malformed lines should still be returned")

	snaps.MatchSnapshot(t, parseErrs[0].Line)
}

// TestParseToolResultBlock verifies that tool_result content blocks are decoded correctly.
func TestParseToolResultBlock(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "tool_result.jsonl"))
	msgs := res.Messages
	require.NoError(t, err)

	require.Len(t, msgs, 1)
	require.Len(t, msgs[0].Content, 1)
	require.IsType(t, (*parser.ToolResultBlock)(nil), msgs[0].Content[0])
	trb := msgs[0].Content[0].(*parser.ToolResultBlock)
	assert.Equal(t, "toolu_01", trb.ToolUseID)
	assert.NotEmpty(t, trb.Content)

	snaps.MatchSnapshot(t, msgs[0].Content)
}

// TestParseToolUseBlock verifies that tool_use content blocks are decoded as ToolUseBlock.
func TestParseToolUseBlock(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "tool_thinking.jsonl"))
	msgs := res.Messages
	require.NoError(t, err)

	require.Len(t, msgs, 1)
	require.Len(t, msgs[0].Content, 2)

	require.IsType(t, (*parser.ToolUseBlock)(nil), msgs[0].Content[0])
	tub := msgs[0].Content[0].(*parser.ToolUseBlock)
	assert.Equal(t, "Bash", tub.Name)

	require.IsType(t, (*parser.ThinkingBlock)(nil), msgs[0].Content[1])
	tb := msgs[0].Content[1].(*parser.ThinkingBlock)
	assert.NotEmpty(t, tb.Thinking)

	snaps.MatchSnapshot(t, msgs[0].Content)
}

// TestParseToolResult_ArrayContent: tool_result blocks whose "content" field
// is an array of typed items (e.g. [{"type":"text","text":"..."}]) decode to
// a ToolResultBlock with the concatenated text, not a RawBlock. Real Claude
// Code sessions emit array-form content for MCP tool output.
func TestParseToolResult_ArrayContent(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "tool_result_array.jsonl"))
	require.NoError(t, err)

	require.Len(t, res.Messages, 1)
	require.Len(t, res.Messages[0].Content, 1)

	tr, ok := res.Messages[0].Content[0].(*parser.ToolResultBlock)
	require.True(t, ok, "array-form content must still decode to ToolResultBlock, got %T",
		res.Messages[0].Content[0])
	assert.Equal(t, "toolu_42", tr.ToolUseID)
	assert.Contains(t, tr.Content, "first text")
	assert.Contains(t, tr.Content, "second text")
	assert.Contains(t, tr.Content, "mcp__example__do")
}

// TestParseNoContent verifies that a message with empty content array is handled.
func TestParseNoContent(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "empty_content.jsonl"))
	msgs := res.Messages
	require.NoError(t, err)

	require.Len(t, msgs, 1)
	assert.Empty(t, msgs[0].Content)
}

// TestParseCustomTitleSingle verifies that a custom-title top-level record
// is exposed via Result.CustomTitles with Text and 1-indexed LineNum.
func TestParseCustomTitleSingle(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "custom_title_single.jsonl"))
	require.NoError(t, err)

	require.Len(t, res.CustomTitles, 1)
	assert.Equal(t, "phase5 title", res.CustomTitles[0].Text)
	assert.Equal(t, 1, res.CustomTitles[0].LineNum)
	assert.Empty(t, res.Messages, "custom-title is not a message")
}

// TestParseSystemRecords verifies that top-level type:"system" records are
// emitted via Result.SystemRecords carrying Subtype, Content (if present),
// and 1-indexed LineNum in source order.
func TestParseSystemRecords(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "system_records.jsonl"))
	require.NoError(t, err)

	require.Len(t, res.SystemRecords, 3)

	assert.Equal(t, "local_command", res.SystemRecords[0].Subtype)
	assert.Contains(t, res.SystemRecords[0].Content, "<command-name>/focus")
	assert.Equal(t, 2, res.SystemRecords[0].LineNum)

	assert.Equal(t, "turn_duration", res.SystemRecords[1].Subtype)
	assert.Equal(t, 3, res.SystemRecords[1].LineNum)

	assert.Equal(t, "away_summary", res.SystemRecords[2].Subtype)
	assert.Equal(t, "user stepped away", res.SystemRecords[2].Content)
	assert.Equal(t, 4, res.SystemRecords[2].LineNum)

	// The interleaved user message on line 1 comes through unaffected.
	require.Len(t, res.Messages, 1)
	assert.Equal(t, 1, res.Messages[0].LineNum)
}

// TestParseSkippedRecords verifies that top-level records whose "type" is
// not user/assistant/system/custom-title are exposed via Result.SkippedRecords
// with their original Type and 1-indexed LineNum preserved in source order.
func TestParseSkippedRecords(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "skipped_records.jsonl"))
	require.NoError(t, err)

	require.Len(t, res.SkippedRecords, 2)
	assert.Equal(t, "queue-operation", res.SkippedRecords[0].Type)
	assert.Equal(t, 2, res.SkippedRecords[0].LineNum)
	assert.Equal(t, "summary", res.SkippedRecords[1].Type)
	assert.Equal(t, 3, res.SkippedRecords[1].LineNum)

	// Valid messages still flow through.
	require.Len(t, res.Messages, 2)
	assert.Equal(t, 1, res.Messages[0].LineNum)
	assert.Equal(t, 4, res.Messages[1].LineNum)
}

// TestParseSkippedRecords_OmitsKnownBookkeeping: top-level types emitted
// by Claude Code for its own bookkeeping (attachment, permission-mode,
// last-prompt, file-history-snapshot, agent-name) carry no renderable data
// and must not surface as SkippedRecords. The list is discovered from real
// sessions and extended here per the Phase 7 replan trigger in
// docs/session-fidelity.plan.md.
func TestParseSkippedRecords_OmitsKnownBookkeeping(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "bookkeeping.jsonl"))
	require.NoError(t, err)

	// All bookkeeping types silently dropped.
	assert.Empty(t, res.SkippedRecords,
		"known Claude Code bookkeeping types must not appear as unhandled")
	// The user message still comes through.
	require.Len(t, res.Messages, 1)
	assert.Equal(t, "user", res.Messages[0].Role)
}

// TestParseSkippedRecords_OnlyUnknownTypesAreSkipped: known types (user,
// assistant, system, custom-title, bookkeeping) pass through; only truly
// unrecognised types surface as SkippedRecords.
func TestParseSkippedRecords_OnlyUnknownTypesAreSkipped(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "fixture.jsonl"))
	require.NoError(t, err)
	// fixture.jsonl has: system(line 1), user/assistant (2-5), queue-operation (6).
	// queue-operation is the only skipped one.
	require.Len(t, res.SkippedRecords, 1)
	assert.Equal(t, "queue-operation", res.SkippedRecords[0].Type)
	assert.Equal(t, 6, res.SkippedRecords[0].LineNum)
}

// TestParseIsMeta verifies that the top-level "isMeta" boolean is surfaced
// on parser.Message. Default (key absent) is false.
func TestParseIsMeta(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "meta_user.jsonl"))
	require.NoError(t, err)

	require.Len(t, res.Messages, 3)
	assert.False(t, res.Messages[0].IsMeta, "explicit isMeta:false")
	assert.True(t, res.Messages[1].IsMeta, "explicit isMeta:true")
	assert.False(t, res.Messages[2].IsMeta, "missing isMeta key defaults to false")
}

// TestParseCustomTitleMulti verifies that multiple custom-title records are
// emitted in source order with their 1-indexed LineNum values intact.
func TestParseCustomTitleMulti(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "custom_title_multi.jsonl"))
	require.NoError(t, err)

	require.Len(t, res.CustomTitles, 3)
	assert.Equal(t, "first", res.CustomTitles[0].Text)
	assert.Equal(t, 1, res.CustomTitles[0].LineNum)
	assert.Equal(t, "second", res.CustomTitles[1].Text)
	assert.Equal(t, 3, res.CustomTitles[1].LineNum)
	assert.Equal(t, "third", res.CustomTitles[2].Text)
	assert.Equal(t, 4, res.CustomTitles[2].LineNum)

	// The interleaved user message on line 2 comes through unaffected.
	require.Len(t, res.Messages, 1)
	assert.Equal(t, 2, res.Messages[0].LineNum)
}

// TestParseEnvelope_IsSidechain: JSONL line with isSidechain:true populates Envelope.
func TestParseEnvelope_IsSidechain(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "envelope.jsonl"))
	require.NoError(t, err)

	require.Len(t, res.Messages, 4)
	env := res.Messages[0].Envelope
	assert.True(t, env.IsSidechain)
	assert.Equal(t, "1.0.42", env.Version)
	assert.Equal(t, "/home/user/project", env.Cwd)
	assert.Equal(t, "main", env.GitBranch)
	assert.Equal(t, "abc123", env.Slug)
}

// TestParseEnvelope_OtherFlags: isCompactSummary and isApiErrorMessage populate Envelope.
func TestParseEnvelope_OtherFlags(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "envelope.jsonl"))
	require.NoError(t, err)

	require.Len(t, res.Messages, 4)
	assert.True(t, res.Messages[1].Envelope.IsCompactSummary)
	assert.True(t, res.Messages[2].Envelope.IsApiErrorMessage)
}

// TestParseRedactedThinking verifies that {"type":"redacted_thinking","data":"..."}
// content blocks are decoded as RedactedThinkingBlock.
func TestParseRedactedThinking(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "redacted_thinking.jsonl"))
	require.NoError(t, err)

	require.Len(t, res.Messages, 1)
	require.Len(t, res.Messages[0].Content, 2)

	require.IsType(t, (*parser.ThinkingBlock)(nil), res.Messages[0].Content[0])
	require.IsType(t, (*parser.RedactedThinkingBlock)(nil), res.Messages[0].Content[1])
	rb := res.Messages[0].Content[1].(*parser.RedactedThinkingBlock)
	assert.Equal(t, "abc123", rb.Data)
}

// TestParseEnvelope_MissingFields: absent envelope fields produce zero-value, no error.
func TestParseEnvelope_MissingFields(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "envelope.jsonl"))
	require.NoError(t, err)

	// Line 4 has no envelope fields.
	env := res.Messages[3].Envelope
	assert.False(t, env.IsSidechain)
	assert.False(t, env.IsCompactSummary)
	assert.False(t, env.IsApiErrorMessage)
	assert.False(t, env.IsMeta)
	assert.Empty(t, env.Version)
	assert.Empty(t, env.Cwd)
	assert.Empty(t, env.GitBranch)
	assert.Empty(t, env.Slug)
}

// TestParseResultRecords: top-level type:"result" records are emitted via
// Result.ResultRecords with subtype, duration, turns, cost, and error fields.
func TestParseResultRecords(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "result_record.jsonl"))
	require.NoError(t, err)

	// User message still flows through.
	require.Len(t, res.Messages, 1)

	require.Len(t, res.ResultRecords, 2)

	r0 := res.ResultRecords[0]
	assert.Equal(t, "success", r0.Subtype)
	assert.Equal(t, 5000, r0.DurationMS)
	assert.Equal(t, 3, r0.NumTurns)
	assert.InDelta(t, 0.42, r0.CostUSD, 0.001)
	assert.Empty(t, r0.Error)
	assert.Equal(t, 2, r0.LineNum)

	r1 := res.ResultRecords[1]
	assert.Equal(t, "error", r1.Subtype)
	assert.Equal(t, "max turns reached", r1.Error)
	assert.Equal(t, 12000, r1.DurationMS)
	assert.Equal(t, 20, r1.NumTurns)
	assert.InDelta(t, 1.42, r1.CostUSD, 0.001)
	assert.Equal(t, 3, r1.LineNum)

	// Result records must not appear as SkippedRecords.
	assert.Empty(t, res.SkippedRecords)
}

func TestParseImageBlock(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "image_block.jsonl"))
	require.NoError(t, err)

	require.Len(t, res.Messages, 1)
	require.Len(t, res.Messages[0].Content, 1)
	require.IsType(t, (*parser.ImageBlock)(nil), res.Messages[0].Content[0])

	img := res.Messages[0].Content[0].(*parser.ImageBlock)
	assert.Equal(t, "image/png", img.MediaType)
	assert.Equal(t, "iVBORw0KGgo=", img.Base64)
}

// TestParseToolUseResult_Dict: a JSONL record with a dict-type toolUseResult
// exposes the raw data on the Envelope.
func TestParseToolUseResult_Dict(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "tool_use_result.jsonl"))
	require.NoError(t, err)

	// Line 3 is the user message with toolUseResult.
	require.Len(t, res.Messages, 4)
	env := res.Messages[2].Envelope

	require.NotNil(t, env.ToolUseResult, "dict toolUseResult must be extracted")
	m, ok := env.ToolUseResult.(map[string]any)
	require.True(t, ok, "dict toolUseResult should be map[string]any, got %T", env.ToolUseResult)
	assert.Equal(t, "ok  ./...\n", m["stdout"])
	assert.Equal(t, "", m["stderr"])
	assert.Equal(t, false, m["interrupted"])
}

// TestParseToolUseResult_Absent: records without toolUseResult leave Envelope.ToolUseResult nil.
func TestParseToolUseResult_Absent(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "tool_use_result.jsonl"))
	require.NoError(t, err)

	// Lines 1, 2, 4 have no toolUseResult.
	assert.Nil(t, res.Messages[0].Envelope.ToolUseResult)
	assert.Nil(t, res.Messages[1].Envelope.ToolUseResult)
	assert.Nil(t, res.Messages[3].Envelope.ToolUseResult)
}

// TestParseOrigin verifies that the "origin" field is extracted into
// Envelope.Origin with Kind, From, To sub-fields. Records without origin
// produce a zero-valued Origin.
func TestParseOrigin(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "origin.jsonl"))
	require.NoError(t, err)
	require.Len(t, res.Messages, 3)

	tests := []struct {
		name string
		msg  parser.Message
		kind string
		from string
		to   string
	}{
		{
			name: "full origin",
			msg:  res.Messages[0],
			kind: "system-reminder",
			from: "agent-1",
			to:   "agent-2",
		},
		{
			name: "absent origin",
			msg:  res.Messages[1],
			kind: "",
			from: "",
			to:   "",
		},
		{
			name: "partial origin (kind only)",
			msg:  res.Messages[2],
			kind: "task-notification",
			from: "",
			to:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.kind, tt.msg.Envelope.Origin.Kind)
			assert.Equal(t, tt.from, tt.msg.Envelope.Origin.From)
			assert.Equal(t, tt.to, tt.msg.Envelope.Origin.To)
		})
	}
}

// TestParseToolUseResult_StringAndArray: string and array toolUseResult variants
// are extracted as-is (string stays string, array stays []any).
func TestParseToolUseResult_StringAndArray(t *testing.T) {
	res, _, err := parser.Parse(openFixture(t, "tool_use_result_variants.jsonl"))
	require.NoError(t, err)

	require.Len(t, res.Messages, 2)

	// Line 1: string toolUseResult.
	strResult := res.Messages[0].Envelope.ToolUseResult
	require.NotNil(t, strResult)
	s, ok := strResult.(string)
	require.True(t, ok, "string toolUseResult should be string, got %T", strResult)
	assert.Contains(t, s, "Error: Exit code 2")

	// Line 2: array toolUseResult.
	arrResult := res.Messages[1].Envelope.ToolUseResult
	require.NotNil(t, arrResult)
	_, ok = arrResult.([]any)
	require.True(t, ok, "array toolUseResult should be []any, got %T", arrResult)
}
