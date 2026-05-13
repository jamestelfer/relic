package renderer

import (
	"html/template"

	_ "github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark/ast"
	goldmarkRenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

type htmlSanitizer struct{}

func (s *htmlSanitizer) RegisterFuncs(reg goldmarkRenderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHTMLBlock, s.renderHTMLBlock)
	reg.Register(ast.KindRawHTML, s.renderRawHTML)
}

func (s *htmlSanitizer) renderHTMLBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.HTMLBlock)
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		_, _ = w.WriteString(template.HTMLEscapeString(string(line.Value(source))))
	}
	if n.HasClosure() {
		_, _ = w.WriteString(template.HTMLEscapeString(string(n.ClosureLine.Value(source))))
	}
	return ast.WalkContinue, nil
}

func (s *htmlSanitizer) renderRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*ast.RawHTML)
	for i := 0; i < n.Segments.Len(); i++ {
		seg := n.Segments.At(i)
		_, _ = w.WriteString(template.HTMLEscapeString(string(seg.Value(source))))
	}
	return ast.WalkSkipChildren, nil
}
