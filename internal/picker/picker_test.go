package picker_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jamestelfer/relic/internal/picker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildFixtureTree creates a temporary directory tree of .jsonl files with
// controlled modification times and returns the root directory.
//
//	root/
//	  .claude/projects/
//	    alpha/session.jsonl     (mtime: oldest)
//	    beta/session.jsonl      (mtime: middle)
//	    gamma/recent.jsonl      (mtime: newest)
//	    gamma/ignored.txt       (not .jsonl, must be excluded)
func buildFixtureTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	base := filepath.Join(root, ".claude", "projects")

	files := []struct {
		rel   string
		mtime time.Time
	}{
		{"alpha/session.jsonl", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"beta/session.jsonl", time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)},
		{"gamma/recent.jsonl", time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)},
		{"gamma/ignored.txt", time.Date(2025, 12, 2, 0, 0, 0, 0, time.UTC)},
	}

	for _, f := range files {
		full := filepath.Join(base, f.rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte("{}"), 0o644))
		require.NoError(t, os.Chtimes(full, f.mtime, f.mtime))
	}
	return root
}

// TestDiscover verifies that Discover finds .jsonl files sorted most-recent-first
// and excludes non-.jsonl files.
func TestDiscover(t *testing.T) {
	root := buildFixtureTree(t)

	entries, err := picker.Discover(root)
	require.NoError(t, err)

	// 3 .jsonl files, no .txt
	require.Len(t, entries, 3)

	// Most recent first.
	assert.True(t, entries[0].ModTime.After(entries[1].ModTime), "entries must be sorted newest first")
	assert.True(t, entries[1].ModTime.After(entries[2].ModTime), "entries must be sorted newest first")

	// Paths should be absolute.
	for _, e := range entries {
		assert.True(t, filepath.IsAbs(e.Path), "path should be absolute: %s", e.Path)
	}

	// Non-.jsonl file must be excluded.
	for _, e := range entries {
		assert.Equal(t, ".jsonl", filepath.Ext(e.Path), "only .jsonl files expected")
	}
}

// TestDiscoverEmpty verifies that Discover returns an error when no .jsonl files exist.
func TestDiscoverEmpty(t *testing.T) {
	root := t.TempDir()
	// Create the directory but no files.
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".claude", "projects"), 0o755))

	_, err := picker.Discover(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no session files found")
}

// TestRelativePath verifies that RelativePath trims the home-dir prefix correctly.
func TestRelativePath(t *testing.T) {
	home := "/home/user"
	cases := []struct {
		path string
		want string
	}{
		{"/home/user/.claude/projects/foo/bar.jsonl", "~/.claude/projects/foo/bar.jsonl"},
		{"/other/path/file.jsonl", "/other/path/file.jsonl"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, picker.RelativePath(home, c.path))
	}
}
