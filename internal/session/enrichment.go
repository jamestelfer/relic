package session

// ToolEnrichment is the sealed interface for tool-specific enrichment data
// attached to ToolResult blocks. Concrete types provide structured metadata
// extracted from the JSONL toolUseResult field.
type ToolEnrichment interface {
	enrichmentType() string
}

// BashEnrichment holds structured data from a Bash tool result.
type BashEnrichment struct {
	Stdout      string
	Stderr      string
	Interrupted bool
}

func (e *BashEnrichment) enrichmentType() string { return "bash" }

// ReadEnrichment holds structured data from a Read tool result.
type ReadEnrichment struct {
	FilePath   string
	LineStart  int
	LineCount  int
	TotalLines int
}

func (e *ReadEnrichment) enrichmentType() string { return "read" }

// interpretEnrichment interprets a raw toolUseResult value based on the tool
// name and produces a typed ToolEnrichment value. Returns nil when the raw value
// is not a map (string/array/nil produce no enrichment) or the tool name is
// unrecognised.
func interpretEnrichment(toolName string, raw any) ToolEnrichment {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}

	switch toolName {
	case "Bash":
		return interpretBash(m)
	case "Read":
		return interpretRead(m)
	default:
		return nil
	}
}

func interpretRead(m map[string]any) *ReadEnrichment {
	file, ok := m["file"].(map[string]any)
	if !ok {
		return nil
	}
	filePath, _ := file["filePath"].(string)
	if filePath == "" {
		return nil
	}
	return &ReadEnrichment{
		FilePath:   filePath,
		LineStart:  intFromAny(file["startLine"]),
		LineCount:  intFromAny(file["numLines"]),
		TotalLines: intFromAny(file["totalLines"]),
	}
}

func intFromAny(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

func interpretBash(m map[string]any) *BashEnrichment {
	stdout, _ := m["stdout"].(string)
	stderr, _ := m["stderr"].(string)
	interrupted, _ := m["interrupted"].(bool)
	return &BashEnrichment{
		Stdout:      stdout,
		Stderr:      stderr,
		Interrupted: interrupted,
	}
}
