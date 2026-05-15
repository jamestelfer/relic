package gist

import (
	"fmt"
	"strings"
)

// Runner executes the gh CLI. It is an interface so tests can inject a fake.
type Runner interface {
	Run(args []string, stdin []byte) (stdout, stderr string, err error)
}

// ErrGhFailed is returned when gh exits with a non-zero status.
type ErrGhFailed struct {
	Stderr string
	Code   int
}

func (e ErrGhFailed) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("gh exited with code %d: %s", e.Code, strings.TrimSpace(e.Stderr))
	}
	return fmt.Sprintf("gh exited with code %d", e.Code)
}

// ErrGhNotFound is returned by Runner when gh is not found in PATH.
var ErrGhNotFound = fmt.Errorf("gh CLI not found in PATH — install it and run `gh auth login`")

// publishOptions holds optional configuration for Publish.
type publishOptions struct {
	runner Runner
}

// Option is a functional option for Publish.
type Option func(*publishOptions)

// WithRunner injects a custom gh runner (for testing).
func WithRunner(r Runner) Option {
	return func(o *publishOptions) {
		o.runner = r
	}
}

// Publish renders html as a new GitHub Gist with the given filename.
// If public is true, the gist is created as public; otherwise it is secret.
// Returns the gist URL and a gisthost preview URL.
func Publish(html []byte, filename string, public bool, opts ...Option) (gistURL, previewURL string, err error) {
	cfg := &publishOptions{
		runner: &execRunner{},
	}
	for _, o := range opts {
		o(cfg)
	}

	args := []string{"gist", "create", "--filename", filename}
	if public {
		args = append(args, "--public")
	}
	args = append(args, "-")

	stdout, _, err := cfg.runner.Run(args, html)
	if err != nil {
		return "", "", err
	}

	gistURL = strings.TrimSpace(stdout)

	// Extract the gist ID from the URL: last path segment.
	// Expected format: https://gist.github.com/{user}/{id}
	gistID := gistID(gistURL)
	if gistID == "" {
		return "", "", fmt.Errorf("could not extract gist ID from gh output: %q", gistURL)
	}

	previewURL = fmt.Sprintf("https://gisthost.github.io/?%s/%s", gistID, filename)
	return gistURL, previewURL, nil
}

// gistID extracts the gist ID (last path segment) from a gist URL.
func gistID(gistURL string) string {
	// Strip trailing slash.
	u := strings.TrimRight(gistURL, "/")
	idx := strings.LastIndex(u, "/")
	if idx < 0 || idx == len(u)-1 {
		return ""
	}
	return u[idx+1:]
}
