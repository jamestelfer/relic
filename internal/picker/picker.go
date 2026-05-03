// Package picker discovers Claude Code session JSONL files and presents an
// interactive selection menu to the user.
package picker

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
)

// Entry represents a discovered session file.
type Entry struct {
	Path    string    // absolute path
	ModTime time.Time // file modification time
}

// ErrAborted is returned by Pick when the user cancels the selection (Ctrl-C / Esc).
var ErrAborted = errors.New("picker aborted")

// ErrNoSessions is returned by Discover when no .jsonl files are found.
var ErrNoSessions = errors.New("no session files found")

// Discover scans homeDir/.claude/projects recursively for .jsonl files,
// returning them sorted by modification time, most recent first.
// Returns ErrNoSessions (wrapped) when the directory exists but is empty.
func Discover(homeDir string) ([]Entry, error) {
	projectsDir := filepath.Join(homeDir, ".claude", "projects")

	var entries []Entry
	err := fs.WalkDir(os.DirFS(projectsDir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip unreadable subtrees.
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".jsonl") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		entries = append(entries, Entry{
			Path:    filepath.Join(projectsDir, path),
			ModTime: info.ModTime(),
		})
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("walk %s: %w", projectsDir, err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("%w in %s", ErrNoSessions, projectsDir)
	}

	// Sort most recent first.
	slices.SortFunc(entries, func(a, b Entry) int {
		if a.ModTime.After(b.ModTime) {
			return -1
		}
		if a.ModTime.Before(b.ModTime) {
			return 1
		}
		return 0
	})

	return entries, nil
}

// RelativePath returns path with homeDir replaced by "~". If path does not
// start with homeDir, it is returned unchanged.
func RelativePath(homeDir, path string) string {
	if strings.HasPrefix(path, homeDir+string(filepath.Separator)) {
		return "~" + path[len(homeDir):]
	}
	return path
}

// Pick presents an interactive huh select menu of discovered session files and
// returns the chosen absolute path. If the user aborts (Ctrl-C / Esc),
// huh.ErrUserAborted is returned.
func Pick(homeDir string) (string, error) {
	entries, err := Discover(homeDir)
	if err != nil {
		return "", err
	}

	options := make([]huh.Option[string], len(entries))
	for i, e := range entries {
		label := RelativePath(homeDir, e.Path)
		options[i] = huh.NewOption(label, e.Path)
	}

	var chosen string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a Claude Code session").
				Options(options...).
				Value(&chosen),
		),
	)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", ErrAborted
		}
		return "", err
	}
	return chosen, nil
}
