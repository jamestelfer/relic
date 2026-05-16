package redact

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/rs/zerolog"
	"github.com/zricethezav/gitleaks/v8/detect"
	"github.com/zricethezav/gitleaks/v8/logging"
	"github.com/zricethezav/gitleaks/v8/report"
)

// Finding records a single deduplicated secret detection. It carries the rule
// ID, a human-readable description, and the number of JSONL lines the secret
// appeared on. It never contains the raw secret value.
type Finding struct {
	RuleID      string
	Description string
	LineCount   int
}

// Summary holds the deduplicated redaction findings after a reader has been
// fully consumed. Entries are keyed by the combination of secret value and
// rule ID during accumulation, then the secret is discarded before export.
type Summary struct {
	Findings []Finding
}

// Reader wraps an io.Reader, scanning each JSONL line for secrets and
// replacing detected values with [REDACTED:<rule-id>] markers.
type Reader struct {
	detector *detect.Detector
	scanner  *bufio.Scanner
	buf      bytes.Buffer
	done     bool

	// findings accumulates detections keyed by secret value for deduplication.
	findings map[string]*accumulator
}

type accumulator struct {
	ruleID      string
	description string
	lineCount   int
}

// NewReader creates a redacting reader that scans lines for secrets using
// gitleaks default rules. Returns an error if the detector cannot be initialized.
func NewReader(r io.Reader) (*Reader, error) {
	logging.Logger = zerolog.New(io.Discard)

	d, err := detect.NewDetectorDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("initialize secret detector: %w (use --no-redact to skip redaction)", err)
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	return &Reader{
		detector: d,
		scanner:  scanner,
		findings: make(map[string]*accumulator),
	}, nil
}

// Read implements io.Reader. It reads one JSONL line at a time from the
// underlying reader, redacts secrets, and serves the cleaned bytes.
func (rd *Reader) Read(p []byte) (int, error) {
	for rd.buf.Len() == 0 {
		if rd.done {
			return 0, io.EOF
		}
		if !rd.scanner.Scan() {
			rd.done = true
			if err := rd.scanner.Err(); err != nil {
				return 0, err
			}
			return 0, io.EOF
		}
		line := rd.scanner.Text()
		cleaned := rd.redactLine(line)
		rd.buf.WriteString(cleaned)
		rd.buf.WriteByte('\n')
	}
	return rd.buf.Read(p)
}

func (rd *Reader) redactLine(line string) string {
	findings := rd.detector.DetectString(line)
	if len(findings) == 0 {
		return line
	}

	rd.accumulate(findings)

	for _, f := range findings {
		marker := "[REDACTED:" + f.RuleID + "]"
		line = strings.ReplaceAll(line, f.Secret, marker)
	}
	return line
}

func (rd *Reader) accumulate(findings []report.Finding) {
	for _, f := range findings {
		if acc, ok := rd.findings[f.Secret]; ok {
			acc.lineCount++
		} else {
			rd.findings[f.Secret] = &accumulator{
				ruleID:      f.RuleID,
				description: f.Description,
				lineCount:   1,
			}
		}
	}
}

// Summary returns the deduplicated redaction findings. Call after the reader
// has been fully consumed.
func (rd *Reader) Summary() Summary {
	findings := make([]Finding, 0, len(rd.findings))
	for _, acc := range rd.findings {
		findings = append(findings, Finding{
			RuleID:      acc.ruleID,
			Description: acc.description,
			LineCount:   acc.lineCount,
		})
	}
	return Summary{Findings: findings}
}
