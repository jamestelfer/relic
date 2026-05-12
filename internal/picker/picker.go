// Package picker discovers Claude Code session JSONL files and presents an
// interactive two-step selection menu (project then session) to the user.
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
}

// ErrAborted is returned by Pick when the user cancels the selection (Ctrl-C / Esc).
var ErrAborted = errors.New("picker aborted")

// ErrNoProjects is returned when no project directories with .jsonl files are found.
var ErrNoProjects = errors.New("no Claude Code projects found")

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
	projectOptions := make([]huh.Option[string], len(projects))
	for i, p := range projects {
		label := filepath.Base(p.Dir)
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
	)

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

	sessionOptions := make([]huh.Option[string], len(sessions))
	for i, s := range sessions {
		label := filepath.Base(s.Path)
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
	)

	if err := sessionForm.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", ErrAborted
		}
		return "", err
	}

	return chosenSession, nil
}
