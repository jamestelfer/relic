# Plan: Secret Redaction

> Source PRD: `docs/redaction.prd.md`

## Architectural decisions

- **Package**: `internal/redact/` — new package, no modifications to parser, session, or renderer.
- **Integration point**: `io.Reader` wrapper inserted between `os.Open` and `parser.Parse` in `execute()` (`cmd/relic/main.go`). The parser receives already-clean input.
- **Detection engine**: `github.com/zricethezav/gitleaks/v8` — `detect.NewDetectorDefaultConfig()`, `DetectString()` API. Default ruleset only, no custom config.
- **Replacement strategy**: `strings.ReplaceAll(line, finding.Secret, "[REDACTED:"+finding.RuleID+"]")` on raw JSONL text before JSON unmarshalling.
- **Zerolog suppression**: Set `logging.Logger = zerolog.New(io.Discard)` in the detector constructor, not in package `init()`.
- **CLI flag**: `--no-redact` top-level boolean on the root command. When set, the raw `io.Reader` passes directly to the parser with zero overhead.
- **Summary output**: Printed to stderr after processing. Format: `N secrets redacted: rule-id (M lines), ...`. Deduplicated by secret value, not by rule ID alone.

## P0 baseline and standard quality gate

- [ ] Standard commands: `just verify` (runs generate → tidy → fix → fmt → build → lint → test)
- [ ] Run as P0 baseline before Phase 1; all must pass
- [ ] If P0 fails, add stabilization work before planned phases
- [ ] Re-run before each phase completion; all must pass

---

## Phase 1: Redacting reader with gitleaks detection

**EARS requirements**: R1, R2, R3, R4, R13

### Why this phase exists

Delivers the core redaction capability as a standalone, testable package. After this phase, `internal/redact.Reader` can scan JSONL lines for secrets and replace them with markers — the foundation everything else builds on.

### Locked decisions (non-negotiable)

- Package path: `internal/redact/`
- Public API: `NewReader(r io.Reader) (*Reader, error)` returning an `io.Reader` wrapper, and `(*Reader).Summary() Summary` for retrieving findings after reading completes.
- Replacement format: `[REDACTED:<rule-id>]` — uses `finding.RuleID` from gitleaks.
- Replacement method: `strings.ReplaceAll` on the raw JSONL line text.
- 1:1 line invariant: one output line per input line, preserving line numbers.
- Zerolog suppression: `logging.Logger = zerolog.New(io.Discard)` inside the constructor.
- Raw secret values must never appear in any return value, error message, or log output from this package.

### Flex zone (implementation choice allowed)

- Internal buffering strategy (bufio.Scanner, line-at-a-time reads, etc.).
- `Summary` struct shape — as long as it supports deduplicated findings with rule ID and line count.
- Whether to expose a `nil`-safe no-op reader variant or handle that at the call site.

### End-to-end behaviour to implement

Create `internal/redact/` with a `Reader` type that wraps an `io.Reader`. When `Read()` is called, it reads one JSONL line at a time from the underlying reader, runs `detect.DetectString()` against it, replaces each detected secret via `strings.ReplaceAll`, and returns the cleaned bytes. Findings are accumulated internally. After the caller finishes reading, `Summary()` returns deduplicated results.

### Acceptance criteria

- [ ] `[observable]` A synthetic JSONL line containing a GitHub PAT (`ghp_...`) is read through the `Reader`; the output line contains `[REDACTED:github-pat]` (or the actual gitleaks rule ID) and no raw secret value.
- [ ] `[observable]` A synthetic JSONL line containing an AWS access key (`AKIA...`) is redacted with the appropriate rule ID marker.
- [ ] `[observable]` Non-secret content passes through unchanged byte-for-byte.
- [ ] `[observable]` Reading N input lines produces exactly N output lines.
- [ ] `[observable]` The same secret value appearing on 3 different lines produces a `Summary` with 1 unique finding and line count 3.
- [ ] `[structural]` No raw secret values appear in `Summary` output, error returns, or any exported type.

### Verification

Run `just test ./internal/redact/...` — all unit tests pass. Manually inspect test assertions to confirm no raw secret values leak into test output or error messages.

### Replan triggers

- Gitleaks `DetectString` API does not exist or has a different signature in the available version.
- Gitleaks default config initialization requires filesystem access or fails in test environments.
- The `strings.ReplaceAll` approach breaks JSON structure for a real-world secret pattern (unlikely but possible with secrets containing JSON-structural characters).

---

## Phase 2: CLI integration and stderr summary

**EARS requirements**: R5, R6, R7, R8, R9, R12, R14

**Carry-forward**: Before starting this phase, re-verify Phase 1 by running `just test ./internal/redact/...`.

### Why this phase exists

Wires the redacting reader into the CLI pipeline so users get automatic secret redaction when running `relic`. After this phase, the feature is end-to-end functional: secrets are redacted in rendered HTML, the user sees a summary on stderr, and `--no-redact` bypasses everything.

### Locked decisions (non-negotiable)

- Redaction is on by default for all output modes (file, stdout, gist).
- `--no-redact` is a top-level boolean flag on the root command.
- When `--no-redact` is set, no `Reader` is created and the raw `io.Reader` passes directly to `parser.Parse`.
- Summary prints to stderr after rendering completes, not during.
- Summary format: `N secrets redacted: rule-id (M lines), rule-id (M lines)`.
- No summary output when `--no-redact` is active or when no secrets are found.
- Redaction markers render as plain text in HTML — no special styling.
- The rendered HTML remains self-contained; redaction introduces no external resources.

### Flex zone (implementation choice allowed)

- Where in `execute()` the reader is inserted (before or after file open, wrapper construction pattern).
- How `--no-redact` propagates into `execute()` (field on `options` struct, separate parameter, etc.).
- Summary formatting implementation (fmt.Fprintf, strings.Builder, etc.).

### End-to-end behaviour to implement

In `cmd/relic/main.go`, add a `--no-redact` flag. In `execute()`, after opening the file and before calling `parser.Parse`, wrap the file reader with `redact.NewReader` (unless `--no-redact`). After `parser.Parse` completes, retrieve the summary from the reader and print it to stderr if any secrets were found.

### Acceptance criteria

- [ ] `[observable]` Running `dist/relic <file-with-secrets>` produces HTML where secret values are replaced with `[REDACTED:...]` markers.
- [ ] `[observable]` stderr shows a summary line like `2 secrets redacted: github-pat (3 lines), aws-access-token (1 line)`.
- [ ] `[observable]` Running `dist/relic --no-redact <file-with-secrets>` produces HTML with raw secret values intact and no stderr summary.
- [ ] `[observable]` Running `dist/relic <file-without-secrets>` produces no stderr summary.
- [ ] `[structural]` The `--no-redact` flag is registered on the root command and available for all output modes.
- [ ] `[structural]` The rendered HTML contains no external resource references introduced by redaction.

### Verification

Build with `just build`. Test with a JSONL fixture containing planted secrets (GitHub PAT, AWS key). Verify HTML output contains markers, stderr shows summary, `--no-redact` bypasses both. Run `just verify` for full suite.

### Regression watchpoints

- Existing HTML output for sessions without secrets must be byte-identical with and without the redacting reader in the pipeline.
- Parser error handling (malformed lines) must still work — the redacting reader must not swallow or alter error-producing lines in a way that changes parser behavior.

### Replan triggers

- The `options` struct or `execute()` signature has changed since Phase 1 in a way that makes wiring difficult.
- The redacting reader introduces measurable latency that makes the tool noticeably slower on large sessions (unlikely but worth checking).

---

## Phase 3: Error handling and full-pipeline snapshot

**EARS requirements**: R10, R11

**Carry-forward**: Before starting this phase, re-verify Phases 1-2 by running `just verify`.

### Why this phase exists

Covers the failure modes: what happens when gitleaks can't initialize, and what happens when redaction produces invalid JSON. Also adds a snapshot test that locks in the full-pipeline behavior (redact → parse → session → render) for regression protection.

### Locked decisions (non-negotiable)

- If gitleaks detector fails to initialize, `relic` exits with non-zero status and an error message mentioning `--no-redact` as the workaround.
- If a redacted line fails JSON parsing, the parser's existing malformed-line handling applies (skip with warning) — no special redaction-layer error handling needed.
- Snapshot test uses a fixture JSONL file with planted secrets, processed through the full pipeline. Updated with `just update-snaps`.

### Flex zone (implementation choice allowed)

- How the init error propagates (return from `NewReader`, checked in `execute()`).
- Fixture file content and which secret patterns to plant.
- Whether the snapshot captures full HTML or a relevant subset.

### End-to-end behaviour to implement

1. Ensure `redact.NewReader` returns a clear error when detector initialization fails. In `execute()`, handle this error by exiting with a message that includes `--no-redact`.
2. Create a test fixture JSONL file containing planted secrets. Write a snapshot test that processes it through the full pipeline and captures the rendered HTML.
3. Verify that a redacted line that becomes invalid JSON is handled by the parser's existing error path.

### Acceptance criteria

- [ ] `[observable]` When gitleaks detector init fails, `relic` exits non-zero with an error message containing `--no-redact`.
- [ ] `[observable]` A JSONL line that becomes invalid JSON after redaction is skipped with a warning, consistent with existing parser behavior.
- [ ] `[observable]` Full-pipeline snapshot test passes: fixture JSONL with secrets → rendered HTML contains `[REDACTED:...]` markers, no raw secrets.
- [ ] `[structural]` The snapshot is committed and can be updated with `just update-snaps`.

### Verification

Run `just verify`. Inspect the snapshot file to confirm redaction markers appear and no raw secrets are present. For the init failure case, a unit test that forces the error path is sufficient (no need to break gitleaks in a running build).

### Regression watchpoints

- Existing snapshot tests must not change — redaction is transparent to sessions without secrets.
- The error exit code and message format should be consistent with other `relic` error paths.

### Replan triggers

- Gitleaks detector initialization cannot be made to fail in a controlled way for testing.
- The snapshot test reveals unexpected differences in rendered HTML that indicate a deeper integration issue.

---

## Requirements coverage matrix

| Requirement ID | Phase(s) | Notes |
|---|---|---|
| R1 | Phase 1 | Core scan + replace on raw JSONL |
| R2 | Phase 1 | strings.ReplaceAll strategy |
| R3 | Phase 1 | gitleaks default config ruleset |
| R4 | Phase 1 | No raw secrets in any output |
| R5 | Phase 2 | Redaction on by default |
| R6 | Phase 2 | --no-redact flag skips redaction |
| R7 | Phase 2 | Deduplicated stderr summary |
| R8 | Phase 2 | Same secret across lines = 1 finding |
| R9 | Phase 2 | --no-redact suppresses summary |
| R10 | Phase 3 | Detector init failure handling |
| R11 | Phase 3 | Malformed JSON after redaction |
| R12 | Phase 2 | Markers render as plain text |
| R13 | Phase 1 | 1:1 line invariant |
| R14 | Phase 2 | No external resources introduced |
