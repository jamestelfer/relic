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

// buildProjectTree creates a temporary directory tree with multiple project
// directories containing .jsonl files at controlled modification times.
//
//	root/
//	  .claude/projects/
//	    -Users-dev-src-alpha/
//	      aaa.jsonl  (mtime: 2025-01-01)
//	    -Users-dev-src-beta/
//	      bbb.jsonl  (mtime: 2025-06-01)
//	      ccc.jsonl  (mtime: 2025-03-01)
//	    -Users-dev-src-gamma/
//	      ddd.jsonl  (mtime: 2025-12-01)
//	    -Users-dev-src-empty/
//	      notes.txt  (not .jsonl — this dir should be excluded)
func buildProjectTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	base := filepath.Join(root, ".claude", "projects")

	files := []struct {
		rel   string
		mtime time.Time
	}{
		{"-Users-dev-src-alpha/aaa.jsonl", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"-Users-dev-src-beta/bbb.jsonl", time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)},
		{"-Users-dev-src-beta/ccc.jsonl", time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)},
		{"-Users-dev-src-gamma/ddd.jsonl", time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)},
		{"-Users-dev-src-empty/notes.txt", time.Date(2025, 12, 2, 0, 0, 0, 0, time.UTC)},
	}

	for _, f := range files {
		full := filepath.Join(base, f.rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte("{}"), 0o644))
		require.NoError(t, os.Chtimes(full, f.mtime, f.mtime))
	}
	return root
}

func TestDiscoverSessions(t *testing.T) {
	root := buildProjectTree(t)
	betaDir := filepath.Join(root, ".claude", "projects", "-Users-dev-src-beta")

	sessions, err := picker.DiscoverSessions(betaDir)
	require.NoError(t, err)
	require.Len(t, sessions, 2)

	// Sorted most-recent first: bbb (June) before ccc (March).
	assert.Contains(t, sessions[0].Path, "bbb.jsonl")
	assert.Contains(t, sessions[1].Path, "ccc.jsonl")
	assert.True(t, sessions[0].ModTime.After(sessions[1].ModTime),
		"sessions must be sorted newest first")

	// Paths should be absolute.
	for _, s := range sessions {
		assert.True(t, filepath.IsAbs(s.Path), "Path should be absolute: %s", s.Path)
	}
}

func TestDiscoverProjects(t *testing.T) {
	root := buildProjectTree(t)

	projects, err := picker.DiscoverProjects(root)
	require.NoError(t, err)

	// 3 project dirs have .jsonl files; the "empty" dir only has .txt
	require.Len(t, projects, 3)

	// All Dirs should be absolute paths.
	for _, p := range projects {
		assert.True(t, filepath.IsAbs(p.Dir), "Dir should be absolute: %s", p.Dir)
	}

	// The "empty" dir with only .txt should be excluded.
	for _, p := range projects {
		assert.NotContains(t, p.Dir, "empty", "dir with no .jsonl should be excluded")
	}
}

func TestDiscoverProjectsSessionCount(t *testing.T) {
	root := buildProjectTree(t)

	projects, err := picker.DiscoverProjects(root)
	require.NoError(t, err)

	counts := make(map[string]int)
	for _, p := range projects {
		counts[filepath.Base(p.Dir)] = p.SessionCount
	}

	assert.Equal(t, 1, counts["-Users-dev-src-alpha"])
	assert.Equal(t, 2, counts["-Users-dev-src-beta"])
	assert.Equal(t, 1, counts["-Users-dev-src-gamma"])
}

func TestDiscoverProjectsMostRecentModTime(t *testing.T) {
	root := buildProjectTree(t)

	projects, err := picker.DiscoverProjects(root)
	require.NoError(t, err)

	times := make(map[string]time.Time)
	for _, p := range projects {
		times[filepath.Base(p.Dir)] = p.MostRecentModTime
	}

	// Beta has two files: bbb (June) and ccc (March). Most recent should be June.
	assert.True(t, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC).Equal(times["-Users-dev-src-beta"]),
		"beta most recent should be June, got %v", times["-Users-dev-src-beta"])
	assert.True(t, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Equal(times["-Users-dev-src-alpha"]),
		"alpha most recent should be January, got %v", times["-Users-dev-src-alpha"])
	assert.True(t, time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC).Equal(times["-Users-dev-src-gamma"]),
		"gamma most recent should be December, got %v", times["-Users-dev-src-gamma"])
}

func TestDiscoverProjectsNoProjects(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".claude", "projects"), 0o755))

	_, err := picker.DiscoverProjects(root)
	require.Error(t, err)
	assert.ErrorIs(t, err, picker.ErrNoProjects)
	assert.Contains(t, err.Error(), ".claude/projects")
}

func TestDiscoverProjectsMissingDir(t *testing.T) {
	root := t.TempDir()

	_, err := picker.DiscoverProjects(root)
	require.Error(t, err)
	assert.ErrorIs(t, err, picker.ErrNoProjects)
}

func TestDiscoverProjectsSingleSession(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, ".claude", "projects", "-Users-dev-src-solo")
	require.NoError(t, os.MkdirAll(base, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(base, "only.jsonl"), []byte("{}"), 0o644))

	projects, err := picker.DiscoverProjects(root)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, 1, projects[0].SessionCount)
}

func TestDecodePath(t *testing.T) {
	cases := []struct {
		name    string
		homeDir string
		encoded string
		want    string
	}{
		{
			name:    "home prefix replaced with ~/",
			homeDir: "/Users/dev",
			encoded: "-Users-dev-src-oss-relic",
			want:    "~/src-oss-relic",
		},
		{
			name:    "dot in username encoded as dash",
			homeDir: "/Users/james.telfer",
			encoded: "-Users-james-telfer-src-oss-relic",
			want:    "~/src-oss-relic",
		},
		{
			name:    "double dash becomes separator",
			homeDir: "/Users/james.telfer",
			encoded: "-Users-james-telfer-src--fiddle-agent-teams",
			want:    "~/src · fiddle-agent-teams",
		},
		{
			name:    "path not under home returned as-is",
			homeDir: "/Users/dev",
			encoded: "-var-data-project",
			want:    "-var-data-project",
		},
		{
			name:    "worktree-style path with double dash",
			homeDir: "/Users/dev",
			encoded: "-Users-dev-src-oss-relic--claude-worktrees-abc",
			want:    "~/src-oss-relic · claude-worktrees-abc",
		},
		{
			name:    "empty string",
			homeDir: "/Users/dev",
			encoded: "",
			want:    "",
		},
		{
			name:    "root home dir",
			homeDir: "/",
			encoded: "-src-project",
			want:    "-src-project",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := picker.DecodePath(c.homeDir, c.encoded)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		t    time.Time
		want string
	}{
		{
			name: "seconds ago",
			t:    now.Add(-30 * time.Second),
			want: "30s ago",
		},
		{
			name: "minutes ago",
			t:    now.Add(-45 * time.Minute),
			want: "45m ago",
		},
		{
			name: "hours ago",
			t:    now.Add(-3 * time.Hour),
			want: "3h ago",
		},
		{
			name: "days ago",
			t:    now.Add(-5 * 24 * time.Hour),
			want: "5d ago",
		},
		{
			name: "29 days ago (still relative)",
			t:    now.Add(-29 * 24 * time.Hour),
			want: "29d ago",
		},
		{
			name: "30 days ago (absolute)",
			t:    now.Add(-30 * 24 * time.Hour),
			want: "May 16",
		},
		{
			name: "months ago (absolute)",
			t:    time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			want: "Jan 15",
		},
		{
			name: "different year",
			t:    time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC),
			want: "Mar 5",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := picker.RelativeTime(now, c.t)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestDiscoverProjectsSortedByMostRecent(t *testing.T) {
	root := buildProjectTree(t)

	projects, err := picker.DiscoverProjects(root)
	require.NoError(t, err)
	require.Len(t, projects, 3)

	// gamma (Dec) > beta (Jun) > alpha (Jan)
	assert.Contains(t, projects[0].Dir, "gamma")
	assert.Contains(t, projects[1].Dir, "beta")
	assert.Contains(t, projects[2].Dir, "alpha")
}

func TestFormatSize(t *testing.T) {
	cases := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"one KiB", 1024, "1.0 KiB"},
		{"fractional KiB", 1536, "1.5 KiB"},
		{"large KiB", 102400, "100.0 KiB"},
		{"one MiB", 1048576, "1.0 MiB"},
		{"fractional MiB", 2097152, "2.0 MiB"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := picker.FormatSize(c.bytes)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestDiscoverSessionsIncludesSize(t *testing.T) {
	root := buildProjectTree(t)
	betaDir := filepath.Join(root, ".claude", "projects", "-Users-dev-src-beta")

	sessions, err := picker.DiscoverSessions(betaDir)
	require.NoError(t, err)
	require.Len(t, sessions, 2)

	// Fixture files contain "{}" (2 bytes).
	for _, s := range sessions {
		assert.Equal(t, int64(2), s.Size, "session %s should have size 2", s.Path)
	}
}

func TestDiscoverSessionsExcludesNonJsonl(t *testing.T) {
	root := buildProjectTree(t)
	emptyDir := filepath.Join(root, ".claude", "projects", "-Users-dev-src-empty")

	sessions, err := picker.DiscoverSessions(emptyDir)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}
