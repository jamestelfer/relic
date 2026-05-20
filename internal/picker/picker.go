package picker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// ProjectEntry represents a discovered project directory containing sessions.
type ProjectEntry struct {
	Dir               string
	MostRecentModTime time.Time
	SessionCount      int
}

// SessionEntry represents a discovered session file.
type SessionEntry struct {
	Path    string
	ModTime time.Time
	Size    int64
}

// ErrAborted is returned by Pick when the user cancels the selection (Ctrl-C / Esc).
var ErrAborted = errors.New("picker aborted")

// ErrNoProjects is returned when no project directories with .jsonl files are found.
var ErrNoProjects = errors.New("no Claude Code projects found")

// RelativeTime formats the duration between now and t as a human-readable
// relative string for times less than 30 days old, or as an absolute date
// otherwise.
func RelativeTime(now, t time.Time) string {
	d := now.Sub(t)
	future := d < 0
	if future {
		d = -d
	}
	if d < 30*24*time.Hour {
		var value string
		switch {
		case d < time.Minute:
			value = fmt.Sprintf("%ds", int(d.Seconds()))
		case d < time.Hour:
			value = fmt.Sprintf("%dm", int(d.Minutes()))
		case d < 24*time.Hour:
			value = fmt.Sprintf("%dh", int(d.Hours()))
		default:
			value = fmt.Sprintf("%dd", int(d.Hours()/24))
		}
		if future {
			return "in " + value
		}
		return value + " ago"
	}
	return t.Format("Jan 2")
}

// FormatSize formats a byte count as a human-readable string using binary
// units (B, KiB, MiB).
func FormatSize(bytes int64) string {
	switch {
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// DecodePath strips the encoded home directory prefix from an encoded project
// directory name and replaces it with "~/". The remainder of the path is left
// encoded. If the encoded name does not start with the home prefix, it is
// returned unchanged.
func DecodePath(homeDir, encoded string) string {
	if encoded == "" {
		return ""
	}
	// Claude Code encodes paths by replacing both "/" and "." with "-".
	encodedHome := strings.NewReplacer(
		string(filepath.Separator), "-",
		".", "-",
	).Replace(homeDir)
	prefix := encodedHome + "-"
	if !strings.HasPrefix(encoded, prefix) {
		return encoded
	}
	remainder := encoded[len(prefix):]
	remainder = strings.ReplaceAll(remainder, "--", " · ")
	return "~/" + remainder
}

// DiscoverProjects scans homeDir/.claude/projects for project directories
// that contain at least one .jsonl file.
func DiscoverProjects(homeDir string) ([]ProjectEntry, error) {
	projectsDir := filepath.Join(homeDir, ".claude", "projects")

	dirEntries, err := os.ReadDir(projectsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w in %s", ErrNoProjects, projectsDir)
		}
		return nil, fmt.Errorf("read %s: %w", projectsDir, err)
	}

	var projects []ProjectEntry
	for _, d := range dirEntries {
		if !d.IsDir() {
			continue
		}

		projDir := filepath.Join(projectsDir, d.Name())
		sessions, err := DiscoverSessions(projDir)
		if err != nil {
			return nil, err
		}
		if len(sessions) == 0 {
			continue
		}

		projects = append(projects, ProjectEntry{
			Dir:               projDir,
			MostRecentModTime: sessions[0].ModTime,
			SessionCount:      len(sessions),
		})
	}

	if len(projects) == 0 {
		return nil, fmt.Errorf("%w in %s", ErrNoProjects, projectsDir)
	}

	slices.SortFunc(projects, func(a, b ProjectEntry) int {
		if a.MostRecentModTime.After(b.MostRecentModTime) {
			return -1
		}
		if a.MostRecentModTime.Before(b.MostRecentModTime) {
			return 1
		}
		return 0
	})

	return projects, nil
}

// DiscoverSessions scans projectDir for .jsonl files, returning them sorted
// by modification time (most recent first). Returns an empty slice if no
// .jsonl files are found.
func DiscoverSessions(projectDir string) ([]SessionEntry, error) {
	dirEntries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", projectDir, err)
	}

	var sessions []SessionEntry
	for _, d := range dirEntries {
		if d.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".jsonl") {
			continue
		}
		info, err := d.Info()
		if err != nil {
			continue
		}
		sessions = append(sessions, SessionEntry{
			Path:    filepath.Join(projectDir, d.Name()),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		})
	}

	slices.SortFunc(sessions, func(a, b SessionEntry) int {
		if a.ModTime.After(b.ModTime) {
			return -1
		}
		if a.ModTime.Before(b.ModTime) {
			return 1
		}
		return 0
	})

	return sessions, nil
}

// Pick presents a two-step interactive selection: first choose a project
// directory, then choose a session file within it. Returns the absolute path
// of the chosen .jsonl file. If the user aborts (Ctrl-C / Esc) at either step,
// ErrAborted is returned.
func Pick(homeDir string) (string, error) {
	projects, err := DiscoverProjects(homeDir)
	if err != nil {
		return "", err
	}

	// Step 1: project selection
	now := time.Now()
	theme := pickerTheme()
	muted := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "242", Dark: "246"})

	projectOptions := make([]huh.Option[string], len(projects))
	for i, p := range projects {
		name := DecodePath(homeDir, filepath.Base(p.Dir))
		meta := muted.Render(fmt.Sprintf("%d sessions  %s", p.SessionCount, RelativeTime(now, p.MostRecentModTime)))
		label := name + "  " + meta
		projectOptions[i] = huh.NewOption(label, p.Dir)
	}

	var chosenProject string
	projectForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a project").
				Options(projectOptions...).
				Value(&chosenProject),
		),
	).WithTheme(theme)

	if err := projectForm.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", ErrAborted
		}
		return "", err
	}

	// Step 2: session selection
	sessions, err := DiscoverSessions(chosenProject)
	if err != nil {
		return "", err
	}
	if len(sessions) == 0 {
		return "", fmt.Errorf("no sessions found in %s", chosenProject)
	}

	sessionOptions := make([]huh.Option[string], len(sessions))
	for i, s := range sessions {
		name := filepath.Base(s.Path)
		meta := muted.Render(FormatSize(s.Size) + "  " + RelativeTime(now, s.ModTime))
		label := name + "  " + meta
		sessionOptions[i] = huh.NewOption(label, s.Path)
	}

	var chosenSession string
	sessionForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a session").
				Options(sessionOptions...).
				Value(&chosenSession),
		),
	).WithTheme(theme)

	if err := sessionForm.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", ErrAborted
		}
		return "", err
	}

	return chosenSession, nil
}

func pickerTheme() *huh.Theme {
	t := huh.ThemeCharm()

	chrome := lipgloss.AdaptiveColor{Light: "242", Dark: "246"}

	t.Focused.Base = t.Focused.Base.BorderForeground(chrome)
	t.Focused.Card = t.Focused.Base

	t.Help.ShortKey = t.Help.ShortKey.Foreground(chrome)
	t.Help.ShortDesc = t.Help.ShortDesc.Foreground(chrome)
	t.Help.ShortSeparator = t.Help.ShortSeparator.Foreground(chrome)

	return t
}
