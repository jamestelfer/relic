// Package session transforms parsed Claude Code messages into a structured
// session model suitable for rendering. It performs semantic classification:
// routing parser records into typed blocks, enforcing turn boundaries,
// filtering sidechain messages, cross-referencing tool pairs, and computing
// session-level metadata (header, duration, files touched).
//
// The Block interface is sealed via an unexported method — all concrete block
// types are defined in this package.
package session
