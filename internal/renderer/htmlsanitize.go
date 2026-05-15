package renderer

import (
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark/ast"
	goldmarkRenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
	"golang.org/x/net/html"
)

// --- Tag classification ---

type tagKind int

const (
	tagOpen tagKind = iota
	tagClose
	tagSelfClose
	tagUnknown
)

type tagEvent struct {
	name string
	kind tagKind
}

// HTML void elements can't have content or closing tags. Tracking them
// on the balance stack would create phantom entries that block legitimate
// closers from matching their openers.
var voidElements = map[string]struct{}{
	"area": {}, "base": {}, "br": {}, "col": {},
	"embed": {}, "hr": {}, "img": {}, "input": {},
	"link": {}, "meta": {}, "param": {}, "source": {},
	"track": {}, "wbr": {},
}

func isVoid(name string) bool {
	_, ok := voidElements[name]
	return ok
}

func parseTag(raw string) (string, tagKind) {
	z := html.NewTokenizer(strings.NewReader(raw))
	switch z.Next() {
	case html.StartTagToken:
		tn, _ := z.TagName()
		return string(tn), tagOpen
	case html.EndTagToken:
		tn, _ := z.TagName()
		return string(tn), tagClose
	case html.SelfClosingTagToken:
		tn, _ := z.TagName()
		return string(tn), tagSelfClose
	}
	return "", tagUnknown
}

// tokenizeTags extracts tag events from an HTML fragment, filtering
// out void and self-closing tags which don't participate in balance
// tracking.
func tokenizeTags(s string) []tagEvent {
	z := html.NewTokenizer(strings.NewReader(s))
	var events []tagEvent
	for {
		switch z.Next() {
		case html.ErrorToken:
			return events
		case html.StartTagToken:
			tn, _ := z.TagName()
			name := string(tn)
			if isVoid(name) {
				continue
			}
			events = append(events, tagEvent{name, tagOpen})
		case html.EndTagToken:
			tn, _ := z.TagName()
			name := string(tn)
			if isVoid(name) {
				continue
			}
			events = append(events, tagEvent{name, tagClose})
		}
	}
}

// --- Tag balance tracking ---

// tagStack prevents layout breakout by tracking the balance of open HTML
// tags. Unclosed openers get force-closed at end of rendering; orphan
// closers (no matching opener) get escaped so they can't close tags in
// the surrounding page structure.
type tagStack struct {
	tags []string
}

func (s *tagStack) push(name string) {
	s.tags = append(s.tags, name)
}

// popTo unwinds the stack to the first matching opener. Returns the
// names of any intermediate tags above the match so the caller can
// decide how to handle them (auto-close for inline, N/A for block).
// Stack is unchanged if no match is found.
func (s *tagStack) popTo(name string) (intermediates []string, found bool) {
	for i := len(s.tags) - 1; i >= 0; i-- {
		if s.tags[i] == name {
			intermediates = s.tags[i+1:]
			s.tags = s.tags[:i]
			return intermediates, true
		}
	}
	return nil, false
}

func (s *tagStack) checkpoint() int {
	return len(s.tags)
}

func (s *tagStack) restore(cp int) {
	s.tags = s.tags[:cp]
}

// unclosed returns currently open tags in close order (innermost first).
func (s *tagStack) unclosed() []string {
	if len(s.tags) == 0 {
		return nil
	}
	result := make([]string, len(s.tags))
	for i, j := 0, len(s.tags)-1; j >= 0; i, j = i+1, j-1 {
		result[i] = s.tags[j]
	}
	return result
}

// tryApply applies a sequence of tag events to the stack. Rolls back
// on any orphan closer — a closer with no matching opener would escape
// the rendered fragment and close tags in the surrounding page.
func (s *tagStack) tryApply(events []tagEvent) bool {
	cp := s.checkpoint()
	for _, e := range events {
		switch e.kind {
		case tagOpen:
			s.push(e.name)
		case tagClose:
			_, found := s.popTo(e.name)
			if !found {
				s.restore(cp)
				return false
			}
		}
	}
	return true
}

// --- Sanitization ---

var sanitizePolicy = func() *bluemonday.Policy {
	// This policy has a bunch of regular expressions internally, so we only want
	// to construct it once.
	p := bluemonday.UGCPolicy()
	p.AllowElements("kbd")
	return p
}()

// sanitize returns false when bluemonday strips everything — the caller
// should escape the original rather than silently dropping content the
// author intended to display.
func sanitize(raw string) (string, bool) {
	out := sanitizePolicy.Sanitize(raw)
	if strings.TrimSpace(out) == "" {
		return "", false
	}
	return out, true
}

func escape(w util.BufWriter, raw string) {
	_, _ = w.WriteString(template.HTMLEscapeString(raw))
}

// --- Goldmark node renderer ---

type htmlSanitizer struct {
	stack tagStack
}

func (s *htmlSanitizer) RegisterFuncs(reg goldmarkRenderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHTMLBlock, s.renderHTMLBlock)
	reg.Register(ast.KindRawHTML, s.renderRawHTML)
}

func (s *htmlSanitizer) closeUnclosed(w io.Writer) {
	for _, tag := range s.stack.unclosed() {
		_, _ = fmt.Fprintf(w, "</%s>", tag)
	}
}

// Goldmark delivers inline HTML one tag at a time. Misnested pairs
// (e.g. <b><i></b></i>) are repaired by auto-closing intermediates
// when popTo skips past them.
func (s *htmlSanitizer) renderRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}

	raw := collectSegments(node.(*ast.RawHTML), source)
	name, kind := parseTag(raw)

	if kind == tagClose && isVoid(name) {
		return ast.WalkSkipChildren, nil
	}

	sanitized, ok := sanitize(raw)
	if !ok {
		escape(w, raw)
		return ast.WalkSkipChildren, nil
	}

	switch {
	case isVoid(name) || kind == tagSelfClose || kind == tagUnknown:
		_, _ = w.WriteString(sanitized)

	case kind == tagOpen:
		s.stack.push(name)
		_, _ = w.WriteString(sanitized)

	case kind == tagClose:
		intermediates, found := s.stack.popTo(name)
		if !found {
			escape(w, raw)
			return ast.WalkSkipChildren, nil
		}
		for i := len(intermediates) - 1; i >= 0; i-- {
			_, _ = w.WriteString("</" + intermediates[i] + ">")
		}
		_, _ = w.WriteString(sanitized)
	}

	return ast.WalkSkipChildren, nil
}

// Goldmark delivers block HTML as a unit. The whole block is trial-applied
// to the stack — if any closer is orphaned, the entire block is escaped
// to prevent breakout.
func (s *htmlSanitizer) renderHTMLBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	raw := collectBlockLines(node.(*ast.HTMLBlock), source)

	sanitized, ok := sanitize(raw)
	if !ok {
		escape(w, raw)
		return ast.WalkContinue, nil
	}

	if !s.stack.tryApply(tokenizeTags(sanitized)) {
		escape(w, raw)
		return ast.WalkContinue, nil
	}

	_, _ = w.WriteString(sanitized)
	return ast.WalkContinue, nil
}

// --- Segment helpers ---

func collectSegments(n *ast.RawHTML, source []byte) string {
	var b strings.Builder
	for i := 0; i < n.Segments.Len(); i++ {
		seg := n.Segments.At(i)
		b.Write(seg.Value(source))
	}
	return b.String()
}

func collectBlockLines(n *ast.HTMLBlock, source []byte) string {
	var b strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		b.Write(line.Value(source))
	}
	if n.HasClosure() {
		b.Write(n.ClosureLine.Value(source))
	}
	return b.String()
}
