package ansi_test

import (
	"testing"

	"github.com/jamestelfer/relic/internal/ansi"
	"github.com/stretchr/testify/assert"
)

// TestConvertANSIColour verifies that a string with an ANSI colour escape
// produces HTML with a <span style="..."> element.
func TestConvertANSIColour(t *testing.T) {
	// ANSI red: ESC[31m ... ESC[0m
	input := "\x1b[31mHello\x1b[0m"
	out := string(ansi.Convert(input))

	assert.Contains(t, out, "<span")
	assert.NotContains(t, out, "\x1b", "ANSI escape byte must be absent in converted output")
}

// TestConvertPlainText verifies that plain text passes through without modification.
func TestConvertPlainText(t *testing.T) {
	input := "plain text output"
	out := string(ansi.Convert(input))
	assert.Contains(t, out, "plain text output")
}
