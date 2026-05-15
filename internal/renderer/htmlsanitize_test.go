package renderer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTag(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantName string
		wantKind tagKind
	}{
		{"opening tag", "<div>", "div", tagOpen},
		{"closing tag", "</div>", "div", tagClose},
		{"self-closing", "<br/>", "br", tagSelfClose},
		{"tag with attrs", `<a href="http://example.com">`, "a", tagOpen},
		{"empty input", "", "", tagUnknown},
		{"plain text", "hello", "", tagUnknown},
		{"html comment", "<!-- comment -->", "", tagUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name, kind := parseTag(tc.raw)
			assert.Equal(t, tc.wantName, name)
			assert.Equal(t, tc.wantKind, kind)
		})
	}
}

func TestTokenizeTags(t *testing.T) {
	tests := []struct {
		name string
		html string
		want []tagEvent
	}{
		{"balanced div",
			"<div>text</div>",
			[]tagEvent{{"div", tagOpen}, {"div", tagClose}}},
		{"nested",
			"<div><span>x</span></div>",
			[]tagEvent{{"div", tagOpen}, {"span", tagOpen}, {"span", tagClose}, {"div", tagClose}}},
		{"void elements filtered out",
			"<div><br><img src='x'></div>",
			[]tagEvent{{"div", tagOpen}, {"div", tagClose}}},
		{"self-closing filtered out",
			"<div><span/>text</div>",
			[]tagEvent{{"div", tagOpen}, {"div", tagClose}}},
		{"empty input", "", nil},
		{"only void elements", "<br><hr>", nil},
		{"orphan closer preserved",
			"</div><span>text</span>",
			[]tagEvent{{"div", tagClose}, {"span", tagOpen}, {"span", tagClose}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tokenizeTags(tc.html)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestIsVoid(t *testing.T) {
	assert.True(t, isVoid("br"))
	assert.True(t, isVoid("img"))
	assert.False(t, isVoid("div"))
	assert.False(t, isVoid("span"))
	assert.False(t, isVoid(""))
}

func TestTagStack_PushAndPopTo(t *testing.T) {
	var s tagStack
	s.push("div")
	s.push("span")
	s.push("b")

	intermediates, found := s.popTo("div")
	require.True(t, found)
	assert.Equal(t, []string{"span", "b"}, intermediates)
	assert.Empty(t, s.unclosed())
}

func TestTagStack_PopTo_NotFound(t *testing.T) {
	var s tagStack
	s.push("div")
	s.push("span")

	intermediates, found := s.popTo("b")
	assert.False(t, found)
	assert.Nil(t, intermediates)
	assert.Equal(t, []string{"span", "div"}, s.unclosed(), "stack unchanged on failed pop")
}

func TestTagStack_PopTo_Innermost(t *testing.T) {
	var s tagStack
	s.push("div")
	s.push("span")

	intermediates, found := s.popTo("span")
	require.True(t, found)
	assert.Empty(t, intermediates, "no intermediates when popping innermost")
	assert.Equal(t, []string{"div"}, s.unclosed())
}

func TestTagStack_CheckpointRestore(t *testing.T) {
	var s tagStack
	s.push("div")
	cp := s.checkpoint()
	s.push("span")
	s.push("b")

	s.restore(cp)
	assert.Equal(t, []string{"div"}, s.unclosed())
}

func TestTagStack_Unclosed(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want []string
	}{
		{"empty stack", nil, nil},
		{"single tag", []string{"div"}, []string{"div"}},
		{"multiple tags innermost first",
			[]string{"div", "span", "b"},
			[]string{"b", "span", "div"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var s tagStack
			for _, tag := range tc.tags {
				s.push(tag)
			}
			assert.Equal(t, tc.want, s.unclosed())
		})
	}
}

func TestTagStack_TryApply(t *testing.T) {
	tests := []struct {
		name         string
		initial      []string
		events       []tagEvent
		wantOK       bool
		wantUnclosed []string
	}{
		{"balanced block",
			nil,
			[]tagEvent{{"div", tagOpen}, {"div", tagClose}},
			true, nil},
		{"opens without closing",
			nil,
			[]tagEvent{{"div", tagOpen}},
			true, []string{"div"}},
		{"closes opener from before",
			[]string{"div"},
			[]tagEvent{{"div", tagClose}},
			true, nil},
		{"orphan closer rejects and rolls back",
			nil,
			[]tagEvent{{"div", tagOpen}, {"span", tagClose}},
			false, nil},
		{"orphan closer preserves pre-existing stack",
			[]string{"section"},
			[]tagEvent{{"div", tagOpen}, {"span", tagClose}},
			false, []string{"section"}},
		{"nested balanced",
			nil,
			[]tagEvent{{"div", tagOpen}, {"span", tagOpen}, {"span", tagClose}, {"div", tagClose}},
			true, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var s tagStack
			for _, tag := range tc.initial {
				s.push(tag)
			}
			ok := s.tryApply(tc.events)
			assert.Equal(t, tc.wantOK, ok)
			if len(tc.wantUnclosed) == 0 {
				assert.Empty(t, s.unclosed())
			} else {
				assert.Equal(t, tc.wantUnclosed, s.unclosed())
			}
		})
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOutput string
		wantOK     bool
	}{
		{"safe element passes", "<kbd>text</kbd>", "<kbd>text</kbd>", true},
		{"safe block passes", "<div>content</div>", "<div>content</div>", true},
		{"unsafe stripped completely", "<script>alert(1)</script>", "", false},
		{"safe anchor with attrs",
			`<a href="http://example.com">link</a>`,
			`<a href="http://example.com" rel="nofollow">link</a>`, true},
		{"unsafe attr stripped but element survives",
			`<div onclick="alert(1)">text</div>`, "<div>text</div>", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, ok := sanitize(tc.input)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantOutput, out)
		})
	}
}
