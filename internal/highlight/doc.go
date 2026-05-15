// Package highlight provides syntax highlighting for code blocks, diffs, and
// file content that appear in rendered sessions. It wraps Chroma with a
// class-based HTML formatter (no inline styles) so that all colour comes from
// a single CSS rule set injected into the document <head> by the renderer.
//
// A custom "userbashsession" Chroma lexer is registered at init time to
// handle interactive terminal transcripts that don't match Chroma's built-in
// lexers. The package also computes semantic diffs between old and new file
// content for Edit tool results, producing highlighted unified-diff output.
package highlight
