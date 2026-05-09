# All go commands run under GOEXPERIMENT=jsonv2.
# This enables encoding/json/v2 in the standard library.
export GOEXPERIMENT := "jsonv2"

# Run templ code generation (must run before build/test).
# templ is a Go tool dependency (go.mod 'tool' directive); no separate install needed.
generate:
    go tool templ generate ./...

# Apply go fix upgrades (run after generate, before lint/test).
fix:
    go fix ./...

# Format
fmt:
    go fmt ./...

# Build
# Injects a version stamp of the form YYYYMMDD-<shortsha> derived from the
# HEAD commit date (not wall clock) so two builds of the same commit produce
# identical stamps. See docs/session-fidelity.prd.md §Build-time version stamp.
build: generate
    #!/usr/bin/env bash
    set -euo pipefail
    stamp="$(git log -1 --format=%cd --date=format:%Y%m%d)-$(git rev-parse --short HEAD)"
    go build -ldflags "-X github.com/jamestelfer/relic/internal/renderer.Version=${stamp}" -o dist/ ./cmd/...

# Lint
lint: generate
    golangci-lint run ./...

# Test — accepts optional arguments to target specific packages or tests.
# Examples:
#   just test                                    # run all tests
#   just test ./internal/parser/...             # one package
#   just test -run TestParse -v ./...           # named test, verbose
test *args="./...": generate
    go test {{args}}

# Update go-snaps golden snapshots.
# Examples:
#   just update-snaps                            # update all
#   just update-snaps ./internal/parser/...     # one package
update-snaps *args="./...": generate
    UPDATE_SNAPS=true go test {{args}}

# Tidy module dependencies
tidy:
    go mod tidy

# Full verify: generate → tidy → fix → fmt → build → lint → test
verify: generate tidy fix fmt build lint test
