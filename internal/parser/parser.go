// Package parser decodes Claude Code JSONL session files into typed Go structs.
package parser

import (
	"bufio"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
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

// Message represents a single user or assistant message from a session.
type Message struct {
	Role      string
	Timestamp *time.Time
	Model     string
	Content   []ContentBlock
}

// record mirrors the top-level structure of a JSONL line.
type record struct {
	Type      string     `json:"type"`
	Timestamp string     `json:"timestamp"`
	Message   msgPayload `json:"message"`
}

// msgPayload is the nested "message" object inside a record.
type msgPayload struct {
	Role    string         `json:"role"`
	Model   string         `json:"model"`
	Content jsontext.Value `json:"content"`
}

// Parse reads a JSONL session file and returns one Message per user/assistant
// record. Lines with other top-level types (system, queue-operation, etc.) are
// silently skipped. Per-line decode failures are collected in []ParseError and
// do not prevent other lines from being parsed. An I/O failure returns a
// non-nil error.
func Parse(r io.Reader) ([]Message, []ParseError, error) {
	var msgs []Message
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
			msgs = append(msgs, Message{
				Role: "error",
				Content: []ContentBlock{&ErrorBlock{
					LineNum: lineNum,
					Raw:     raw,
					Msg:     err.Error(),
				}},
			})
			continue
		}

		if rec.Type != "user" && rec.Type != "assistant" {
			continue
		}

		msg := Message{
			Role:  rec.Message.Role,
			Model: rec.Message.Model,
		}

		if rec.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339Nano, rec.Timestamp); err == nil {
				msg.Timestamp = &t
			} else if t, err := time.Parse(time.RFC3339, rec.Timestamp); err == nil {
				msg.Timestamp = &t
			}
		}

		msg.Content = decodeContent(rec.Message.Content)
		msgs = append(msgs, msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return msgs, parseErrs, nil
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
	}

	return &RawBlock{RawType: typed.Type, Data: raw}
}
