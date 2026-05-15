// Package parser is the first stage of the data pipeline: it decodes Claude
// Code JSONL session files into typed Go structs. Its job is faithful
// structural mapping — JSON shapes to Go types — without interpreting
// semantics. That separation exists so the parser can evolve with JSONL
// format changes independently of the classification logic in session.
//
// Each JSONL line is decoded into one of: Message (user/assistant records),
// SystemRecord (system records), CustomTitleRecord (custom-title records), or
// SkippedRecord (unrecognised top-level types). Parse errors are collected
// per-line without aborting the stream, because partial sessions are common
// (interrupted conversations, truncated files) and should still render.
package parser
