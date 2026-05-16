package redact_test

import (
	"io"
	"strings"
	"testing"

	"github.com/jamestelfer/relic/internal/redact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testGitHubPAT = "ghp_zR8k4mVq2xN7pLw9cJ3hYf6eDgA5tB0sQiUo"
	testAWSKey    = "AKIAZ7V4Q2XRNJ3WBTY5"
)

func TestReader_RedactsGitHubPAT(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","content":"token: ` + testGitHubPAT + `"}}` + "\n"

	r, err := redact.NewReader(strings.NewReader(line))
	require.NoError(t, err)

	out, err := io.ReadAll(r)
	require.NoError(t, err)

	result := string(out)
	assert.NotContains(t, result, testGitHubPAT)
	assert.Contains(t, result, "[REDACTED:github-pat]")
}

func TestReader_RedactsAWSAccessKey(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","content":"key: ` + testAWSKey + `"}}` + "\n"

	r, err := redact.NewReader(strings.NewReader(line))
	require.NoError(t, err)

	out, err := io.ReadAll(r)
	require.NoError(t, err)

	result := string(out)
	assert.NotContains(t, result, testAWSKey)
	assert.Contains(t, result, "[REDACTED:aws-access-token]")
}

func TestReader_PassesThroughNonSecretContent(t *testing.T) {
	line := `{"type":"user","message":{"role":"user","content":"hello world"}}` + "\n"

	r, err := redact.NewReader(strings.NewReader(line))
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

	r, err := redact.NewReader(strings.NewReader(lines))
	require.NoError(t, err)

	out, err := io.ReadAll(r)
	require.NoError(t, err)

	outLines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	assert.Len(t, outLines, 3)
}

func TestReader_SummaryDeduplicatesSameSecret(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","content":"token: ` + testGitHubPAT + `"}}` + "\n"
	input := line + line + line

	r, err := redact.NewReader(strings.NewReader(input))
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

	r, err := redact.NewReader(strings.NewReader(line))
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

	r, err := redact.NewReader(strings.NewReader(lines))
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
