package gist_test

import (
	"testing"

	"github.com/jamestelfer/relic/internal/gist"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner is a test double for the gh runner.
type fakeRunner struct {
	// inputs captured from the call
	capturedArgs  []string
	capturedStdin []byte
	// configured outputs
	stdout   string
	stderr   string
	exitCode int
}

func (f *fakeRunner) Run(args []string, stdin []byte) (stdout, stderr string, err error) {
	f.capturedArgs = args
	f.capturedStdin = stdin
	if f.exitCode != 0 {
		return f.stdout, f.stderr, gist.ErrGhFailed{Stderr: f.stderr, Code: f.exitCode}
	}
	return f.stdout, f.stderr, nil
}

// TestPublish_GhNonZeroExit verifies that when gh exits non-zero, its stderr is
// surfaced in the returned error (R14).
func TestPublish_GhNonZeroExit(t *testing.T) {
	runner := &fakeRunner{
		stderr:   "authentication required",
		exitCode: 1,
	}

	_, _, err := gist.Publish(
		[]byte("<html>hi</html>"),
		"session.html",
		false,
		gist.WithRunner(runner),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication required",
		"gh stderr must appear in the error message")
}

// TestPublish_GhNotFound verifies that when gh is not found, the error message
// references gh auth login (R13).
func TestPublish_GhNotFound(t *testing.T) {
	// A runner that simulates exec.LookPath failure.
	runner := &notFoundRunner{}

	_, _, err := gist.Publish(
		[]byte("<html>hi</html>"),
		"session.html",
		false,
		gist.WithRunner(runner),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh auth login",
		"error when gh is not found must mention 'gh auth login'")
}

// notFoundRunner simulates gh not being available in PATH.
type notFoundRunner struct{}

func (n *notFoundRunner) Run([]string, []byte) (string, string, error) {
	return "", "", gist.ErrGhNotFound
}

// TestPublish_Public verifies that a public gist includes --public in the gh args (R5).
func TestPublish_Public(t *testing.T) {
	runner := &fakeRunner{
		stdout: "https://gist.github.com/user/abc123def456\n",
	}

	_, _, err := gist.Publish(
		[]byte("<html>hi</html>"),
		"session.html",
		true, // public
		gist.WithRunner(runner),
	)

	require.NoError(t, err)
	assert.Contains(t, runner.capturedArgs, "--public",
		"public gist must pass --public to gh")
}

// TestPublish_Secret verifies that a secret gist is published with the correct
// gh arguments, that the Gist URL is returned, and that the gisthost preview
// URL is constructed correctly (R4, R9, R12).
func TestPublish_Secret(t *testing.T) {
	runner := &fakeRunner{
		stdout: "https://gist.github.com/user/abc123def456\n",
	}

	gistURL, previewURL, err := gist.Publish(
		[]byte("<html>hi</html>"),
		"session.html",
		false,
		gist.WithRunner(runner),
	)

	require.NoError(t, err)

	// Gist URL returned as-is from gh output.
	assert.Equal(t, "https://gist.github.com/user/abc123def456", gistURL)

	// Preview URL uses gist ID extracted from gistURL.
	assert.Equal(t, "https://gisthost.github.io/?abc123def456/session.html", previewURL)

	// gh invoked with correct arguments: no --public flag for secret gist.
	assert.Equal(t, []string{"gist", "create", "--filename", "session.html", "-"}, runner.capturedArgs)

	// HTML piped to stdin.
	assert.Equal(t, []byte("<html>hi</html>"), runner.capturedStdin)
}
