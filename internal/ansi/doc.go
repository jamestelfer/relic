// Package ansi converts ANSI escape sequences into HTML for display in
// rendered sessions. Tool results frequently contain terminal output with
// colour codes that would otherwise render as garbage in the browser; this
// package bridges that gap by producing CSS-class-based spans (e.g.
// term-fg31) that pair with the terminal stylesheet in the renderer.
//
// It sits between the session layer (which supplies raw tool output) and the
// renderer (which embeds the HTML into the final document). A strip function
// is also provided for contexts that need plain text, such as block
// summaries.
package ansi
