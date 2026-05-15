// Package picker provides the interactive session-selection UI shown when
// relic is invoked without a file argument. It scans ~/.claude/projects for
// JSONL session files and presents a two-step terminal menu: first choose a
// project directory, then choose a session within it. Sessions are sorted by
// modification time so the most recent work surfaces first.
//
// This package owns the discovery and presentation of sessions but has no
// knowledge of their content — parsing is deferred to the parser package
// after the user makes a selection.
package picker
