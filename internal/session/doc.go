// Package session is the middle stage of the data pipeline: it transforms
// flat parser output into a structured, render-ready session model. This is
// where domain knowledge about Claude Code's message conventions lives —
// detecting hook injections, local-command echoes, request interruptions, and
// other synthetic user messages that the JSONL format does not distinguish
// from ordinary conversation.
//
// The detection cascade in classifyUserMessage routes each user message into
// a specialised block type. Messages are grouped into turns (human-initiated
// conversation units) and enriched with session-level metadata (title,
// duration, files touched). The Block interface is sealed via an unexported
// method so the renderer can exhaustively match on all block types.
package session
