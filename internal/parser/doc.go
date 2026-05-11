// Package parser performs faithful structural decoding of Claude Code JSONL
// session files into typed Go structs. It does not perform semantic
// classification — that responsibility belongs to the session package.
//
// Each JSONL line is decoded into one of: Message (user/assistant records),
// SystemRecord (system records), CustomTitleRecord (custom-title records), or
// SkippedRecord (unrecognised top-level types). Parse errors are collected
// per-line without aborting the stream.
package parser
