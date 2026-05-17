package redact

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/zricethezav/gitleaks/v8/config"
	"github.com/zricethezav/gitleaks/v8/detect"
	"github.com/zricethezav/gitleaks/v8/logging"
	"github.com/zricethezav/gitleaks/v8/report"
)

//go:embed rules.toml
var rulesConfig string

var (
	cachedDetector *detect.Detector
	cachedErr      error
)

// resetCache clears the detector cache. Used for testing only.
func resetCache() {
	cachedDetector = nil
	cachedErr = nil
}

// RuleCount pairs a rule ID with the number of distinct secrets it matched.
type RuleCount struct {
	RuleID string
	Count  int
}

// LineFinding records which rules matched on a single JSONL line.
type LineFinding struct {
	Line  int
	Rules []RuleCount
}

// Summary holds the redaction findings after a reader has been fully consumed.
type Summary struct {
	SecretCount int
	Lines       []LineFinding
}

// Reader wraps an io.Reader, scanning each JSONL line for secrets and
// replacing detected values with [REDACTED:<rule-id>] markers.
type Reader struct {
	detector *detect.Detector
	scanner  *bufio.Scanner
	buf      bytes.Buffer
	done     bool
	line     int

	secrets map[string]struct{}
	lines   []LineFinding
}

// newDetector creates the gitleaks detector with embedded custom rules merged
// with gitleaks defaults. The detector is created once and cached. Replaced in
// tests to simulate initialization failures.
var newDetector = func() (*detect.Detector, error) {
	if cachedDetector != nil || cachedErr != nil {
		return cachedDetector, cachedErr
	}

	v := viper.New()
	v.SetConfigType("toml")
	if err := v.ReadConfig(strings.NewReader(rulesConfig)); err != nil {
		cachedErr = err
		return nil, err
	}
	var vc config.ViperConfig
	if err := v.Unmarshal(&vc); err != nil {
		cachedErr = err
		return nil, err
	}
	cfg, err := vc.Translate()
	if err != nil {
		cachedErr = err
		return nil, err
	}
	cachedDetector = detect.NewDetector(cfg)
	return cachedDetector, nil
}

// NewReader creates a redacting reader that scans lines for secrets using
// gitleaks default rules. Returns an error if the detector cannot be initialized.
func NewReader(r io.Reader) (*Reader, error) {
	logging.Logger = zerolog.New(io.Discard)

	d, err := newDetector()
	if err != nil {
		return nil, fmt.Errorf("initialize secret detector: %w (use --no-redact to skip redaction)", err)
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	return &Reader{
		detector: d,
		scanner:  scanner,
		secrets:  make(map[string]struct{}),
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
	rd.line++

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
	ruleSecrets := make(map[string]map[string]struct{}, len(findings))
	for _, f := range findings {
		rd.secrets[f.Secret] = struct{}{}

		if ruleSecrets[f.RuleID] == nil {
			ruleSecrets[f.RuleID] = make(map[string]struct{})
		}
		ruleSecrets[f.RuleID][f.Secret] = struct{}{}
	}

	rules := make([]RuleCount, 0, len(ruleSecrets))
	for ruleID, secrets := range ruleSecrets {
		rules = append(rules, RuleCount{RuleID: ruleID, Count: len(secrets)})
	}
	slices.SortFunc(rules, func(a, b RuleCount) int {
		return strings.Compare(a.RuleID, b.RuleID)
	})

	rd.lines = append(rd.lines, LineFinding{
		Line:  rd.line,
		Rules: rules,
	})
}

// Summary returns the redaction findings. Call after the reader has been fully
// consumed.
func (rd *Reader) Summary() Summary {
	return Summary{
		SecretCount: len(rd.secrets),
		Lines:       rd.lines,
	}
}
