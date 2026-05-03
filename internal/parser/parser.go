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

// TextBlock holds a plain-text or Markdown content block.
type TextBlock struct {
	Text string
}

func (b *TextBlock) BlockType() string { return "text" }

// RawBlock holds an unknown content block type, preserved as raw JSON.
type RawBlock struct {
	RawType string
	Data    jsontext.Value
}

func (b *RawBlock) BlockType() string { return b.RawType }

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
// silently skipped. An I/O failure returns a non-nil error.
func Parse(r io.Reader) ([]Message, error) {
	var msgs []Message
	scanner := bufio.NewScanner(r)
	// Increase buffer size for lines with large content.
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec record
		if err := json.Unmarshal(line, &rec); err != nil {
			// Phase 7 will handle per-line errors gracefully; for now skip malformed lines.
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
		return nil, err
	}

	return msgs, nil
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
	}

	return &RawBlock{RawType: typed.Type, Data: raw}
}
