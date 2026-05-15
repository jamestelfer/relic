package renderer_test

import (
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/jamestelfer/relic/internal/renderer"
	"github.com/jamestelfer/relic/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_VoidElements_DontConsumeStack(t *testing.T) {
	t.Parallel()
	// If <br> or <img> are pushed onto the tag stack, then </kbd>
	// would either match the void element (wrong) or be treated as
	// an orphan and escaped (visible bug). This test proves void
	// elements are transparent to the stack.
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "<kbd>line one<br>line two</kbd>"},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "<kbd>line one", "kbd opens")
	assert.Contains(t, out, "</kbd>", "kbd closes (not escaped)")
	assert.NotContains(t, out, "&lt;/kbd&gt;", "closing kbd must not be escaped")
	assert.Contains(t, out, "<br>", "br rendered, not escaped")
	assert.NotContains(t, out, "&lt;br&gt;", "br must not be escaped")
}

func TestRender_VoidElements_ImgInsideBlock(t *testing.T) {
	// Verify <img> inside a block-level HTML element doesn't
	// interfere with the block's own closing tag.
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "See:\n\n<div><img src=\"icon.png\" alt=\"icon\">caption</div>\n\nEnd."},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "<div>", "div opens")
	assert.Contains(t, out, "</div>", "div closes")
	assert.Contains(t, out, "caption", "text content preserved")
	assert.NotContains(t, out, "&lt;div&gt;", "div must not be escaped")
}

func TestRender_BlockHTML_ScriptEscaped(t *testing.T) {
	t.Parallel()
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "Check this:\n\n<script>alert(1)</script>\n\nDone."},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "&lt;script&gt;alert(1)&lt;/script&gt;", "block script should be escaped")
	assert.NotContains(t, out, "<script>alert(1)</script>", "no raw script element in output")
	assert.NotContains(t, out, "<!-- raw HTML omitted -->", "no omit comment")
}

func TestRender_BlockHTML_SafeDivRendered(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "Example:\n\n<div>content</div>\n\nEnd."},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "<div>content</div>", "safe div renders as HTML")
	assert.NotContains(t, out, "&lt;div&gt;", "safe div should not be escaped")
	assert.NotContains(t, out, "<!-- raw HTML omitted -->", "no omit comment")
}

func TestRender_MixedHTML_SafeAndUnsafe(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "Use <kbd>Enter</kbd> to continue.\n\n<iframe src=\"evil\"></iframe>\n\nDone."},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "<kbd>Enter</kbd>", "safe kbd rendered as HTML")
	assert.Contains(t, out, "&lt;iframe", "unsafe iframe escaped")
	assert.NotContains(t, out, "<!-- raw HTML omitted -->", "no omit comment")
}

func TestRender_NormalMarkdown_Unaffected(t *testing.T) {
	t.Parallel()
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "**bold** and [link](http://example.com)\n\n```go\nfmt.Println(\"hi\")\n```"},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "<strong>bold</strong>", "bold renders as HTML")
	assert.Contains(t, out, `<a href="http://example.com">link</a>`, "link renders as HTML")
	assert.Contains(t, out, "chroma", "code block gets syntax highlighting")
}

func TestRender_KbdElement_RenderedAsHTML(t *testing.T) {
	t.Parallel()
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "Press <kbd>Ctrl+C</kbd> to cancel."},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "<kbd>Ctrl+C</kbd>", "kbd element rendered as HTML")
	assert.NotContains(t, out, "&lt;kbd&gt;", "kbd should not be escaped")
}

func TestRender_EmptyBluemondayOutput_EscapesOriginal(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "Danger:\n\n<script></script>\n\nEnd."},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "&lt;script&gt;&lt;/script&gt;", "empty script tag is escaped, not empty output")
}

func TestRender_InvalidMarkdown_NoCrash(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "```\nunclosed code block with <script>alert(1)</script>"},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.NotEmpty(t, out, "should render without crashing")
	assert.NotContains(t, out, `<script>alert(1)</script>`, "script in code block must not render as HTML")
}

func TestRender_BreakoutClosingTags_Escaped(t *testing.T) {
	breakouts := []struct {
		name    string
		input   string
		escaped string
	}{
		{"page structure breakout",
			"</p></div></section></article></main></body></html><script>alert('x')</script><html><body><main><article><section><div><p>",
			"&lt;/p&gt;"},
		{"table breakout",
			"</table><script>alert('x')</script><table>",
			"&lt;/table&gt;"},
		{"div breakout",
			"</div><script>alert('x')</script><div>",
			"&lt;/div&gt;"},
	}
	for _, tc := range breakouts {
		t.Run(tc.name, func(t *testing.T) {
			s := session.Session{
				Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
					&session.AssistantText{Text: "Before.\n\n" + tc.input + "\n\nAfter."},
				}}},
			}
			out := render(t, s, renderer.Options{Name: "test"})
			assert.Contains(t, out, "&lt;script&gt;", "script tag must be escaped")
			assert.Contains(t, out, tc.escaped, "orphan closing tags must be escaped")
			assert.Contains(t, out, "After.", "content after breakout still renders")
		})
	}
}

func TestRender_SafeAnchor_RenderedAsHTML(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: `Visit <a href="http://example.com">link</a> now.`},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, `<a href="http://example.com"`, "safe anchor rendered as HTML")
	assert.Contains(t, out, `>link</a>`, "link text and closing tag present")
	assert.NotContains(t, out, "&lt;a href", "safe anchor should not be escaped")
}

func TestRender_JavascriptHref_Stripped(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "Try this:\n\n<a href=\"javascript:alert(1)\">click</a>\n\nDone."},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.NotContains(t, out, `href="javascript:`, "no functional javascript: href in output")
}

func TestRender_OnclickAttr_Stripped(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "Try this:\n\n<button onclick=\"alert(1)\">click</button>\n\nDone."},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.NotContains(t, out, `onclick="`, "no functional onclick attribute in output")
}

func TestRender_GitHubElements_DetailsRendered(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "Info:\n\n<details><summary>Click</summary>Content</details>\n\nEnd."},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "<details>", "details element rendered as HTML")
	assert.Contains(t, out, "<summary>Click</summary>", "summary element rendered as HTML")
}

func TestRender_GitHubElements_MarkSupSub(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "Text with <mark>highlight</mark> and x<sup>2</sup> and H<sub>2</sub>O."},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "<mark>highlight</mark>", "mark element rendered")
	assert.Contains(t, out, "<sup>2</sup>", "sup element rendered")
	assert.Contains(t, out, "<sub>2</sub>", "sub element rendered")
}

func TestRender_HTMLSanitization_Snapshot(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: strings.Join([]string{
				"Inline: press <kbd>Ctrl+C</kbd> to cancel.",
				"",
				"<script>alert('xss')</script>",
				"",
				"<div>safe block div</div>",
				"",
				"<iframe src=\"http://evil.com\"></iframe>",
				"",
				"Link: <a href=\"http://example.com\">safe link</a> here.",
				"",
				"<details><summary>Expand</summary>Detail content</details>",
				"",
				"Safe markdown: **bold** and `code`.",
			}, "\n")},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "snap"})

	bodyStart := strings.Index(out, `class="body"`)
	require.NotEqual(t, bodyStart, -1, "body class marker not found in output")
	bodyEnd := strings.Index(out[bodyStart:], `</div><!-- end body -->`)
	if bodyEnd < 0 {
		bodyEnd = strings.Index(out[bodyStart:], `</main>`)
	}
	require.NotEqual(t, bodyEnd, -1, "body end marker not found in output")
	body := out[bodyStart : bodyStart+bodyEnd]

	snaps.MatchSnapshot(t, body)
}

func TestRender_InlineBreakout_OrphanCloserEscaped(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		escaped string
	}{
		{"span closer", "Text </span> inline.", "&lt;/span&gt;"},
		{"div closer", "Text </div> inline.", "&lt;/div&gt;"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := session.Session{
				Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
					&session.AssistantText{Text: tc.input},
				}}},
			}
			out := render(t, s, renderer.Options{Name: "test"})
			assert.Contains(t, out, tc.escaped, "orphan inline closing tag must be escaped")
		})
	}
}

func TestRender_VoidClosingTag_Dropped(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "Line</br>break."},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.NotContains(t, out, "&lt;/br&gt;", "closing br must not be escaped")
	assert.NotContains(t, out, "</br>", "closing br must not be rendered")
	assert.Contains(t, out, "Line", "text before void closer preserved")
	assert.Contains(t, out, "break.", "text after void closer preserved")
}

func TestRender_UnclosedTag_FlushedAtEnd(t *testing.T) {
	t.Parallel()
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "Open <kbd>text without close"},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "<kbd>text without close", "unclosed tag opens and text renders")
	bodyIdx := strings.Index(out, "<kbd>text without close")
	remaining := out[bodyIdx:]
	assert.Contains(t, remaining, "</kbd>", "flush emits closing tag for unclosed opener")
}

func TestRender_MisnestedAutoRepair(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "<b><i>text</b></i>"},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "</i></b>", "intermediate i auto-closed before b closes")
	assert.Contains(t, out, "&lt;/i&gt;", "orphan </i> is escaped after auto-repair")
}

func TestRender_CrossBlockIsolation(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "Start <kbd>content"},
			&session.AssistantText{Text: "More </kbd> text"},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	assert.Contains(t, out, "&lt;/kbd&gt;", "cross-block closing tag escaped after stack reset")
}

func TestRender_SelfClosingNonVoid_NoSpuriousClose(t *testing.T) {
	s := session.Session{
		Turns: []session.Turn{{Index: 1, Blocks: []session.Block{
			&session.AssistantText{Text: "Text <span/>end"},
		}}},
	}
	out := render(t, s, renderer.Options{Name: "test"})
	bodyStart := strings.Index(out, "Text")
	if bodyStart > 0 {
		bodyEnd := strings.Index(out[bodyStart:], "end")
		if bodyEnd > 0 {
			section := out[bodyStart : bodyStart+bodyEnd+3]
			assert.NotContains(t, section, "</span>", "self-closing span must not produce spurious closing tag in content area")
		}
	}
}
