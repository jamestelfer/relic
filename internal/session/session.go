package session

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jamestelfer/relic/internal/ansi"
	"github.com/jamestelfer/relic/internal/parser"
)

// Block is the common interface for all session content blocks.
// The unexported sealedBlock method prevents external implementations.
type Block interface {
	BlockType() string
	sealedBlock()
}

// UserText is a text block from a user message.
type UserText struct {
	Text    string `json:"text"`
	LineNum int    `json:"line_num"`
}

func (b *UserText) BlockType() string { return "user_text" }
func (b *UserText) sealedBlock()      {}

// AssistantText is a text block from an assistant message (raw markdown).
type AssistantText struct {
	Text    string `json:"text"`
	LineNum int    `json:"line_num"`
}

func (b *AssistantText) BlockType() string { return "assistant_text" }
func (b *AssistantText) sealedBlock()      {}

// ToolCall is a tool invocation from an assistant message.
type ToolCall struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Input          map[string]any `json:"input"`
	LinkedResultID *string        `json:"linked_result_id,omitempty"`
	LineNum        int            `json:"line_num"`
}

func (b *ToolCall) BlockType() string { return "tool_call" }
func (b *ToolCall) sealedBlock()      {}

// Arg returns a single arg string for display in the tool card head.
// Known tools map to a specific Input field; unknown tools fall back to the
// alphabetically-first string-valued field in Input, or "" when none exists.
// No meta string is produced — diff stats, line counts, exit codes, and
// durations are out of scope for this phase.
func (b *ToolCall) Arg() string {
	switch b.Name {
	case "Bash":
		return inputString(b.Input, "description")
	case "Read", "Write", "Edit", "MultiEdit":
		return inputString(b.Input, "file_path")
	case "Grep":
		return inputString(b.Input, "pattern")
	case "WebSearch":
		return inputString(b.Input, "query")
	case "TodoWrite":
		if todos, ok := b.Input["todos"].([]any); ok {
			return fmt.Sprintf("%d todos", len(todos))
		}
		return ""
	}
	return defaultArgFromInput(b.Input)
}

// inputString reads a string field from the input map, returning "" when the
// key is absent or the value is not a string.
func inputString(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

// defaultArgFromInput returns the alphabetically-first string-valued field's
// value, or "" when the map has no string fields.
func defaultArgFromInput(m map[string]any) string {
	var keys []string
	for k, v := range m {
		if _, ok := v.(string); ok {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	return m[keys[0]].(string)
}

// ToolResult is tool output from a user message.
type ToolResult struct {
	ToolUseID      string  `json:"tool_use_id"`
	Content        string  `json:"content"`
	LinkedCallID   *string `json:"linked_call_id,omitempty"`
	LinkedCallName string  `json:"linked_call_name,omitempty"`
	LineNum        int     `json:"line_num"`
}

func (b *ToolResult) BlockType() string { return "tool_result" }
func (b *ToolResult) sealedBlock()      {}

// LocalCommand is a slash-command invocation (e.g. `/focus`) lifted from a
// top-level system record with subtype == "local_command".
type LocalCommand struct {
	Name    string `json:"name"`
	Message string `json:"message,omitempty"`
	Args    string `json:"args,omitempty"`
	LineNum int    `json:"line_num"`
}

func (b *LocalCommand) BlockType() string { return "local_command" }
func (b *LocalCommand) sealedBlock()      {}

// System is a generic system-origin block (away summary, system reminder, etc).
// Source identifies the origin: "system_record" for top-level system records,
// "meta_user" for isMeta:true user messages (Phase 6b).
type System struct {
	Subtype string `json:"subtype,omitempty"`
	Content string `json:"content"`
	Source  string `json:"source"`
	LineNum int    `json:"line_num"`
}

func (b *System) BlockType() string { return "system" }
func (b *System) sealedBlock()      {}

// Thinking is an internal reasoning block from an assistant message.
type Thinking struct {
	Text    string `json:"thinking"`
	LineNum int    `json:"line_num"`
}

func (b *Thinking) BlockType() string { return "thinking" }
func (b *Thinking) sealedBlock()      {}

// RedactedThinking is a redacted internal reasoning block.
type RedactedThinking struct {
	Data    string `json:"data"`
	LineNum int    `json:"line_num"`
}

func (b *RedactedThinking) BlockType() string { return "redacted_thinking" }
func (b *RedactedThinking) sealedBlock()      {}

// CompactionBoundary is a visual separator marking where context was compacted.
type CompactionBoundary struct {
	Label   string `json:"label"`
	LineNum int    `json:"line_num"`
}

func (b *CompactionBoundary) BlockType() string { return "compaction_boundary" }
func (b *CompactionBoundary) sealedBlock()      {}

// CompactionSummary is a collapsible block showing compacted context.
type CompactionSummary struct {
	Content string `json:"content"`
	LineNum int    `json:"line_num"`
}

func (b *CompactionSummary) BlockType() string { return "compaction_summary" }
func (b *CompactionSummary) sealedBlock()      {}

// ApiError is an API error message surfaced to the user.
type ApiError struct {
	ErrorType string `json:"error_type"`
	Message   string `json:"message"`
	LineNum   int    `json:"line_num"`
}

func (b *ApiError) BlockType() string { return "api_error" }
func (b *ApiError) sealedBlock()      {}

// HookInjection is a hook-injected system note (pre/post tool use hooks etc).
type HookInjection struct {
	Content string `json:"content"`
	LineNum int    `json:"line_num"`
}

func (b *HookInjection) BlockType() string { return "hook_injection" }
func (b *HookInjection) sealedBlock()      {}

// RequestInterrupted is a synthetic user message injected when the user
// interrupts an in-progress assistant response.
type RequestInterrupted struct {
	Detail  string `json:"detail"`
	LineNum int    `json:"line_num"`
}

func (b *RequestInterrupted) BlockType() string { return "request_interrupted" }
func (b *RequestInterrupted) sealedBlock()      {}

// SessionResult records the outcome of a session (success or error).
type SessionResult struct {
	Subtype    string  `json:"subtype"`
	DurationMS int     `json:"duration_ms"`
	NumTurns   int     `json:"num_turns"`
	CostUSD    float64 `json:"cost_usd"`
	Error      string  `json:"error"`
	LineNum    int     `json:"line_num"`
}

func (b *SessionResult) BlockType() string { return "session_result" }
func (b *SessionResult) sealedBlock()      {}

// Error is a parse-failure callout.
type Error struct {
	LineNum int    `json:"line_num"`
	Raw     []byte `json:"raw"`
	Msg     string `json:"msg"`
}

func (b *Error) BlockType() string { return "error" }
func (b *Error) sealedBlock()      {}

// Raw holds an unrecognised block type, preserved verbatim.
type Raw struct {
	RawType string `json:"raw_type"`
	Data    []byte `json:"data"`
	LineNum int    `json:"line_num"`
}

func (b *Raw) BlockType() string { return b.RawType }
func (b *Raw) sealedBlock()      {}

// UserBashInput is a user-initiated shell command (! prefix in Claude Code).
type UserBashInput struct {
	Command        string  `json:"command"`
	ID             string  `json:"id"`
	LinkedResultID *string `json:"linked_result_id,omitempty"`
	LineNum        int     `json:"line_num"`
}

func (b *UserBashInput) BlockType() string { return "user_bash_input" }
func (b *UserBashInput) sealedBlock()      {}

// UserBashResult is the output of a user-initiated shell command.
type UserBashResult struct {
	Stdout       string  `json:"stdout"`
	Stderr       string  `json:"stderr"`
	ID           string  `json:"id"`
	LinkedCallID *string `json:"linked_call_id,omitempty"`
	LineNum      int     `json:"line_num"`
}

func (b *UserBashResult) BlockType() string { return "user_bash_result" }
func (b *UserBashResult) sealedBlock()      {}

// Image holds an inline image (base64 data URI).
type Image struct {
	MediaType string `json:"media_type"`
	Base64    string `json:"base64"`
	LineNum   int    `json:"line_num"`
}

func (b *Image) BlockType() string { return "image" }
func (b *Image) sealedBlock()      {}

// Turn groups one user-initiated exchange with all subsequent assistant messages
// up to the next user turn boundary.
type Turn struct {
	Index  int        `json:"index"`
	Title  string     `json:"title"`
	Blocks []Block    `json:"blocks"`
	Start  *time.Time `json:"start,omitempty"`
}

// RelativeOffset returns the duration between this turn's Start and prev's Start.
// Returns 0 when either timestamp is missing.
func (t Turn) RelativeOffset(prev *Turn) time.Duration {
	if prev == nil || t.Start == nil || prev.Start == nil {
		return 0
	}
	return t.Start.Sub(*prev.Start)
}

// SessionHeader provides metadata for the page chrome.
type SessionHeader struct {
	Model        string
	Cwd          string
	GitBranch    string
	Version      string
	Title        string
	StartTime    *time.Time
	Duration     time.Duration
	TurnCount    int
	FilesTouched int
}

// Session is the top-level representation of a Claude Code session.
type Session struct {
	Name          string        `json:"name"`
	FilePath      string        `json:"file_path"`
	Start         *time.Time    `json:"start,omitempty"`
	Turns         []Turn        `json:"turns"`
	Model         string        `json:"model,omitempty"`
	DurationNanos time.Duration `json:"duration_nanos,omitempty"`
	FilesTouched  []string      `json:"files_touched,omitempty"`
	EmbeddedName  string        `json:"embedded_name,omitempty"`
}

// Header returns a SessionHeader computed from the session's metadata.
func (s Session) Header() SessionHeader {
	return SessionHeader{
		Model:        s.Model,
		Title:        s.EmbeddedName,
		StartTime:    s.Start,
		Duration:     s.DurationNanos,
		TurnCount:    len(s.Turns),
		FilesTouched: len(s.FilesTouched),
	}
}

// Transform converts a parser.Result into a Session.
//
// Turn boundaries: a new turn begins only when a user message contains at
// least one TextBlock. User messages containing only tool_result blocks stay
// in the current turn. Assistant messages that appear before any user turn are
// collected into a synthetic turn 0 — this handles sessions whose first
// recorded message is an assistant reply.
//
// Session.EmbeddedName is set from the last CustomTitleRecord in source order;
// empty when the result carries none.
func Transform(res parser.Result) Session {
	msgs := res.Messages
	sys := res.SystemRecords
	var turns []Turn
	var current *Turn
	var preamble []Block // blocks before the first TextBlock-user-message

	var start *time.Time
	var last *time.Time
	var model string
	filesSeen := make(map[string]struct{})
	var filesOrder []string
	recordFile := func(path string) {
		if path == "" {
			return
		}
		if _, ok := filesSeen[path]; ok {
			return
		}
		filesSeen[path] = struct{}{}
		filesOrder = append(filesOrder, path)
	}

	appendCurrent := func(bs []Block) {
		if len(bs) == 0 {
			return
		}
		if current != nil {
			current.Blocks = append(current.Blocks, bs...)
		} else {
			preamble = append(preamble, bs...)
		}
	}

	mi, si := 0, 0
	for mi < len(msgs) || si < len(sys) {
		// Pick next by LineNum; messages win on tie (stable).
		takeSys := mi >= len(msgs) || (si < len(sys) && sys[si].LineNum < msgs[mi].LineNum)
		if takeSys {
			rec := sys[si]
			si++
			if b := systemRecordToBlock(rec); b != nil {
				appendCurrent([]Block{b})
			}
			continue
		}

		msg := msgs[mi]
		mi++

		if msg.Envelope.IsSidechain {
			continue
		}

		if start == nil && msg.Timestamp != nil {
			t := *msg.Timestamp
			start = &t
		}
		if msg.Timestamp != nil {
			t := *msg.Timestamp
			last = &t
		}
		if model == "" && msg.Model != "" {
			model = msg.Model
		}
		for _, c := range msg.Content {
			if tu, ok := c.(*parser.ToolUseBlock); ok {
				switch tu.Name {
				case "Read", "Write", "Edit", "MultiEdit":
					if p, ok := tu.Input["file_path"].(string); ok {
						recordFile(p)
					}
				}
			}
		}

		blocks := messageToBlocks(msg)

		switch msg.Role {
		case "user":
			if classified := classifyUserMessage(msg); classified != nil {
				appendCurrent(classified)
			} else if hasTextBlock(msg) {
				if current != nil {
					turns = append(turns, *current)
				}
				var turnStart *time.Time
				if msg.Timestamp != nil {
					t := *msg.Timestamp
					turnStart = &t
				}
				current = &Turn{
					Index:  len(turns) + 1,
					Title:  extractTitle(msg),
					Blocks: append(preamble, blocks...),
					Start:  turnStart,
				}
				preamble = nil
			} else {
				// Tool-result-only message: part of the agent loop, not a new turn.
				appendCurrent(blocks)
			}
		case "error":
			appendCurrent(blocks)
		default: // "assistant" and anything else
			appendCurrent(blocks)
		}
	}

	// Append ResultRecords as SessionResult blocks to the current turn.
	for _, rr := range res.ResultRecords {
		block := &SessionResult{
			Subtype:    rr.Subtype,
			DurationMS: rr.DurationMS,
			NumTurns:   rr.NumTurns,
			CostUSD:    rr.CostUSD,
			Error:      rr.Error,
			LineNum:    rr.LineNum,
		}
		appendCurrent([]Block{block})
	}

	if current != nil {
		turns = append(turns, *current)
	} else if len(preamble) > 0 {
		// All messages were non-TextBlock-user: fold into a synthetic turn.
		turns = append(turns, Turn{Index: 1, Blocks: preamble})
	}

	for i := range turns {
		crossReferenceTools(&turns[i])
	}

	var duration time.Duration
	if start != nil && last != nil {
		duration = last.Sub(*start)
	}

	var embeddedName string
	if n := len(res.CustomTitles); n > 0 {
		embeddedName = res.CustomTitles[n-1].Text
	}

	return Session{
		Start:         start,
		Turns:         turns,
		Model:         model,
		DurationNanos: duration,
		FilesTouched:  filesOrder,
		EmbeddedName:  embeddedName,
	}
}

// hasTextBlock reports whether a message contains at least one TextBlock.
func hasTextBlock(msg parser.Message) bool {
	for _, c := range msg.Content {
		if _, ok := c.(*parser.TextBlock); ok {
			return true
		}
	}
	return false
}

// extractTitle returns the first line of the first TextBlock in msg, truncated
// to 80 characters. Returns an empty string if no TextBlock is present.
func extractTitle(msg parser.Message) string {
	for _, c := range msg.Content {
		if tb, ok := c.(*parser.TextBlock); ok {
			line := tb.Text
			if i := strings.IndexByte(line, '\n'); i >= 0 {
				line = line[:i]
			}
			if len(line) > 80 {
				line = line[:80]
			}
			return line
		}
	}
	return ""
}

// localCommandNameRE, localCommandMessageRE, and localCommandArgsRE pull
// pseudo-XML fields out of a local_command system record's content. The
// content is not well-formed XML (arbitrary whitespace, no root element,
// occasional unescaped HTML-looking text), so a tolerant regex pass is the
// pragmatic choice.
var (
	localCommandNameRE    = regexp.MustCompile(`(?s)<command-name>(.*?)</command-name>`)
	localCommandMessageRE = regexp.MustCompile(`(?s)<command-message>(.*?)</command-message>`)
	localCommandArgsRE    = regexp.MustCompile(`(?s)<command-args>(.*?)</command-args>`)
)

// systemRecordToBlock routes a SystemRecord into the right session Block.
// Returns nil for subtypes that should produce no output (turn_duration).
func systemRecordToBlock(rec parser.SystemRecord) Block {
	switch rec.Subtype {
	case "turn_duration":
		return nil
	case "local_command":
		return parseLocalCommand(rec)
	case "compact_boundary":
		return &CompactionBoundary{
			Label:   rec.Content,
			LineNum: rec.LineNum,
		}
	default:
		return &System{
			Subtype: rec.Subtype,
			Content: rec.Content,
			Source:  "system_record",
			LineNum: rec.LineNum,
		}
	}
}

var localCommandStdoutRE = regexp.MustCompile(`(?s)<local-command-stdout>(.*?)</local-command-stdout>`)

var localCommandCaveatRE = regexp.MustCompile(`(?s)<local-command-caveat>`)

var requestInterruptedRE = regexp.MustCompile(`^\[Request interrupted by user(.*?)\]$`)

var (
	bashInputRE  = regexp.MustCompile(`(?s)<bash-input>(.*?)</bash-input>`)
	bashStdoutRE = regexp.MustCompile(`(?s)<bash-stdout>(.*?)</bash-stdout>`)
	bashStderrRE = regexp.MustCompile(`(?s)<bash-stderr>(.*?)</bash-stderr>`)
)

// classifyUserMessage routes a user message per the spec's detection cascade:
//  1. Envelope flags: isApiErrorMessage, isCompactSummary
//  2. Content patterns: <command-name>, <local-command-stdout>, <local-command-caveat>
//  3. isMeta fallback: hook injection
//
// Returns nil when the message is a real user prompt (no classification matched).
func classifyUserMessage(msg parser.Message) []Block {
	var parts []string
	for _, c := range msg.Content {
		if tb, ok := c.(*parser.TextBlock); ok {
			parts = append(parts, tb.Text)
		}
	}
	content := strings.Join(parts, "\n\n")

	// 1. Envelope flags take priority.
	if msg.Envelope.IsApiErrorMessage {
		return []Block{apiErrorFromContent(content, msg.LineNum)}
	}
	if msg.Envelope.IsCompactSummary {
		return []Block{&CompactionSummary{
			Content: content,
			LineNum: msg.LineNum,
		}}
	}

	// 2. Content-pattern detection (independent of isMeta).

	// User bash (! command) detection — before slash-command checks.
	if m := bashInputRE.FindStringSubmatch(content); m != nil {
		return []Block{&UserBashInput{
			Command: strings.TrimSpace(m[1]),
			ID:      fmt.Sprintf("ubash-in-%d", msg.LineNum),
			LineNum: msg.LineNum,
		}}
	}
	if bashStdoutRE.MatchString(content) || bashStderrRE.MatchString(content) {
		var stdout, stderr string
		if m := bashStdoutRE.FindStringSubmatch(content); m != nil {
			stdout = m[1]
		}
		if m := bashStderrRE.FindStringSubmatch(content); m != nil {
			stderr = m[1]
		}
		return []Block{&UserBashResult{
			Stdout:  stdout,
			Stderr:  stderr,
			ID:      fmt.Sprintf("ubash-out-%d", msg.LineNum),
			LineNum: msg.LineNum,
		}}
	}

	if localCommandNameRE.MatchString(content) {
		lc := &LocalCommand{LineNum: msg.LineNum}
		nameM := localCommandNameRE.FindStringSubmatch(content)
		msgM := localCommandMessageRE.FindStringSubmatch(content)
		argsM := localCommandArgsRE.FindStringSubmatch(content)
		if nameM != nil {
			lc.Name = strings.TrimPrefix(strings.TrimSpace(nameM[1]), "/")
		}
		if msgM != nil {
			lc.Message = strings.TrimSpace(msgM[1])
		}
		if argsM != nil {
			lc.Args = strings.TrimSpace(argsM[1])
		}
		return []Block{lc}
	}
	if m := localCommandStdoutRE.FindStringSubmatch(content); m != nil {
		return []Block{&System{
			Source:  "meta_user",
			Subtype: "command_output",
			Content: ansi.Strip(strings.TrimSpace(m[1])),
			LineNum: msg.LineNum,
		}}
	}
	if localCommandCaveatRE.MatchString(content) {
		return []Block{}
	}

	// 3. Synthetic messages injected by the harness.
	if m := requestInterruptedRE.FindStringSubmatch(strings.TrimSpace(content)); m != nil {
		return []Block{&RequestInterrupted{
			Detail:  "by user" + m[1],
			LineNum: msg.LineNum,
		}}
	}

	// 4. isMeta fallback: unrecognized meta content is a hook injection.
	if msg.IsMeta {
		return []Block{&HookInjection{
			Content: content,
			LineNum: msg.LineNum,
		}}
	}

	// Not classified — caller treats as a real user prompt.
	return nil
}

// apiErrorFromContent extracts error type and message from content text.
// Expected format: "error_type: message" or just the full message.
func apiErrorFromContent(content string, lineNum int) *ApiError {
	ae := &ApiError{LineNum: lineNum, Message: content}
	if idx := strings.Index(content, ": "); idx > 0 {
		ae.ErrorType = content[:idx]
		ae.Message = content[idx+2:]
	}
	return ae
}

// parseLocalCommand extracts a Block from a system record with subtype
// "local_command". The content may be a slash command invocation (with
// <command-name> XML), a command stdout result (with <local-command-stdout>),
// or unstructured text. Returns the appropriate block type for each case.
func parseLocalCommand(rec parser.SystemRecord) Block {
	if m := localCommandStdoutRE.FindStringSubmatch(rec.Content); m != nil {
		return &System{
			Source:  "system_record",
			Subtype: "command_output",
			Content: ansi.Strip(strings.TrimSpace(m[1])),
			LineNum: rec.LineNum,
		}
	}

	lc := &LocalCommand{LineNum: rec.LineNum}
	nameM := localCommandNameRE.FindStringSubmatch(rec.Content)
	msgM := localCommandMessageRE.FindStringSubmatch(rec.Content)
	argsM := localCommandArgsRE.FindStringSubmatch(rec.Content)
	if nameM == nil && msgM == nil && argsM == nil {
		lc.Message = rec.Content
		return lc
	}
	if nameM != nil {
		lc.Name = strings.TrimPrefix(strings.TrimSpace(nameM[1]), "/")
	}
	if msgM != nil {
		lc.Message = strings.TrimSpace(msgM[1])
	}
	if argsM != nil {
		lc.Args = strings.TrimSpace(argsM[1])
	}
	return lc
}

// messageToBlocks converts a parser.Message into the appropriate session Block types.
func messageToBlocks(msg parser.Message) []Block {
	ln := msg.LineNum
	blocks := make([]Block, 0, len(msg.Content))
	for _, content := range msg.Content {
		var block Block
		switch c := content.(type) {
		case *parser.TextBlock:
			if msg.Role == "user" {
				block = &UserText{Text: c.Text, LineNum: ln}
			} else {
				block = &AssistantText{Text: c.Text, LineNum: ln}
			}
		case *parser.ToolUseBlock:
			block = &ToolCall{ID: c.ID, Name: c.Name, Input: c.Input, LineNum: ln}
		case *parser.ToolResultBlock:
			block = &ToolResult{ToolUseID: c.ToolUseID, Content: c.Content, LineNum: ln}
		case *parser.ThinkingBlock:
			block = &Thinking{Text: c.Thinking, LineNum: ln}
		case *parser.RedactedThinkingBlock:
			block = &RedactedThinking{Data: c.Data, LineNum: ln}
		case *parser.ImageBlock:
			block = &Image{MediaType: c.MediaType, Base64: c.Base64, LineNum: ln}
		case *parser.ErrorBlock:
			block = &Error{LineNum: c.LineNum, Raw: c.Raw, Msg: c.Msg}
		case *parser.RawBlock:
			block = &Raw{RawType: c.RawType, Data: []byte(c.Data), LineNum: ln}
		default:
			block = &Raw{RawType: content.BlockType(), LineNum: ln}
		}
		blocks = append(blocks, block)
	}
	return blocks
}

// crossReferenceTools sets LinkedResultID on ToolCall blocks and LinkedCallID
// on ToolResult blocks for matched pairs within a single turn. It also pairs
// adjacent UserBashInput/UserBashResult blocks.
func crossReferenceTools(turn *Turn) {
	// Map tool_use_id to block index for calls and results.
	callByID := make(map[string]*ToolCall)
	resultByID := make(map[string]*ToolResult)

	for _, block := range turn.Blocks {
		switch b := block.(type) {
		case *ToolCall:
			callByID[b.ID] = b
		case *ToolResult:
			resultByID[b.ToolUseID] = b
		}
	}

	for _, block := range turn.Blocks {
		switch b := block.(type) {
		case *ToolCall:
			if _, hasResult := resultByID[b.ID]; hasResult {
				id := b.ID
				b.LinkedResultID = &id
			}
		case *ToolResult:
			if call, hasCall := callByID[b.ToolUseID]; hasCall {
				id := b.ToolUseID
				b.LinkedCallID = &id
				b.LinkedCallName = call.Name
			}
		}
	}

	// Pair adjacent UserBashInput → UserBashResult blocks.
	for i := 0; i < len(turn.Blocks)-1; i++ {
		input, ok := turn.Blocks[i].(*UserBashInput)
		if !ok {
			continue
		}
		result, ok := turn.Blocks[i+1].(*UserBashResult)
		if !ok {
			continue
		}
		input.LinkedResultID = &result.ID
		result.LinkedCallID = &input.ID
	}
}
