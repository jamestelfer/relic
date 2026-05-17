package redact

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zricethezav/gitleaks/v8/detect"
)

const (
	testGitHubPAT = "ghp_" + "zR8k4mVq2xN7pLw9cJ3hYf6eDgA5tB0sQiUo"
	testAWSKey    = "AKIA" + "Z7V4Q2XRNJ3WBTY5"
	testNpmToken  = "NpmToken." + "f00df00d-f00d-f00d-f00d-f00df00df00d"
)

func TestReader_RedactsGitHubPAT(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","content":"token: ` + testGitHubPAT + `"}}` + "\n"

	r, err := NewReader(strings.NewReader(line))
	require.NoError(t, err)

	out, err := io.ReadAll(r)
	require.NoError(t, err)

	result := string(out)
	assert.NotContains(t, result, testGitHubPAT)
	assert.Contains(t, result, "[REDACTED:github-pat]")
}

func TestReader_RedactsAWSAccessKey(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","content":"key: ` + testAWSKey + `"}}` + "\n"

	r, err := NewReader(strings.NewReader(line))
	require.NoError(t, err)

	out, err := io.ReadAll(r)
	require.NoError(t, err)

	result := string(out)
	assert.NotContains(t, result, testAWSKey)
	assert.Contains(t, result, "[REDACTED:aws-access-token]")
}

func TestReader_RedactsClassicNpmToken(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","content":"token: ` + testNpmToken + `"}}` + "\n"

	r, err := NewReader(strings.NewReader(line))
	require.NoError(t, err)

	out, err := io.ReadAll(r)
	require.NoError(t, err)

	result := string(out)
	assert.NotContains(t, result, testNpmToken)
	assert.Contains(t, result, "[REDACTED:npm-access-token-classic]")
}

func TestReader_PassesThroughNonSecretContent(t *testing.T) {
	line := `{"type":"user","message":{"role":"user","content":"hello world"}}` + "\n"

	r, err := NewReader(strings.NewReader(line))
	require.NoError(t, err)

	out, err := io.ReadAll(r)
	require.NoError(t, err)

	assert.Equal(t, line, string(out))
}

func TestReader_PreservesLineCount(t *testing.T) {
	lines := strings.Join([]string{
		`{"type":"user","message":{"role":"user","content":"hi"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":"token: ` + testGitHubPAT + `"}}`,
		`{"type":"user","message":{"role":"user","content":"thanks"}}`,
	}, "\n") + "\n"

	r, err := NewReader(strings.NewReader(lines))
	require.NoError(t, err)

	out, err := io.ReadAll(r)
	require.NoError(t, err)

	outLines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	assert.Len(t, outLines, 3)
}

func TestReader_SummaryDeduplicatesSameSecret(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","content":"token: ` + testGitHubPAT + `"}}` + "\n"
	input := line + line + line

	r, err := NewReader(strings.NewReader(input))
	require.NoError(t, err)

	_, err = io.ReadAll(r)
	require.NoError(t, err)

	summary := r.Summary()
	assert.Len(t, summary.Findings, 1)
	assert.Equal(t, "github-pat", summary.Findings[0].RuleID)
	assert.Equal(t, 3, summary.Findings[0].LineCount)
}

func TestReader_SummaryIsEmptyForNoSecrets(t *testing.T) {
	line := `{"type":"user","message":{"role":"user","content":"hello"}}` + "\n"

	r, err := NewReader(strings.NewReader(line))
	require.NoError(t, err)

	_, err = io.ReadAll(r)
	require.NoError(t, err)

	summary := r.Summary()
	assert.Empty(t, summary.Findings)
}

func TestReader_SummaryContainsNoRawSecrets(t *testing.T) {
	lines := strings.Join([]string{
		`{"type":"assistant","message":{"role":"assistant","content":"token: ` + testGitHubPAT + `"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":"key: ` + testAWSKey + `"}}`,
	}, "\n") + "\n"

	r, err := NewReader(strings.NewReader(lines))
	require.NoError(t, err)

	_, err = io.ReadAll(r)
	require.NoError(t, err)

	summary := r.Summary()
	for _, f := range summary.Findings {
		assert.NotContains(t, f.RuleID, testGitHubPAT)
		assert.NotContains(t, f.RuleID, testAWSKey)
		assert.NotContains(t, f.Description, testGitHubPAT)
		assert.NotContains(t, f.Description, testAWSKey)
	}
}

func TestReader_MalformedJSONAfterRedaction(t *testing.T) {
	// A line where the secret value is also a JSON structural element won't
	// happen in practice (secrets are alphanumeric tokens), but this verifies
	// the reader doesn't hide errors — the caller (parser) handles malformed
	// lines in its own error path.
	line := `{"key":"` + testGitHubPAT + `"}` + "\n"

	r, err := NewReader(strings.NewReader(line))
	require.NoError(t, err)

	out, err := io.ReadAll(r)
	require.NoError(t, err)

	result := string(out)
	assert.NotContains(t, result, testGitHubPAT)
	assert.Contains(t, result, "[REDACTED:github-pat]")
}

func TestNewReader_DetectorInitFailure(t *testing.T) {
	orig := newDetector
	t.Cleanup(func() {
		newDetector = orig
		resetCache()
	})

	resetCache()
	newDetector = func() (*detect.Detector, error) {
		return nil, errors.New("config broken")
	}

	_, err := NewReader(strings.NewReader(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--no-redact")
	assert.Contains(t, err.Error(), "config broken")
}
