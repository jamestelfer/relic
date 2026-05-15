// Package renderer is the final stage of the data pipeline: it takes a
// [session.Session] and produces a self-contained HTML document with all CSS,
// JS, and images inlined. The output has no external dependencies and works
// offline (Google Fonts is the sole exception — the font payload is too large
// to inline, and the system font stack provides an acceptable fallback).
//
// The central dispatch is blockComponent, which maps each [session.Block]
// implementation to a templ template. Markdown in assistant messages is
// rendered via goldmark with fenced-code highlighting delegated to the
// highlight package. Raw HTML within markdown is sanitised through a
// bluemonday allowlist to prevent XSS while preserving structural markup.
// The templ templates are defined in renderer.templ and must be compiled with
// `go tool templ generate` before building.
package renderer
