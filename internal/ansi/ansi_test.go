package ansi_test

import (
	"strings"
	"testing"

	"github.com/jamestelfer/relic/internal/ansi"
)

// TestConvertANSIColour verifies that a string with an ANSI colour escape
// produces HTML with a <span style="..."> element.
func TestConvertANSIColour(t *testing.T) {
	// ANSI red: ESC[31m ... ESC[0m
	input := "\x1b[31mHello\x1b[0m"
	out := string(ansi.Convert(input))

	if !strings.Contains(out, "<span") {
		t.Errorf("expected <span> in output, got: %s", out)
	}
	if strings.Contains(out, "\x1b") {
		t.Error("expected ANSI escape byte to be absent in converted output")
	}
}

// TestConvertPlainText verifies that plain text passes through without modification.
func TestConvertPlainText(t *testing.T) {
	input := "plain text output"
	out := string(ansi.Convert(input))

	if !strings.Contains(out, "plain text output") {
		t.Errorf("expected plain text in output, got: %s", out)
	}
}
