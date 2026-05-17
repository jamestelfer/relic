// Package redact provides an io.Reader wrapper that scans JSONL lines for
// secrets using gitleaks and replaces detected values with [REDACTED:<rule-id>]
// markers before the downstream consumer sees them.
package redact
