package session_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jamestelfer/relic/internal/parser"
	"github.com/jamestelfer/relic/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	require.NoError(t, err, "open %s", name)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func parseFixture(t *testing.T, name string) parser.Result {
	t.Helper()
	res, _, err := parser.Parse(openFixture(t, name))
	require.NoError(t, err)
	return res
}

// TestTransformEmpty: empty JSONL → Session with zero turns.
func TestTransformEmpty(t *testing.T) {
	res := parseFixture(t, "empty.jsonl")
	s := session.Transform(res)
	assert.Empty(t, s.Turns)
}

// TestTransformSingleTurn: one user + one assistant message → one turn with
// correct block types in temporal order and Start set from first timestamp.
func TestTransformSingleTurn(t *testing.T) {
	res := parseFixture(t, "single_turn.jsonl")
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	turn := s.Turns[0]
	assert.Equal(t, 1, turn.Index)

	require.Len(t, turn.Blocks, 2)
	require.IsType(t, (*session.UserText)(nil), turn.Blocks[0])
	require.IsType(t, (*session.AssistantText)(nil), turn.Blocks[1])

	assert.Equal(t, "Hello", turn.Blocks[0].(*session.UserText).Text)
	assert.Equal(t, "Hi there!", turn.Blocks[1].(*session.AssistantText).Text)

	require.NotNil(t, s.Start)
	assert.Equal(t, 2025, s.Start.Year())
}

// TestTransformMultiTurn: two user+assistant pairs → two turns, each 1-indexed.
func TestTransformMultiTurn(t *testing.T) {
	res := parseFixture(t, "multi_turn.jsonl")
	s := session.Transform(res)

	require.Len(t, s.Turns, 2)
	assert.Equal(t, 1, s.Turns[0].Index)
	assert.Equal(t, 2, s.Turns[1].Index)

	// Each turn has user + assistant block.
	require.Len(t, s.Turns[0].Blocks, 2)
	require.Len(t, s.Turns[1].Blocks, 2)

	assert.IsType(t, (*session.UserText)(nil), s.Turns[0].Blocks[0])
	assert.IsType(t, (*session.UserText)(nil), s.Turns[1].Blocks[0])
}

// TestTransformToolResultNoNewTurn: a user message with only tool_result blocks
// does not create a new turn boundary; blocks appear in the existing turn.
func TestTransformToolResultNoNewTurn(t *testing.T) {
	res := parseFixture(t, "tool_flow.jsonl")
	s := session.Transform(res)

	// user(text) + assistant(tool_use) + user(tool_result) + assistant(text) → 1 turn
	require.Len(t, s.Turns, 1)
	turn := s.Turns[0]

	// Block order: UserText, ToolCall, ToolResult, AssistantText
	require.Len(t, turn.Blocks, 4)
	assert.IsType(t, (*session.UserText)(nil), turn.Blocks[0])
	assert.IsType(t, (*session.ToolCall)(nil), turn.Blocks[1])
	assert.IsType(t, (*session.ToolResult)(nil), turn.Blocks[2])
	assert.IsType(t, (*session.AssistantText)(nil), turn.Blocks[3])
}

// TestTransformToolPairingMatched: a ToolCall and ToolResult with the same
// tool_use_id get their cross-reference fields set.
func TestTransformToolPairingMatched(t *testing.T) {
	res := parseFixture(t, "tool_flow.jsonl")
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	turn := s.Turns[0]

	call := turn.Blocks[1].(*session.ToolCall)
	result := turn.Blocks[2].(*session.ToolResult)

	require.NotNil(t, call.LinkedResultID, "ToolCall should have LinkedResultID set")
	require.NotNil(t, result.LinkedCallID, "ToolResult should have LinkedCallID set")
	assert.Equal(t, "toolu_01", *call.LinkedResultID)
	assert.Equal(t, "toolu_01", *result.LinkedCallID)
}

// TestTransformToolPairingUnmatched: an unmatched ToolCall has nil LinkedResultID
// and an unmatched ToolResult has nil LinkedCallID.
func TestTransformToolPairingUnmatched(t *testing.T) {
	res := parseFixture(t, "unmatched_tools.jsonl")
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	turn := s.Turns[0]

	var call *session.ToolCall
	var result *session.ToolResult
	for _, b := range turn.Blocks {
		switch v := b.(type) {
		case *session.ToolCall:
			call = v
		case *session.ToolResult:
			result = v
		}
	}

	require.NotNil(t, call)
	require.NotNil(t, result)
	assert.Nil(t, call.LinkedResultID, "unmatched ToolCall should have nil LinkedResultID")
	assert.Nil(t, result.LinkedCallID, "unmatched ToolResult should have nil LinkedCallID")
}

// TestTransformTitleDerivation: titles are derived from the first TextBlock of
// each user message, truncated to 80 chars and stripped after first newline.
func TestTransformTitleDerivation(t *testing.T) {
	res := parseFixture(t, "title_variants.jsonl")
	s := session.Transform(res)

	require.Len(t, s.Turns, 3)

	// Turn 1: plain short string content
	assert.Equal(t, "Short title", s.Turns[0].Title)

	// Turn 2: text longer than 80 chars — truncated to exactly 80 characters
	assert.Len(t, s.Turns[1].Title, 80)
	assert.Equal(t, "A question with a very long title that exceeds the eighty character truncation l", s.Turns[1].Title)

	// Turn 3: multiline text — only first line
	assert.Equal(t, "Multiline title", s.Turns[2].Title)
}

// TestTransformDurationNanos: duration is computed as last message timestamp minus Start.
func TestTransformDurationNanos(t *testing.T) {
	res := parseFixture(t, "tool_flow.jsonl")
	s := session.Transform(res)
	// First timestamp 10:00:01, last 10:00:06 → 5s.
	assert.Equal(t, 5*time.Second, s.DurationNanos)
}

// TestTransformModel: Session.Model is the first non-empty model from messages.
func TestTransformModel(t *testing.T) {
	res := parseFixture(t, "tool_flow.jsonl")
	s := session.Transform(res)
	assert.Equal(t, "claude-opus-4-5", s.Model)
}

// TestToolCallArg_KnownTools: per-tool adapter returns the documented field.
func TestToolCallArg_KnownTools(t *testing.T) {
	cases := []struct {
		name  string
		input map[string]any
		want  string
	}{
		{"Bash", map[string]any{"command": "ls -la", "description": "List files"}, "List files"},
		{"Read", map[string]any{"file_path": "foo.go"}, "foo.go"},
		{"Write", map[string]any{"file_path": "bar.go"}, "bar.go"},
		{"Edit", map[string]any{"file_path": "baz.go", "new_string": "x"}, "baz.go"},
		{"Grep", map[string]any{"pattern": "TODO"}, "TODO"},
		{"WebSearch", map[string]any{"query": "golang jsonv2"}, "golang jsonv2"},
		{"TodoWrite", map[string]any{"todos": []any{1, 2, 3}}, "3 todos"},
		{"AskUserQuestion", map[string]any{"questions": []any{map[string]any{"question": "Should I continue?", "header": "Confirm"}}}, "Should I continue?"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &session.ToolCall{Name: tc.name, Input: tc.input}
			assert.Equal(t, tc.want, c.Arg())
		})
	}
}

// TestToolCallArg_UnknownTool: default adapter picks the alphabetically-first string field.
func TestToolCallArg_UnknownTool(t *testing.T) {
	c := &session.ToolCall{
		Name:  "SomeUnknownTool",
		Input: map[string]any{"zeta": "last", "alpha": "first", "count": 7},
	}
	assert.Equal(t, "first", c.Arg())
}

// TestToolCallArg_UnknownTool_NoString: default adapter returns "" when no string fields.
func TestToolCallArg_UnknownTool_NoString(t *testing.T) {
	c := &session.ToolCall{Name: "X", Input: map[string]any{"n": 42, "ok": true}}
	assert.Equal(t, "", c.Arg())
}

// TestTransformFilesTouched: union of file paths from Read/Write/Edit/MultiEdit tool calls.
func TestTransformFilesTouched(t *testing.T) {
	res := parseFixture(t, "files_touched.jsonl")
	s := session.Transform(res)
	// Three unique paths across Read+Write+Edit+MultiEdit; duplicates coalesced.
	assert.Equal(t, []string{"foo.go", "bar.go", "baz.go"}, s.FilesTouched)
}

// TestTransformEmbeddedName_LastCustomTitleWins: with multiple custom-title
// records, Session.EmbeddedName is the last record's Text.
func TestTransformEmbeddedName_LastCustomTitleWins(t *testing.T) {
	res := parser.Result{
		CustomTitles: []parser.CustomTitleRecord{
			{Text: "first", LineNum: 1},
			{Text: "second", LineNum: 5},
			{Text: "final", LineNum: 9},
		},
	}
	s := session.Transform(res)
	assert.Equal(t, "final", s.EmbeddedName)
}

// TestTransformEmbeddedName_NoCustomTitle: absent custom-title records leave
// EmbeddedName empty (renderer's fallback chain picks filename).
func TestTransformEmbeddedName_NoCustomTitle(t *testing.T) {
	s := session.Transform(parser.Result{})
	assert.Empty(t, s.EmbeddedName)
}

// TestTransformSystemRecords_RoutingAndOrdering: system records route by
// subtype (local_command → LocalCommand, turn_duration → omitted,
// else → System) and interleave with messages in source-line order.
func TestTransformSystemRecords_RoutingAndOrdering(t *testing.T) {
	res := parseFixture(t, "system_interleaved.jsonl")
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	blocks := s.Turns[0].Blocks

	// Expected order across the single turn:
	// 1 user → UserText
	// 2 local_command → LocalCommand (Name=focus)
	// 3 assistant → AssistantText
	// 4 turn_duration → omitted
	// 5 away_summary → System
	// 6 local_command (no XML) → LocalCommand (Name="", Message=raw)
	require.Len(t, blocks, 5, "turn_duration must not produce a block")

	assert.IsType(t, (*session.UserText)(nil), blocks[0])

	lc1, ok := blocks[1].(*session.LocalCommand)
	require.True(t, ok, "blocks[1] must be *LocalCommand, got %T", blocks[1])
	assert.Equal(t, "focus", lc1.Name, "leading '/' stripped from command name")
	assert.Equal(t, "focus mode", lc1.Message)
	assert.Equal(t, "all", lc1.Args)
	assert.Equal(t, 2, lc1.LineNum)

	assert.IsType(t, (*session.AssistantText)(nil), blocks[2])

	sys, ok := blocks[3].(*session.System)
	require.True(t, ok, "blocks[3] must be *System, got %T", blocks[3])
	assert.Equal(t, "away_summary", sys.Subtype)
	assert.Equal(t, "system_record", sys.Source)
	assert.Contains(t, sys.Content, "<system-reminder>")
	assert.Equal(t, 5, sys.LineNum)

	lc2, ok := blocks[4].(*session.LocalCommand)
	require.True(t, ok, "blocks[4] must be *LocalCommand, got %T", blocks[4])
	assert.Empty(t, lc2.Name, "malformed content yields empty Name")
	assert.Empty(t, lc2.Args, "malformed content yields empty Args")
	assert.Equal(t, "no xml here, just text", lc2.Message)
	assert.Equal(t, 6, lc2.LineNum)
}

// TestTransformMetaUser_ProducesHookInjection: a user message with IsMeta:true
// (no special flags) becomes a *HookInjection block, not a *UserText.
func TestTransformMetaUser_ProducesHookInjection(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	res := parser.Result{
		Messages: []parser.Message{
			{
				Role: "user", Timestamp: &ts, LineNum: 1,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "go"}},
			},
			{
				Role: "user", Timestamp: &ts, LineNum: 2, IsMeta: true,
				Envelope: parser.Envelope{IsMeta: true},
				Content:  []parser.ContentBlock{&parser.TextBlock{Text: "<system-reminder>wake up</system-reminder>"}},
			},
			{
				Role: "assistant", Timestamp: &ts, LineNum: 3,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "ok"}},
			},
		},
	}
	s := session.Transform(res)

	require.Len(t, s.Turns, 1, "meta-user must not open a new turn")
	blocks := s.Turns[0].Blocks
	require.Len(t, blocks, 3)

	hi, ok := blocks[1].(*session.HookInjection)
	require.True(t, ok, "blocks[1] must be *HookInjection, got %T", blocks[1])
	assert.Equal(t, "<system-reminder>wake up</system-reminder>", hi.Content)
	assert.Equal(t, 2, hi.LineNum)

	assert.IsType(t, (*session.UserText)(nil), blocks[0])
	assert.IsType(t, (*session.AssistantText)(nil), blocks[2])
}

// TestTransformMetaUser_MultipleTextBlocksConcatenated: a meta user message
// with multiple text blocks surfaces a single HookInjection block whose Content
// joins the text blocks with a blank-line separator (preserving paragraph breaks).
func TestTransformMetaUser_MultipleTextBlocksConcatenated(t *testing.T) {
	res := parser.Result{
		Messages: []parser.Message{
			{Role: "user", LineNum: 1, Content: []parser.ContentBlock{&parser.TextBlock{Text: "open"}}},
			{
				Role: "user", LineNum: 2, IsMeta: true,
				Envelope: parser.Envelope{IsMeta: true},
				Content: []parser.ContentBlock{
					&parser.TextBlock{Text: "first"},
					&parser.TextBlock{Text: "second"},
				},
			},
		},
	}
	s := session.Transform(res)
	require.Len(t, s.Turns, 1)
	require.Len(t, s.Turns[0].Blocks, 2)

	hi, ok := s.Turns[0].Blocks[1].(*session.HookInjection)
	require.True(t, ok)
	assert.Equal(t, "first\n\nsecond", hi.Content)
}

// TestTransformMetaUser_NoNewTurn_HasTextBlockGating: a meta user with a
// TextBlock must NOT flip the turn boundary. Two non-meta user messages plus
// a meta-user between them → two turns, not three.
func TestTransformMetaUser_NoNewTurn_HasTextBlockGating(t *testing.T) {
	res := parser.Result{
		Messages: []parser.Message{
			{Role: "user", LineNum: 1, Content: []parser.ContentBlock{&parser.TextBlock{Text: "first"}}},
			{Role: "user", LineNum: 2, IsMeta: true, Envelope: parser.Envelope{IsMeta: true}, Content: []parser.ContentBlock{&parser.TextBlock{Text: "meta"}}},
			{Role: "assistant", LineNum: 3, Content: []parser.ContentBlock{&parser.TextBlock{Text: "a1"}}},
			{Role: "user", LineNum: 4, Content: []parser.ContentBlock{&parser.TextBlock{Text: "second"}}},
			{Role: "assistant", LineNum: 5, Content: []parser.ContentBlock{&parser.TextBlock{Text: "a2"}}},
		},
	}
	s := session.Transform(res)
	require.Len(t, s.Turns, 2, "meta user must not open a turn; expect 2 turns from non-meta user messages")
	assert.Equal(t, 1, s.Turns[0].Index)
	assert.Equal(t, 2, s.Turns[1].Index)
	// Turn 1 holds: user, meta-hook, assistant.
	require.Len(t, s.Turns[0].Blocks, 3)
	_, ok := s.Turns[0].Blocks[1].(*session.HookInjection)
	assert.True(t, ok)
}

// TestTransformBlocksCarryLineNum: every session.Block produced from parsed
// content carries the 1-indexed JSONL line number of the record it came from.
// Exercises every block type that exists today: UserText, AssistantText,
// ToolCall, ToolResult, Thinking, Error, Raw.
func TestTransformBlocksCarryLineNum(t *testing.T) {
	f := openFixture(t, "line_nums.jsonl")
	res, _, err := parser.Parse(f)
	require.NoError(t, err)

	s := session.Transform(res)

	// Collect (type, lineNum) for every block across all turns.
	lineNumOf := func(b session.Block) int {
		switch v := b.(type) {
		case *session.UserText:
			return v.LineNum
		case *session.AssistantText:
			return v.LineNum
		case *session.ToolCall:
			return v.LineNum
		case *session.ToolResult:
			return v.LineNum
		case *session.Thinking:
			return v.LineNum
		case *session.Error:
			return v.LineNum
		case *session.Raw:
			return v.LineNum
		}
		t.Fatalf("unexpected block type %T", b)
		return 0
	}

	byType := map[string]int{}
	for _, turn := range s.Turns {
		for _, b := range turn.Blocks {
			ln := lineNumOf(b)
			assert.NotZero(t, ln, "block %T should have non-zero LineNum", b)
			byType[b.BlockType()] = ln
		}
	}
	assert.Equal(t, 1, byType["user_text"])
	assert.Equal(t, 2, byType["assistant_text"])
	assert.Equal(t, 2, byType["tool_call"])
	assert.Equal(t, 2, byType["thinking"])
	assert.Equal(t, 2, byType["custom_widget"]) // Raw
	assert.Equal(t, 3, byType["tool_result"])
	assert.Equal(t, 4, byType["error"])
}

// TestTransformSidechainExcluded: records with IsSidechain:true in their
// Envelope are excluded from output turns entirely.
func TestTransformSidechainExcluded(t *testing.T) {
	res := parseFixture(t, "sidechain.jsonl")
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	// The sidechain assistant reply must not appear. Only user + visible reply.
	require.Len(t, s.Turns[0].Blocks, 2)
	assert.IsType(t, (*session.UserText)(nil), s.Turns[0].Blocks[0])
	assert.IsType(t, (*session.AssistantText)(nil), s.Turns[0].Blocks[1])
	assert.Equal(t, "visible reply", s.Turns[0].Blocks[1].(*session.AssistantText).Text)
}

// TestTransformSessionHeader: SessionHeader populated with model, start time, turn count.
func TestTransformSessionHeader(t *testing.T) {
	res := parseFixture(t, "single_turn.jsonl")
	s := session.Transform(res)

	hdr := s.Header()
	assert.Equal(t, "claude-opus-4-5", hdr.Model)
	assert.NotNil(t, hdr.StartTime)
	assert.Equal(t, 1, hdr.TurnCount)
}

// TestTransformTitleAbsentErrorPreamble: an error pseudo-message before any
// user turn is folded into the first TextBlock turn; the turn title comes from
// the user text, not the error.
func TestTransformTitleAbsentErrorPreamble(t *testing.T) {
	// error_preamble.jsonl has a malformed line first, then a user message.
	f := openFixture(t, "error_preamble.jsonl")
	res, parseErrs, err := parser.Parse(f)
	require.NoError(t, err)
	require.Len(t, parseErrs, 1, "expected one parse error from the malformed line")

	s := session.Transform(res)

	// The error is folded into the first (and only) turn.
	require.Len(t, s.Turns, 1)
	assert.Equal(t, "After error", s.Turns[0].Title)

	// Error block is present in the turn's blocks.
	var hasError bool
	for _, b := range s.Turns[0].Blocks {
		if _, ok := b.(*session.Error); ok {
			hasError = true
		}
	}
	assert.True(t, hasError, "error block should be present in turn")
}

// TestTransformUserCommand: user isMeta message with <command-name> → LocalCommand block.
func TestTransformUserCommand(t *testing.T) {
	res := parseFixture(t, "user_command.jsonl")
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	blocks := s.Turns[0].Blocks
	// user text + local command + command output
	require.GreaterOrEqual(t, len(blocks), 3)
	require.IsType(t, (*session.UserText)(nil), blocks[0])
	require.IsType(t, (*session.LocalCommand)(nil), blocks[1])
	lc := blocks[1].(*session.LocalCommand)
	assert.Equal(t, "compact", lc.Name)
	assert.Equal(t, "compacted", lc.Message)
}

// TestTransformUserCommandOutput: user isMeta message with <local-command-stdout> → System with stdout.
func TestTransformUserCommandOutput(t *testing.T) {
	res := parseFixture(t, "user_command.jsonl")
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	blocks := s.Turns[0].Blocks
	require.GreaterOrEqual(t, len(blocks), 3)
	// The stdout block should be a System or LocalCommand with content
	found := false
	for _, b := range blocks {
		if sys, ok := b.(*session.System); ok && strings.Contains(sys.Content, "Authentication successful") {
			found = true
			break
		}
		if lc, ok := b.(*session.LocalCommand); ok && strings.Contains(lc.Message, "Authentication successful") {
			found = true
			break
		}
	}
	assert.True(t, found, "command stdout should be present in some block")
}

// TestTransformApiError: user with isApiErrorMessage:true → ApiError block (not UserText).
func TestTransformApiError(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	res := parser.Result{
		Messages: []parser.Message{
			{
				Role: "user", Timestamp: &ts, LineNum: 1,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "start"}},
			},
			{
				Role: "user", Timestamp: &ts, LineNum: 2,
				IsMeta: true,
				Envelope: parser.Envelope{
					IsApiErrorMessage: true,
					IsMeta:            true,
				},
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "rate_limit: Rate limit exceeded"}},
			},
		},
	}
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	blocks := s.Turns[0].Blocks

	var found *session.ApiError
	for _, b := range blocks {
		if ae, ok := b.(*session.ApiError); ok {
			found = ae
			break
		}
	}
	require.NotNil(t, found, "expected an ApiError block")
	assert.Contains(t, found.Message, "Rate limit exceeded")
	assert.Equal(t, 2, found.LineNum)
}

// TestTransformHookInjection: user isMeta without special flags or patterns → HookInjection.
func TestTransformHookInjection(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	res := parser.Result{
		Messages: []parser.Message{
			{
				Role: "user", Timestamp: &ts, LineNum: 1,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "start"}},
			},
			{
				Role: "user", Timestamp: &ts, LineNum: 2,
				IsMeta:   true,
				Envelope: parser.Envelope{IsMeta: true},
				Content:  []parser.ContentBlock{&parser.TextBlock{Text: "<user-prompt-submit-hook>PreToolUse · Bash · allowed</user-prompt-submit-hook>"}},
			},
		},
	}
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	blocks := s.Turns[0].Blocks
	require.Len(t, blocks, 2)

	hi, ok := blocks[1].(*session.HookInjection)
	require.True(t, ok, "expected HookInjection, got %T", blocks[1])
	assert.Contains(t, hi.Content, "PreToolUse")
	assert.Equal(t, 2, hi.LineNum)
}

// TestTransformLocalCommandCaveat_Filtered: user isMeta with <local-command-caveat>
// produces no block (filtered from output per R6 cascade).
func TestTransformLocalCommandCaveat_Filtered(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	res := parser.Result{
		Messages: []parser.Message{
			{
				Role: "user", Timestamp: &ts, LineNum: 1,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "start"}},
			},
			{
				Role: "user", Timestamp: &ts, LineNum: 2,
				IsMeta:   true,
				Envelope: parser.Envelope{IsMeta: true},
				Content: []parser.ContentBlock{&parser.TextBlock{
					Text: "<local-command-caveat>Caveat: The messages below were generated by the user while running local commands.</local-command-caveat>",
				}},
			},
			{
				Role: "assistant", Timestamp: &ts, LineNum: 3,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "ok"}},
			},
		},
	}
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	blocks := s.Turns[0].Blocks
	// Should have only user + assistant (caveat filtered).
	require.Len(t, blocks, 2)
	assert.IsType(t, (*session.UserText)(nil), blocks[0])
	assert.IsType(t, (*session.AssistantText)(nil), blocks[1])
}

// TestTransformCompactionSummary: user with isCompactSummary:true → CompactionSummary block.
func TestTransformCompactionSummary(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	res := parser.Result{
		Messages: []parser.Message{
			{
				Role: "user", Timestamp: &ts, LineNum: 1,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "start"}},
			},
			{
				Role: "user", Timestamp: &ts, LineNum: 2,
				IsMeta: true,
				Envelope: parser.Envelope{
					IsCompactSummary: true,
					IsMeta:           true,
				},
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "Summary of previous context"}},
			},
		},
	}
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	blocks := s.Turns[0].Blocks

	var found *session.CompactionSummary
	for _, b := range blocks {
		if cs, ok := b.(*session.CompactionSummary); ok {
			found = cs
			break
		}
	}
	require.NotNil(t, found, "expected a CompactionSummary block")
	assert.Equal(t, "Summary of previous context", found.Content)
	assert.Equal(t, 2, found.LineNum)
}

// TestTransformSessionResult: ResultRecords produce SessionResult blocks in
// the appropriate position (after all messages, at the end of the last turn).
func TestTransformSessionResult(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	res := parser.Result{
		Messages: []parser.Message{
			{
				Role: "user", Timestamp: &ts, LineNum: 1,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "hello"}},
			},
			{
				Role: "assistant", Timestamp: &ts, LineNum: 2,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "hi"}},
			},
		},
		ResultRecords: []parser.ResultRecord{
			{Subtype: "success", DurationMS: 5000, NumTurns: 1, CostUSD: 0.42, LineNum: 3},
		},
	}
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	blocks := s.Turns[0].Blocks
	// Last block should be SessionResult.
	last := blocks[len(blocks)-1]
	sr, ok := last.(*session.SessionResult)
	require.True(t, ok, "last block should be *SessionResult, got %T", last)
	assert.Equal(t, "success", sr.Subtype)
	assert.Equal(t, 5000, sr.DurationMS)
	assert.Equal(t, 1, sr.NumTurns)
	assert.InDelta(t, 0.42, sr.CostUSD, 0.001)
	assert.Equal(t, 3, sr.LineNum)
}

// TestTransformCompactBoundary: system record with subtype "compact_boundary"
// produces a *CompactionBoundary block with the label content.
func TestTransformCompactBoundary(t *testing.T) {
	res := parseFixture(t, "compact_boundary.jsonl")
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	blocks := s.Turns[0].Blocks

	var found *session.CompactionBoundary
	for _, b := range blocks {
		if cb, ok := b.(*session.CompactionBoundary); ok {
			found = cb
			break
		}
	}
	require.NotNil(t, found, "expected a CompactionBoundary block")
	assert.Equal(t, "Context compacted · auto · 42k → 18k tokens", found.Label)
	assert.Equal(t, 2, found.LineNum)
}

// TestTransformUserBash: user ! command messages are classified as UserBashInput
// and UserBashResult blocks.
func TestTransformUserBash(t *testing.T) {
	res := parseFixture(t, "slash_commands.jsonl")
	s := session.Transform(res)

	require.NotEmpty(t, s.Turns)
	turn := s.Turns[0]

	var inputs []*session.UserBashInput
	var results []*session.UserBashResult
	for _, b := range turn.Blocks {
		switch v := b.(type) {
		case *session.UserBashInput:
			inputs = append(inputs, v)
		case *session.UserBashResult:
			results = append(results, v)
		}
	}

	require.Len(t, inputs, 2, "should detect 2 UserBashInput blocks")
	assert.Equal(t, "echo hello world", inputs[0].Command)
	assert.Equal(t, 4, inputs[0].LineNum)
	assert.Equal(t, "ls -la", inputs[1].Command)
	assert.Equal(t, 8, inputs[1].LineNum)

	require.Len(t, results, 2, "should detect 2 UserBashResult blocks")
	assert.Equal(t, "hello world", results[0].Stdout)
	assert.Equal(t, "", results[0].Stderr)
	assert.Equal(t, 5, results[0].LineNum)
	assert.Contains(t, results[1].Stdout, "total 8")
	assert.Equal(t, "ls: warning: test", results[1].Stderr)
	assert.Equal(t, 9, results[1].LineNum)

	// Pair links: each input links to its adjacent result and vice versa.
	require.NotNil(t, inputs[0].LinkedResultID)
	assert.Equal(t, results[0].ID, *inputs[0].LinkedResultID)
	require.NotNil(t, results[0].LinkedCallID)
	assert.Equal(t, inputs[0].ID, *results[0].LinkedCallID)

	require.NotNil(t, inputs[1].LinkedResultID)
	assert.Equal(t, results[1].ID, *inputs[1].LinkedResultID)
	require.NotNil(t, results[1].LinkedCallID)
	assert.Equal(t, inputs[1].ID, *results[1].LinkedCallID)
}

// TestTransformBashEnrichment: a Bash ToolResult with dict toolUseResult
// produces BashEnrichment with stdout, stderr, and interrupted fields.
func TestTransformBashEnrichment(t *testing.T) {
	res := parseFixture(t, "bash_enrichment.jsonl")
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	turn := s.Turns[0]

	var result *session.ToolResult
	for _, b := range turn.Blocks {
		if tr, ok := b.(*session.ToolResult); ok {
			result = tr
			break
		}
	}
	require.NotNil(t, result)

	require.NotNil(t, result.Enrichment, "ToolResult should have enrichment")
	bash, ok := result.Enrichment.(*session.BashEnrichment)
	require.True(t, ok, "enrichment should be *BashEnrichment, got %T", result.Enrichment)
	assert.Equal(t, "ok  ./...\n", bash.Stdout)
	assert.Equal(t, "warning: deprecated\n", bash.Stderr)
	assert.False(t, bash.Interrupted)
}

// TestTransformBashEnrichment_Interrupted: interrupted Bash execution produces
// enrichment with Interrupted=true.
func TestTransformBashEnrichment_Interrupted(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	res := parser.Result{
		Messages: []parser.Message{
			{Role: "user", Timestamp: &ts, LineNum: 1,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "go"}}},
			{Role: "assistant", Timestamp: &ts, LineNum: 2,
				Content: []parser.ContentBlock{&parser.ToolUseBlock{ID: "toolu_01", Name: "Bash", Input: map[string]any{"command": "sleep 999"}}}},
			{Role: "user", Timestamp: &ts, LineNum: 3,
				Envelope: parser.Envelope{ToolUseResult: map[string]any{"stdout": "partial", "stderr": "", "interrupted": true}},
				Content:  []parser.ContentBlock{&parser.ToolResultBlock{ToolUseID: "toolu_01", Content: "partial"}}},
		},
	}
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	var result *session.ToolResult
	for _, b := range s.Turns[0].Blocks {
		if tr, ok := b.(*session.ToolResult); ok {
			result = tr
			break
		}
	}
	require.NotNil(t, result)
	require.NotNil(t, result.Enrichment)
	bash, ok := result.Enrichment.(*session.BashEnrichment)
	require.True(t, ok)
	assert.True(t, bash.Interrupted)
	assert.Equal(t, "partial", bash.Stdout)
}

// TestTransformEnrichment_NilCases: enrichment is nil for absent toolUseResult,
// string toolUseResult, array toolUseResult, and unpaired ToolResult.
func TestTransformEnrichment_NilCases(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	t.Run("absent toolUseResult", func(t *testing.T) {
		res := parser.Result{
			Messages: []parser.Message{
				{Role: "user", Timestamp: &ts, LineNum: 1,
					Content: []parser.ContentBlock{&parser.TextBlock{Text: "go"}}},
				{Role: "assistant", Timestamp: &ts, LineNum: 2,
					Content: []parser.ContentBlock{&parser.ToolUseBlock{ID: "toolu_01", Name: "Bash", Input: map[string]any{}}}},
				{Role: "user", Timestamp: &ts, LineNum: 3,
					Content: []parser.ContentBlock{&parser.ToolResultBlock{ToolUseID: "toolu_01", Content: "ok"}}},
			},
		}
		s := session.Transform(res)
		for _, b := range s.Turns[0].Blocks {
			if tr, ok := b.(*session.ToolResult); ok {
				assert.Nil(t, tr.Enrichment, "absent toolUseResult → nil enrichment")
			}
		}
	})

	t.Run("string toolUseResult", func(t *testing.T) {
		res := parser.Result{
			Messages: []parser.Message{
				{Role: "user", Timestamp: &ts, LineNum: 1,
					Content: []parser.ContentBlock{&parser.TextBlock{Text: "go"}}},
				{Role: "assistant", Timestamp: &ts, LineNum: 2,
					Content: []parser.ContentBlock{&parser.ToolUseBlock{ID: "toolu_01", Name: "Bash", Input: map[string]any{}}}},
				{Role: "user", Timestamp: &ts, LineNum: 3,
					Envelope: parser.Envelope{ToolUseResult: "Error: Exit code 1"},
					Content:  []parser.ContentBlock{&parser.ToolResultBlock{ToolUseID: "toolu_01", Content: "Error"}}},
			},
		}
		s := session.Transform(res)
		for _, b := range s.Turns[0].Blocks {
			if tr, ok := b.(*session.ToolResult); ok {
				assert.Nil(t, tr.Enrichment, "string toolUseResult → nil enrichment")
			}
		}
	})

	t.Run("unpaired ToolResult", func(t *testing.T) {
		res := parser.Result{
			Messages: []parser.Message{
				{Role: "user", Timestamp: &ts, LineNum: 1,
					Content: []parser.ContentBlock{&parser.TextBlock{Text: "go"}}},
				{Role: "user", Timestamp: &ts, LineNum: 2,
					Envelope: parser.Envelope{ToolUseResult: map[string]any{"stdout": "x", "stderr": "", "interrupted": false}},
					Content:  []parser.ContentBlock{&parser.ToolResultBlock{ToolUseID: "toolu_orphan", Content: "x"}}},
			},
		}
		s := session.Transform(res)
		for _, b := range s.Turns[0].Blocks {
			if tr, ok := b.(*session.ToolResult); ok {
				assert.Nil(t, tr.Enrichment, "unpaired ToolResult → nil enrichment")
			}
		}
	})
}

// TestTransformReadEnrichment: a Read ToolResult with dict toolUseResult
// produces ReadEnrichment with file path from result and line range from
// ToolCall input (offset/limit).
func TestTransformReadEnrichment(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	res := parser.Result{
		Messages: []parser.Message{
			{Role: "user", Timestamp: &ts, LineNum: 1,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "show me"}}},
			{Role: "assistant", Timestamp: &ts, LineNum: 2,
				Content: []parser.ContentBlock{&parser.ToolUseBlock{ID: "toolu_01", Name: "Read", Input: map[string]any{"file_path": "/src/foo.go", "offset": float64(10), "limit": float64(50)}}}},
			{Role: "user", Timestamp: &ts, LineNum: 3,
				Envelope: parser.Envelope{ToolUseResult: map[string]any{
					"type": "text",
					"file": map[string]any{
						"filePath": "/src/foo.go",
						"content":  "package main\n",
					},
				}},
				Content: []parser.ContentBlock{&parser.ToolResultBlock{ToolUseID: "toolu_01", Content: "package main\n"}}},
		},
	}
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	var result *session.ToolResult
	for _, b := range s.Turns[0].Blocks {
		if tr, ok := b.(*session.ToolResult); ok {
			result = tr
			break
		}
	}
	require.NotNil(t, result)
	require.NotNil(t, result.Enrichment)
	read, ok := result.Enrichment.(*session.ReadEnrichment)
	require.True(t, ok, "enrichment should be *ReadEnrichment, got %T", result.Enrichment)
	assert.Equal(t, "/src/foo.go", read.FilePath)
	assert.Equal(t, 10, read.LineStart, "LineStart from ToolCall input offset")
	assert.Equal(t, 59, read.LineEnd, "LineEnd from ToolCall input offset+limit-1")
}

// TestTransformReadEnrichment_NoLineRange: Read without offset/limit in ToolCall
// input produces enrichment with file path but no line range.
func TestTransformReadEnrichment_NoLineRange(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	res := parser.Result{
		Messages: []parser.Message{
			{Role: "user", Timestamp: &ts, LineNum: 1,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "show me"}}},
			{Role: "assistant", Timestamp: &ts, LineNum: 2,
				Content: []parser.ContentBlock{&parser.ToolUseBlock{ID: "toolu_01", Name: "Read", Input: map[string]any{"file_path": "/src/foo.go"}}}},
			{Role: "user", Timestamp: &ts, LineNum: 3,
				Envelope: parser.Envelope{ToolUseResult: map[string]any{
					"type": "text",
					"file": map[string]any{
						"filePath": "/src/foo.go",
						"content":  "full file",
					},
				}},
				Content: []parser.ContentBlock{&parser.ToolResultBlock{ToolUseID: "toolu_01", Content: "full file"}}},
		},
	}
	s := session.Transform(res)

	var result *session.ToolResult
	for _, b := range s.Turns[0].Blocks {
		if tr, ok := b.(*session.ToolResult); ok {
			result = tr
		}
	}
	require.NotNil(t, result)
	require.NotNil(t, result.Enrichment)
	read, ok := result.Enrichment.(*session.ReadEnrichment)
	require.True(t, ok)
	assert.Equal(t, "/src/foo.go", read.FilePath)
	assert.Equal(t, 0, read.LineStart)
	assert.Equal(t, 0, read.LineEnd)
}

// TestTransformWriteEnrichment: Write tool result produces WriteEnrichment
// with file path and action (created/updated).
func TestTransformWriteEnrichment(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		writeType  string
		wantAction string
	}{
		{"create", "create", "created"},
		{"update", "update", "updated"},
		{"unknown type", "overwrite", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := parser.Result{
				Messages: []parser.Message{
					{Role: "user", Timestamp: &ts, LineNum: 1,
						Content: []parser.ContentBlock{&parser.TextBlock{Text: "go"}}},
					{Role: "assistant", Timestamp: &ts, LineNum: 2,
						Content: []parser.ContentBlock{&parser.ToolUseBlock{ID: "toolu_01", Name: "Write", Input: map[string]any{"file_path": "/src/foo.go"}}}},
					{Role: "user", Timestamp: &ts, LineNum: 3,
						Envelope: parser.Envelope{ToolUseResult: map[string]any{
							"type":     tc.writeType,
							"filePath": "/src/foo.go",
							"content":  "package main",
						}},
						Content: []parser.ContentBlock{&parser.ToolResultBlock{ToolUseID: "toolu_01", Content: "ok"}}},
				},
			}
			s := session.Transform(res)
			var result *session.ToolResult
			for _, b := range s.Turns[0].Blocks {
				if tr, ok := b.(*session.ToolResult); ok {
					result = tr
				}
			}
			require.NotNil(t, result)
			require.NotNil(t, result.Enrichment)
			w, ok := result.Enrichment.(*session.WriteEnrichment)
			require.True(t, ok, "expected *WriteEnrichment, got %T", result.Enrichment)
			assert.Equal(t, "/src/foo.go", w.FilePath)
			assert.Equal(t, tc.wantAction, w.Action)
		})
	}
}

// TestTransformEditEnrichment: Edit tool result produces EditEnrichment
// with file path (action is always "updated").
func TestTransformEditEnrichment(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	res := parser.Result{
		Messages: []parser.Message{
			{Role: "user", Timestamp: &ts, LineNum: 1,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "go"}}},
			{Role: "assistant", Timestamp: &ts, LineNum: 2,
				Content: []parser.ContentBlock{&parser.ToolUseBlock{ID: "toolu_01", Name: "Edit", Input: map[string]any{"file_path": "/src/bar.go"}}}},
			{Role: "user", Timestamp: &ts, LineNum: 3,
				Envelope: parser.Envelope{ToolUseResult: map[string]any{
					"filePath":  "/src/bar.go",
					"oldString": "old",
					"newString": "new",
				}},
				Content: []parser.ContentBlock{&parser.ToolResultBlock{ToolUseID: "toolu_01", Content: "ok"}}},
		},
	}
	s := session.Transform(res)
	var result *session.ToolResult
	for _, b := range s.Turns[0].Blocks {
		if tr, ok := b.(*session.ToolResult); ok {
			result = tr
		}
	}
	require.NotNil(t, result)
	require.NotNil(t, result.Enrichment)
	e, ok := result.Enrichment.(*session.EditEnrichment)
	require.True(t, ok, "expected *EditEnrichment, got %T", result.Enrichment)
	assert.Equal(t, "/src/bar.go", e.FilePath)
	assert.Equal(t, "updated", e.Action)
}

// TestTransformGrepEnrichment: Grep tool result produces GrepEnrichment with
// mode, file count, and line count, varying by grep mode.
func TestTransformGrepEnrichment(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name           string
		toolResult     map[string]any
		wantMode       string
		wantNumFiles   int
		wantNumLines   int
		wantNumMatches int
	}{
		{
			name: "content mode",
			toolResult: map[string]any{
				"mode":      "content",
				"numFiles":  float64(3),
				"filenames": []any{},
				"content":   "file.go:42:matching line",
				"numLines":  float64(25),
			},
			wantMode: "content", wantNumFiles: 3, wantNumLines: 25,
		},
		{
			name: "files_with_matches mode",
			toolResult: map[string]any{
				"mode":      "files_with_matches",
				"numFiles":  float64(7),
				"filenames": []any{"a.go", "b.go"},
			},
			wantMode: "files_with_matches", wantNumFiles: 7,
		},
		{
			name: "count mode",
			toolResult: map[string]any{
				"mode":       "count",
				"numFiles":   float64(0),
				"filenames":  []any{},
				"content":    "25",
				"numMatches": float64(25),
			},
			wantMode: "count", wantNumMatches: 25,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := parser.Result{
				Messages: []parser.Message{
					{Role: "user", Timestamp: &ts, LineNum: 1,
						Content: []parser.ContentBlock{&parser.TextBlock{Text: "find it"}}},
					{Role: "assistant", Timestamp: &ts, LineNum: 2,
						Content: []parser.ContentBlock{&parser.ToolUseBlock{ID: "toolu_01", Name: "Grep", Input: map[string]any{"pattern": "foo"}}}},
					{Role: "user", Timestamp: &ts, LineNum: 3,
						Envelope: parser.Envelope{ToolUseResult: tc.toolResult},
						Content:  []parser.ContentBlock{&parser.ToolResultBlock{ToolUseID: "toolu_01", Content: "results"}}},
				},
			}
			s := session.Transform(res)
			var result *session.ToolResult
			for _, b := range s.Turns[0].Blocks {
				if tr, ok := b.(*session.ToolResult); ok {
					result = tr
				}
			}
			require.NotNil(t, result)
			require.NotNil(t, result.Enrichment)
			g, ok := result.Enrichment.(*session.GrepEnrichment)
			require.True(t, ok, "expected *GrepEnrichment, got %T", result.Enrichment)
			assert.Equal(t, tc.wantMode, g.Mode)
			assert.Equal(t, tc.wantNumFiles, g.NumFiles)
			assert.Equal(t, tc.wantNumLines, g.NumLines)
			assert.Equal(t, tc.wantNumMatches, g.NumMatches)
		})
	}
}

// TestTransformGlobEnrichment: Glob tool result produces GlobEnrichment with
// file count and truncated flag.
func TestTransformGlobEnrichment(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name          string
		toolResult    map[string]any
		wantNumFiles  int
		wantTruncated bool
	}{
		{
			name: "normal",
			toolResult: map[string]any{
				"filenames":  []any{"a.go", "b.go"},
				"durationMs": float64(23),
				"numFiles":   float64(8),
				"truncated":  false,
			},
			wantNumFiles: 8, wantTruncated: false,
		},
		{
			name: "truncated",
			toolResult: map[string]any{
				"filenames":  []any{"a.go"},
				"durationMs": float64(50),
				"numFiles":   float64(100),
				"truncated":  true,
			},
			wantNumFiles: 100, wantTruncated: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := parser.Result{
				Messages: []parser.Message{
					{Role: "user", Timestamp: &ts, LineNum: 1,
						Content: []parser.ContentBlock{&parser.TextBlock{Text: "list"}}},
					{Role: "assistant", Timestamp: &ts, LineNum: 2,
						Content: []parser.ContentBlock{&parser.ToolUseBlock{ID: "toolu_01", Name: "Glob", Input: map[string]any{"pattern": "*.go"}}}},
					{Role: "user", Timestamp: &ts, LineNum: 3,
						Envelope: parser.Envelope{ToolUseResult: tc.toolResult},
						Content:  []parser.ContentBlock{&parser.ToolResultBlock{ToolUseID: "toolu_01", Content: "files"}}},
				},
			}
			s := session.Transform(res)
			var result *session.ToolResult
			for _, b := range s.Turns[0].Blocks {
				if tr, ok := b.(*session.ToolResult); ok {
					result = tr
				}
			}
			require.NotNil(t, result)
			require.NotNil(t, result.Enrichment)
			g, ok := result.Enrichment.(*session.GlobEnrichment)
			require.True(t, ok, "expected *GlobEnrichment, got %T", result.Enrichment)
			assert.Equal(t, tc.wantNumFiles, g.NumFiles)
			assert.Equal(t, tc.wantTruncated, g.Truncated)
		})
	}
}

// TestTransformGlobEnrichment_Malformed: Glob result missing numFiles produces nil enrichment.
func TestTransformGlobEnrichment_Malformed(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	res := parser.Result{
		Messages: []parser.Message{
			{Role: "user", Timestamp: &ts, LineNum: 1,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "list"}}},
			{Role: "assistant", Timestamp: &ts, LineNum: 2,
				Content: []parser.ContentBlock{&parser.ToolUseBlock{ID: "toolu_01", Name: "Glob", Input: map[string]any{}}}},
			{Role: "user", Timestamp: &ts, LineNum: 3,
				Envelope: parser.Envelope{ToolUseResult: map[string]any{"truncated": false}},
				Content:  []parser.ContentBlock{&parser.ToolResultBlock{ToolUseID: "toolu_01", Content: "err"}}},
		},
	}
	s := session.Transform(res)
	var result *session.ToolResult
	for _, b := range s.Turns[0].Blocks {
		if tr, ok := b.(*session.ToolResult); ok {
			result = tr
		}
	}
	require.NotNil(t, result)
	assert.Nil(t, result.Enrichment, "glob without numFiles should produce nil enrichment")
}

// TestTransformGrepEnrichment_NoMode: Grep result without mode field produces nil enrichment.
func TestTransformGrepEnrichment_NoMode(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	res := parser.Result{
		Messages: []parser.Message{
			{Role: "user", Timestamp: &ts, LineNum: 1,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "find"}}},
			{Role: "assistant", Timestamp: &ts, LineNum: 2,
				Content: []parser.ContentBlock{&parser.ToolUseBlock{ID: "toolu_01", Name: "Grep", Input: map[string]any{}}}},
			{Role: "user", Timestamp: &ts, LineNum: 3,
				Envelope: parser.Envelope{ToolUseResult: map[string]any{"numFiles": float64(0)}},
				Content:  []parser.ContentBlock{&parser.ToolResultBlock{ToolUseID: "toolu_01", Content: "no match"}}},
		},
	}
	s := session.Transform(res)
	var result *session.ToolResult
	for _, b := range s.Turns[0].Blocks {
		if tr, ok := b.(*session.ToolResult); ok {
			result = tr
		}
	}
	require.NotNil(t, result)
	assert.Nil(t, result.Enrichment, "malformed grep result should produce nil enrichment")
}

// TestTransformAgentEnrichment: completed Agent tool result produces AgentEnrichment
// with agent type, status, duration, token count, and tool use count.
func TestTransformAgentEnrichment(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	res := parser.Result{
		Messages: []parser.Message{
			{Role: "user", Timestamp: &ts, LineNum: 1,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "research this"}}},
			{Role: "assistant", Timestamp: &ts, LineNum: 2,
				Content: []parser.ContentBlock{&parser.ToolUseBlock{ID: "toolu_01", Name: "Agent", Input: map[string]any{"prompt": "find it"}}}},
			{Role: "user", Timestamp: &ts, LineNum: 3,
				Envelope: parser.Envelope{ToolUseResult: map[string]any{
					"status":            "completed",
					"agentId":           "a573d391f7b0477b8",
					"agentType":         "general-purpose",
					"prompt":            "find it",
					"totalDurationMs":   float64(45000),
					"totalTokens":       float64(12500),
					"totalToolUseCount": float64(8),
				}},
				Content: []parser.ContentBlock{&parser.ToolResultBlock{ToolUseID: "toolu_01", Content: "done"}}},
		},
	}
	s := session.Transform(res)
	var result *session.ToolResult
	for _, b := range s.Turns[0].Blocks {
		if tr, ok := b.(*session.ToolResult); ok {
			result = tr
		}
	}
	require.NotNil(t, result)
	require.NotNil(t, result.Enrichment)
	a, ok := result.Enrichment.(*session.AgentEnrichment)
	require.True(t, ok, "expected *AgentEnrichment, got %T", result.Enrichment)
	assert.Equal(t, "general-purpose", a.AgentType)
	assert.Equal(t, "completed", a.Status)
	assert.Equal(t, 45000, a.DurationMs)
	assert.Equal(t, 12500, a.TokenCount)
	assert.Equal(t, 8, a.ToolUseCount)
}

// TestTransformAgentEnrichment_AsyncLaunch: async Agent produces nil enrichment.
func TestTransformAgentEnrichment_AsyncLaunch(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	res := parser.Result{
		Messages: []parser.Message{
			{Role: "user", Timestamp: &ts, LineNum: 1,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "go"}}},
			{Role: "assistant", Timestamp: &ts, LineNum: 2,
				Content: []parser.ContentBlock{&parser.ToolUseBlock{ID: "toolu_01", Name: "Agent", Input: map[string]any{}}}},
			{Role: "user", Timestamp: &ts, LineNum: 3,
				Envelope: parser.Envelope{ToolUseResult: map[string]any{
					"isAsync":     true,
					"status":      "async_launched",
					"agentId":     "abc123",
					"description": "background task",
				}},
				Content: []parser.ContentBlock{&parser.ToolResultBlock{ToolUseID: "toolu_01", Content: "launched"}}},
		},
	}
	s := session.Transform(res)
	var result *session.ToolResult
	for _, b := range s.Turns[0].Blocks {
		if tr, ok := b.(*session.ToolResult); ok {
			result = tr
		}
	}
	require.NotNil(t, result)
	assert.Nil(t, result.Enrichment, "async agent should produce nil enrichment")
}

// TestTransformAgentEnrichment_StringError: string-type Agent result produces nil enrichment.
func TestTransformAgentEnrichment_StringError(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	res := parser.Result{
		Messages: []parser.Message{
			{Role: "user", Timestamp: &ts, LineNum: 1,
				Content: []parser.ContentBlock{&parser.TextBlock{Text: "go"}}},
			{Role: "assistant", Timestamp: &ts, LineNum: 2,
				Content: []parser.ContentBlock{&parser.ToolUseBlock{ID: "toolu_01", Name: "Agent", Input: map[string]any{}}}},
			{Role: "user", Timestamp: &ts, LineNum: 3,
				Envelope: parser.Envelope{ToolUseResult: "Error: agent failed"},
				Content:  []parser.ContentBlock{&parser.ToolResultBlock{ToolUseID: "toolu_01", Content: "error"}}},
		},
	}
	s := session.Transform(res)
	var result *session.ToolResult
	for _, b := range s.Turns[0].Blocks {
		if tr, ok := b.(*session.ToolResult); ok {
			result = tr
		}
	}
	require.NotNil(t, result)
	assert.Nil(t, result.Enrichment, "string agent error should produce nil enrichment")
}

// TestTransformRedactedThinking: RedactedThinkingBlock maps to *session.RedactedThinking.
func TestTransformRedactedThinking(t *testing.T) {
	res := parseFixture(t, "redacted_thinking.jsonl")
	s := session.Transform(res)

	require.Len(t, s.Turns, 1)
	blocks := s.Turns[0].Blocks
	// user text + thinking + redacted thinking + assistant text
	require.Len(t, blocks, 4)
	require.IsType(t, (*session.Thinking)(nil), blocks[1])
	require.IsType(t, (*session.RedactedThinking)(nil), blocks[2])
	assert.Equal(t, "redacted-data-xyz", blocks[2].(*session.RedactedThinking).Data)
}
