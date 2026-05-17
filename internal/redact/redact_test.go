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
	testGitHubPAT      = "ghp_" + "zR8k4mVq2xN7pLw9cJ3hYf6eDgA5tB0sQiUo"
	testAWSKey         = "AKIA" + "Z7V4Q2XRNJ3WBTY5"
	testNpmToken       = "NpmToken." + "f00df00d-f00d-f00d-f00d-f00df00df00d"
	testBuildkiteAPI   = "bkua_" + "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6ab"
	testBuildkiteAgent = "bkaa_" + "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d"
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

func TestReader_RedactsBuildkiteTokens(t *testing.T) {
	tests := []struct {
		name   string
		token  string
		ruleID string
	}{
		{"api token", testBuildkiteAPI, "buildkite-api-token"},
		{"agent session token", testBuildkiteAgent, "buildkite-agent-session-token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := `{"type":"assistant","message":{"role":"assistant","content":"token: ` + tt.token + `"}}` + "\n"

			r, err := NewReader(strings.NewReader(line))
			require.NoError(t, err)

			out, err := io.ReadAll(r)
			require.NoError(t, err)

			result := string(out)
			assert.NotContains(t, result, tt.token)
			assert.Contains(t, result, "[REDACTED:"+tt.ruleID+"]")
		})
	}
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

func TestReader_SummaryTracksLineNumbers(t *testing.T) {
	lines := strings.Join([]string{
		`{"type":"user","message":{"role":"user","content":"hello"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":"token: ` + testGitHubPAT + `"}}`,
		`{"type":"user","message":{"role":"user","content":"thanks"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":"token: ` + testGitHubPAT + `"}}`,
	}, "\n") + "\n"

	r, err := NewReader(strings.NewReader(lines))
	require.NoError(t, err)

	_, err = io.ReadAll(r)
	require.NoError(t, err)

	summary := r.Summary()
	assert.Equal(t, 1, summary.SecretCount)
	assert.Equal(t, []LineFinding{
		{Line: 2, Rules: []RuleCount{{RuleID: "github-pat", Count: 1}}},
		{Line: 4, Rules: []RuleCount{{RuleID: "github-pat", Count: 1}}},
	}, summary.Lines)
}

func TestReader_SummaryMultipleSecretsOnOneLine(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","content":"token: ` + testGitHubPAT + ` key: ` + testAWSKey + `"}}` + "\n"

	r, err := NewReader(strings.NewReader(line))
	require.NoError(t, err)

	_, err = io.ReadAll(r)
	require.NoError(t, err)

	summary := r.Summary()
	assert.Equal(t, 2, summary.SecretCount)
	require.Len(t, summary.Lines, 1)
	assert.Equal(t, 1, summary.Lines[0].Line)
	assert.Equal(t, []RuleCount{
		{RuleID: "aws-access-token", Count: 1},
		{RuleID: "github-pat", Count: 1},
	}, summary.Lines[0].Rules)
}

func TestReader_SummaryIsEmptyForNoSecrets(t *testing.T) {
	line := `{"type":"user","message":{"role":"user","content":"hello"}}` + "\n"

	r, err := NewReader(strings.NewReader(line))
	require.NoError(t, err)

	_, err = io.ReadAll(r)
	require.NoError(t, err)

	summary := r.Summary()
	assert.Equal(t, 0, summary.SecretCount)
	assert.Empty(t, summary.Lines)
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
	for _, lf := range summary.Lines {
		for _, rc := range lf.Rules {
			assert.NotContains(t, rc.RuleID, testGitHubPAT)
			assert.NotContains(t, rc.RuleID, testAWSKey)
		}
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
