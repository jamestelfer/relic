// Package gist handles the "publish to GitHub Gist" output mode. When the
// user chooses gist output instead of a local file, the rendered HTML is
// uploaded as a single-file gist via the gh CLI, and both the canonical gist
// URL and a gisthost preview URL are returned.
//
// The gh CLI dependency is abstracted behind the [Runner] interface so that
// tests can verify publishing logic without network access or an
// authenticated gh session.
package gist
