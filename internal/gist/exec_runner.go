package gist

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// execRunner is the production Runner that invokes the real gh CLI.
type execRunner struct{}

func (e *execRunner) Run(args []string, stdin []byte) (stdout, stderr string, err error) {
	ghPath, lookErr := exec.LookPath("gh")
	if lookErr != nil {
		return "", "", ErrGhNotFound
	}

	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(ghPath, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return stdout, stderr, ErrGhFailed{
				Stderr: strings.TrimSpace(stderr),
				Code:   exitErr.ExitCode(),
			}
		}
		return stdout, stderr, fmt.Errorf("running gh: %w", runErr)
	}

	return stdout, stderr, nil
}
