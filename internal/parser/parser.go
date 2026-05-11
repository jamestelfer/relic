package parser

import (
	"bufio"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
	"strings"
	"time"
)

// ContentBlock is the common interface for all message content blocks.
type ContentBlock interface {
	// BlockType returns the raw JSON "type" discriminator string.
	BlockType() string
}

// ErrorBlock represents a failed JSONL line, rendered as an error callout.
type ErrorBlock struct {
	LineNum int
	Raw     []byte
	Msg     string
}

func (b *ErrorBlock) BlockType() string { return "error" }

// TextBlock holds a plain-text or Markdown content block.
type TextBlock struct {
	Text string
}

func (b *TextBlock) BlockType() string { return "text" }

// ToolResultBlock represents a tool_result content block (output of a tool call).
type ToolResultBlock struct {
	ToolUseID string
	Content   string // may contain ANSI escape sequences
}

func (b *ToolResultBlock) BlockType() string { return "tool_result" }

// ToolUseBlock represents a tool_use content block (a tool invocation).
type ToolUseBlock struct {
	ID    string
	Name  string
	Input map[string]any
}

func (b *ToolUseBlock) BlockType() string { return "tool_use" }

// ThinkingBlock represents a thinking content block (internal reasoning).
type ThinkingBlock struct {
	Thinking string
}

func (b *ThinkingBlock) BlockType() string { return "thinking" }

// RedactedThinkingBlock represents a redacted thinking content block.
type RedactedThinkingBlock struct {
	Data string
}

func (b *RedactedThinkingBlock) BlockType() string { return "redacted_thinking" }

// ImageBlock represents a base64-encoded inline image content block.
type ImageBlock struct {
	MediaType string
	Base64    string
}

func (b *ImageBlock) BlockType() string { return "image" }

// RawBlock holds an unknown content block type, preserved as raw JSON.
type RawBlock struct {
	RawType string
	Data    jsontext.Value
}

func (b *RawBlock) BlockType() string { return b.RawType }

// ParseError records a non-fatal failure decoding a single JSONL line.
type ParseError struct {
	Line int    // 1-indexed line number in the source file
	Raw  []byte // raw bytes of the failed line
	Err  error  // underlying decode error
}

// Envelope holds top-level metadata fields present on every JSONL record.
// Absent fields produce zero values — no error is raised.
type Envelope struct {
	IsSidechain       bool
	IsCompactSummary  bool
	IsApiErrorMessage bool
	IsMeta            bool
	Version           string
	Cwd               string
	GitBranch         string
	Slug              string
}

// Message represents a single user or assistant message from a session.
type Message struct {
	Role      string
	Timestamp *time.Time
	Model     string
	Content   []ContentBlock
	// LineNum is the 1-indexed JSONL line the message was parsed from.
	LineNum int
	// IsMeta mirrors the top-level JSONL "isMeta" flag. True marks a message
	// whose content is Claude Code bookkeeping (system-reminder, local-command
	// caveat, etc.) rather than a user-authored prompt.
	IsMeta   bool
	Envelope Envelope
}

// CustomTitleRecord carries a top-level "custom-title" record's text with its
// source line number. Emitted as part of parser.Result.
type CustomTitleRecord struct {
	Text    string
	LineNum int
}

// SystemRecord carries a top-level "system" record's subtype + content with
// its source line number. Emitted as part of parser.Result; the session layer
// decides how to render each subtype.
type SystemRecord struct {
	Subtype string
	Content string
	LineNum int
}

// ResultRecord carries a top-level "result" record's fields. Emitted at the
// end of a session to report outcome (success/error), duration, cost, etc.
type ResultRecord struct {
	Subtype    string
	DurationMS int
	NumTurns   int
	CostUSD    float64
	Error      string
	LineNum    int
}

// SkippedRecord flags a top-level JSONL record whose "type" is not one of
// user/assistant/system/custom-title/result. Phase 7 debug logging surfaces
// these so schema drift in the session format is visible rather than silent.
type SkippedRecord struct {
	LineNum int
	Type    string
}

// Result is the structured output of Parse. Messages, CustomTitles,
// SystemRecords, ResultRecords, and SkippedRecords are the five record streams
// extracted from a JSONL session.
type Result struct {
	Messages       []Message
	CustomTitles   []CustomTitleRecord
	SystemRecords  []SystemRecord
	ResultRecords  []ResultRecord
	SkippedRecords []SkippedRecord
}

// record mirrors the top-level structure of a JSONL line.
type record struct {
	Type              string     `json:"type"`
	Timestamp         string     `json:"timestamp"`
	IsMeta            bool       `json:"isMeta"`
	IsSidechain       bool       `json:"isSidechain"`
	IsCompactSummary  bool       `json:"isCompactSummary"`
	IsApiErrorMessage bool       `json:"isApiErrorMessage"`
	Version           string     `json:"version"`
	Cwd               string     `json:"cwd"`
	GitBranch         string     `json:"gitBranch"`
	Slug              string     `json:"slug"`
	Message           msgPayload `json:"message"`
}

// msgPayload is the nested "message" object inside a record.
type msgPayload struct {
	Role    string         `json:"role"`
	Model   string         `json:"model"`
	Content jsontext.Value `json:"content"`
}

// customTitleRecord mirrors a top-level custom-title JSONL line.
type customTitleRecord struct {
	Type        string `json:"type"`
	CustomTitle string `json:"customTitle"`
}

// systemRecord mirrors a top-level system JSONL line.
type systemRecord struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Content string `json:"content"`
}

// resultRecord mirrors a top-level result JSONL line.
type resultRecord struct {
	Type       string  `json:"type"`
	Subtype    string  `json:"subtype"`
	DurationMS int     `json:"duration_ms"`
	NumTurns   int     `json:"num_turns"`
	CostUSD    float64 `json:"cost_usd"`
	Error      string  `json:"error"`
}

// knownBookkeepingTypes enumerates top-level record types Claude Code emits
// for its own state tracking. They carry no renderable payload; surfacing them
// as SkippedRecords would flood --debug output against real sessions. The set
// was discovered from the Phase 7 local sign-off against the reference JSONL.
var knownBookkeepingTypes = map[string]bool{
	"attachment":            true,
	"permission-mode":       true,
	"last-prompt":           true,
	"file-history-snapshot": true,
	"agent-name":            true,
}

// Parse reads a JSONL session file and returns a Result carrying user/assistant
// messages and any custom-title records, in source order. Lines with other
// top-level types (system, queue-operation, etc.) are silently skipped.
// Per-line decode failures are collected in []ParseError and do not prevent
// other lines from being parsed. An I/O failure returns a non-nil error.
func Parse(r io.Reader) (Result, []ParseError, error) {
	var res Result
	var parseErrs []ParseError
	scanner := bufio.NewScanner(r)
	// Increase buffer size for lines with large content.
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec record
		if err := json.Unmarshal(line, &rec); err != nil {
			raw := make([]byte, len(line))
			copy(raw, line)
			pe := ParseError{Line: lineNum, Raw: raw, Err: err}
			parseErrs = append(parseErrs, pe)
			// Insert an error pseudo-message at this position so the renderer
			// can show a callout in the right place.
			res.Messages = append(res.Messages, Message{
				Role:    "error",
				LineNum: lineNum,
				Content: []ContentBlock{&ErrorBlock{
					LineNum: lineNum,
					Raw:     raw,
					Msg:     err.Error(),
				}},
			})
			continue
		}

		if rec.Type == "custom-title" {
			var ct customTitleRecord
			if err := json.Unmarshal(line, &ct); err == nil && ct.CustomTitle != "" {
				res.CustomTitles = append(res.CustomTitles, CustomTitleRecord{
					Text:    ct.CustomTitle,
					LineNum: lineNum,
				})
			}
			continue
		}

		if rec.Type == "system" {
			var sr systemRecord
			if err := json.Unmarshal(line, &sr); err == nil {
				res.SystemRecords = append(res.SystemRecords, SystemRecord{
					Subtype: sr.Subtype,
					Content: sr.Content,
					LineNum: lineNum,
				})
			}
			continue
		}

		if rec.Type == "result" {
			var rr resultRecord
			if err := json.Unmarshal(line, &rr); err == nil {
				res.ResultRecords = append(res.ResultRecords, ResultRecord{
					Subtype:    rr.Subtype,
					DurationMS: rr.DurationMS,
					NumTurns:   rr.NumTurns,
					CostUSD:    rr.CostUSD,
					Error:      rr.Error,
					LineNum:    lineNum,
				})
			}
			continue
		}

		if rec.Type != "user" && rec.Type != "assistant" {
			if !knownBookkeepingTypes[rec.Type] {
				res.SkippedRecords = append(res.SkippedRecords, SkippedRecord{
					LineNum: lineNum,
					Type:    rec.Type,
				})
			}
			continue
		}

		msg := Message{
			Role:    rec.Message.Role,
			Model:   rec.Message.Model,
			LineNum: lineNum,
			IsMeta:  rec.IsMeta,
			Envelope: Envelope{
				IsSidechain:       rec.IsSidechain,
				IsCompactSummary:  rec.IsCompactSummary,
				IsApiErrorMessage: rec.IsApiErrorMessage,
				IsMeta:            rec.IsMeta,
				Version:           rec.Version,
				Cwd:               rec.Cwd,
				GitBranch:         rec.GitBranch,
				Slug:              rec.Slug,
			},
		}

		if rec.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339Nano, rec.Timestamp); err == nil {
				msg.Timestamp = &t
			} else if t, err := time.Parse(time.RFC3339, rec.Timestamp); err == nil {
				msg.Timestamp = &t
			}
		}

		msg.Content = decodeContent(rec.Message.Content)
		res.Messages = append(res.Messages, msg)
	}

	if err := scanner.Err(); err != nil {
		return Result{}, nil, err
	}

	return res, parseErrs, nil
}

// decodeContent converts a raw JSON content value (string or array) into
// a typed []ContentBlock. A plain JSON string becomes a single TextBlock.
func decodeContent(raw jsontext.Value) []ContentBlock {
	if len(raw) == 0 {
		return nil
	}

	switch raw.Kind() {
	case '"':
		// Plain string content — wrap as a single TextBlock.
		var s string
		if err := json.Unmarshal([]byte(raw), &s); err == nil && s != "" {
			return []ContentBlock{&TextBlock{Text: s}}
		}
		return nil

	case '[':
		// Array of typed blocks.
		var rawBlocks []jsontext.Value
		if err := json.Unmarshal([]byte(raw), &rawBlocks); err != nil {
			return nil
		}
		blocks := make([]ContentBlock, 0, len(rawBlocks))
		for _, rb := range rawBlocks {
			blocks = append(blocks, decodeBlock(rb))
		}
		return blocks

	default:
		return nil
	}
}

// toolResultContent flattens a tool_result "content" field into a single
// string. Claude Code emits this field as either a plain string or an array
// of typed items. Recognised item types:
//   - "text": contributes the .text value
//   - "tool_reference": contributes the .tool_name value
//
// Items are joined with ", ".
func toolResultContent(raw jsontext.Value) string {
	if len(raw) == 0 {
		return ""
	}
	switch raw.Kind() {
	case '"':
		var s string
		if err := json.Unmarshal([]byte(raw), &s); err == nil {
			return s
		}
	case '[':
		var items []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			ToolName string `json:"tool_name"`
		}
		if err := json.Unmarshal([]byte(raw), &items); err == nil {
			parts := make([]string, 0, len(items))
			for _, it := range items {
				switch {
				case it.Type == "text" && it.Text != "":
					parts = append(parts, it.Text)
				case it.Type == "tool_reference" && it.ToolName != "":
					parts = append(parts, it.ToolName)
				}
			}
			return strings.Join(parts, ", ")
		}
	}
	return ""
}

// decodeBlock converts a single raw JSON block into the appropriate ContentBlock.
func decodeBlock(raw jsontext.Value) ContentBlock {
	// Extract the "type" field first.
	var typed struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(raw), &typed); err != nil || typed.Type == "" {
		return &RawBlock{RawType: "unknown", Data: raw}
	}

	switch typed.Type {
	case "text":
		var tb struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(raw), &tb); err == nil {
			return &TextBlock{Text: tb.Text}
		}
	case "tool_result":
		var tr struct {
			ToolUseID string         `json:"tool_use_id"`
			Content   jsontext.Value `json:"content"`
		}
		if err := json.Unmarshal([]byte(raw), &tr); err == nil {
			return &ToolResultBlock{
				ToolUseID: tr.ToolUseID,
				Content:   toolResultContent(tr.Content),
			}
		}
	case "tool_use":
		var tu struct {
			ID    string         `json:"id"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		}
		if err := json.Unmarshal([]byte(raw), &tu); err == nil {
			return &ToolUseBlock{ID: tu.ID, Name: tu.Name, Input: tu.Input}
		}
	case "thinking":
		var th struct {
			Thinking string `json:"thinking"`
		}
		if err := json.Unmarshal([]byte(raw), &th); err == nil {
			return &ThinkingBlock{Thinking: th.Thinking}
		}
	case "redacted_thinking":
		var rt struct {
			Data string `json:"data"`
		}
		if err := json.Unmarshal([]byte(raw), &rt); err == nil {
			return &RedactedThinkingBlock{Data: rt.Data}
		}
	case "image":
		var img struct {
			Source struct {
				Type      string `json:"type"`
				MediaType string `json:"media_type"`
				Data      string `json:"data"`
			} `json:"source"`
		}
		if err := json.Unmarshal([]byte(raw), &img); err == nil && img.Source.Data != "" {
			return &ImageBlock{MediaType: img.Source.MediaType, Base64: img.Source.Data}
		}
	}

	return &RawBlock{RawType: typed.Type, Data: raw}
}
