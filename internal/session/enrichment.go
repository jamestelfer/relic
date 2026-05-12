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
	FilePath  string
	LineStart int
	LineEnd   int
}

func (e *ReadEnrichment) enrichmentType() string { return "read" }

// WriteEnrichment holds structured data from a Write tool result.
type WriteEnrichment struct {
	FilePath string
	Action   string // "created" or "updated"
}

func (e *WriteEnrichment) enrichmentType() string { return "write" }

// EditEnrichment holds structured data from an Edit tool result.
type EditEnrichment struct {
	FilePath string
	Action   string // always "updated"
}

func (e *EditEnrichment) enrichmentType() string { return "edit" }

// GrepEnrichment holds structured data from a Grep tool result.
type GrepEnrichment struct {
	Mode       string // "content", "files_with_matches", or "count"
	NumFiles   int
	NumLines   int
	NumMatches int
}

func (e *GrepEnrichment) enrichmentType() string { return "grep" }

// GlobEnrichment holds structured data from a Glob tool result.
type GlobEnrichment struct {
	NumFiles  int
	Truncated bool
}

func (e *GlobEnrichment) enrichmentType() string { return "glob" }

// interpretEnrichment interprets a raw toolUseResult value based on the tool
// name and produces a typed ToolEnrichment value. Returns nil when the raw value
// is not a map (string/array/nil produce no enrichment) or the tool name is
// unrecognised.
func interpretEnrichment(toolName string, raw any, callInput map[string]any) ToolEnrichment {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}

	switch toolName {
	case "Bash":
		return interpretBash(m)
	case "Read":
		return interpretRead(m, callInput)
	case "Write":
		return interpretWrite(m)
	case "Edit":
		return interpretEdit(m)
	case "Grep":
		return interpretGrep(m)
	case "Glob":
		return interpretGlob(m)
	default:
		return nil
	}
}

func interpretRead(m map[string]any, callInput map[string]any) *ReadEnrichment {
	file, ok := m["file"].(map[string]any)
	if !ok {
		return nil
	}
	filePath, _ := file["filePath"].(string)
	if filePath == "" {
		return nil
	}
	e := &ReadEnrichment{FilePath: filePath}
	offset := intFromAny(callInput["offset"])
	limit := intFromAny(callInput["limit"])
	if offset > 0 && limit > 0 {
		e.LineStart = offset
		e.LineEnd = offset + limit - 1
	}
	return e
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

func interpretWrite(m map[string]any) *WriteEnrichment {
	filePath, _ := m["filePath"].(string)
	if filePath == "" {
		return nil
	}
	var action string
	switch m["type"] {
	case "create":
		action = "created"
	case "update":
		action = "updated"
	}
	return &WriteEnrichment{FilePath: filePath, Action: action}
}

func interpretEdit(m map[string]any) *EditEnrichment {
	filePath, _ := m["filePath"].(string)
	if filePath == "" {
		return nil
	}
	return &EditEnrichment{FilePath: filePath, Action: "updated"}
}

func interpretGlob(m map[string]any) *GlobEnrichment {
	numFiles := intFromAny(m["numFiles"])
	if numFiles == 0 {
		return nil
	}
	truncated, _ := m["truncated"].(bool)
	return &GlobEnrichment{NumFiles: numFiles, Truncated: truncated}
}

func interpretGrep(m map[string]any) *GrepEnrichment {
	mode, _ := m["mode"].(string)
	if mode == "" {
		return nil
	}
	return &GrepEnrichment{
		Mode:       mode,
		NumFiles:   intFromAny(m["numFiles"]),
		NumLines:   intFromAny(m["numLines"]),
		NumMatches: intFromAny(m["numMatches"]),
	}
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
