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

// AgentEnrichment holds structured data from a completed Agent tool result.
type AgentEnrichment struct {
	AgentType    string
	Status       string
	DurationMs   int
	TokenCount   int
	ToolUseCount int
}

func (e *AgentEnrichment) enrichmentType() string { return "agent" }

// AskUserQuestionEnrichment holds structured data from an AskUserQuestion
// tool result, carrying questions with their selected answers and annotations.
type AskUserQuestionEnrichment struct {
	Questions []AskQuestionResult
}

func (e *AskUserQuestionEnrichment) enrichmentType() string { return "askuserquestion" }

// AskQuestionResult holds a single question with its options, selected
// answers, and optional free-text annotation.
type AskQuestionResult struct {
	Header   string
	Question string
	Options  []string
	Selected []string
	Freetext string
}

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
		if e := interpretBash(m); e != nil {
			return e
		}
	case "Read":
		if e := interpretRead(m, callInput); e != nil {
			return e
		}
	case "Write":
		if e := interpretWrite(m); e != nil {
			return e
		}
	case "Edit":
		if e := interpretEdit(m); e != nil {
			return e
		}
	case "Grep":
		if e := interpretGrep(m); e != nil {
			return e
		}
	case "Glob":
		if e := interpretGlob(m); e != nil {
			return e
		}
	case "Agent":
		if e := interpretAgent(m); e != nil {
			return e
		}
	case "AskUserQuestion":
		if e := interpretAskUserQuestion(m); e != nil {
			return e
		}
	}
	return nil
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

func interpretAgent(m map[string]any) *AgentEnrichment {
	status, _ := m["status"].(string)
	if status != "completed" {
		return nil
	}
	agentType, _ := m["agentType"].(string)
	return &AgentEnrichment{
		AgentType:    agentType,
		Status:       status,
		DurationMs:   intFromAny(m["totalDurationMs"]),
		TokenCount:   intFromAny(m["totalTokens"]),
		ToolUseCount: intFromAny(m["totalToolUseCount"]),
	}
}

func interpretAskUserQuestion(m map[string]any) *AskUserQuestionEnrichment {
	questions, ok := m["questions"].([]any)
	if !ok || len(questions) == 0 {
		return nil
	}

	answers, _ := m["answers"].(map[string]any)
	annotations, _ := m["annotations"].(map[string]any)

	results := make([]AskQuestionResult, 0, len(questions))
	for _, q := range questions {
		qm, ok := q.(map[string]any)
		if !ok {
			continue
		}
		questionText, _ := qm["question"].(string)
		header, _ := qm["header"].(string)

		var optionLabels []string
		if opts, ok := qm["options"].([]any); ok {
			for _, o := range opts {
				if om, ok := o.(map[string]any); ok {
					if l, ok := om["label"].(string); ok {
						optionLabels = append(optionLabels, l)
					}
				}
			}
		}

		var selected []string
		switch v := answers[questionText].(type) {
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok && s != "" {
					selected = append(selected, s)
				}
			}
		case string:
			if v != "" {
				selected = []string{v}
			}
		}

		var freetext string
		if ann, ok := annotations[questionText].(map[string]any); ok {
			freetext, _ = ann["notes"].(string)
		}

		results = append(results, AskQuestionResult{
			Header:   header,
			Question: questionText,
			Options:  optionLabels,
			Selected: selected,
			Freetext: freetext,
		})
	}

	if len(results) == 0 {
		return nil
	}
	return &AskUserQuestionEnrichment{Questions: results}
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
