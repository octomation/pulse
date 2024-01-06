package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"go.octolab.org/toolset/pulse/internal/hide"
)

// Source is the name of the vault among the sources that hide a card.
const Source = "vault"

// Label is what a card hidden by a note says before the name of the note.
const Label = "In the vault"

// Settings configure a provider of the vault.
type Settings struct {
	// Bookmarks is the directory of the bookmark notes; empty turns the
	// vault off.
	Bookmarks string
	// Obsidian is the file where Obsidian lists the vaults it knows; empty
	// skips the check that the directory is inside one of them.
	Obsidian string
}

// ObsidianConfig returns where Obsidian lists its vaults for the user of
// home: on macOS in Application Support, elsewhere in the XDG configuration
// directory.
func ObsidianConfig(home string, getenv func(string) string) string {
	if runtime.GOOS == "darwin" {
		if home == "" {
			return ""
		}
		return filepath.Join(home, "Library", "Application Support", "obsidian", "obsidian.json")
	}
	if xdg := getenv("XDG_CONFIG_HOME"); filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "obsidian", "obsidian.json")
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "obsidian", "obsidian.json")
}

// Dir is the provider that reads the bookmark notes of a directory itself.
type Dir struct {
	Settings
}

// NewDir returns the provider that reads the directory of the settings.
func NewDir(settings Settings) (hide.Provider, error) { return Dir{Settings: settings}, nil }

// Check reads the notes again and answers, for each repository, the note
// that names it.
func (d Dir) Check(_ context.Context, items []hide.Item) ([][]hide.Reason, hide.Status) {
	status := hide.Status{Source: Source, Provider: "dir", Origin: d.Bookmarks}
	if d.Bookmarks == "" {
		status.State = hide.Off
		return nil, status
	}
	index, err := Read(d.Bookmarks)
	status.Entries, status.Documents, status.Broken = len(index.Repos), index.Notes, index.Broken
	status.Newest = index.Newest
	status.Warning = obsidianWarning(d.Bookmarks, d.Obsidian)
	if err != nil {
		status.State, status.Error = hide.Failed, err.Error()
		if errors.Is(err, ErrNoRepositories) {
			status.Error = fmt.Sprintf("no note of %s names a repository", d.Bookmarks)
		}
		return nil, status
	}
	status.State = hide.OK
	answer := make([][]hide.Reason, len(items))
	for i, item := range items {
		if note, found := index.Note(item.Repo); found {
			answer[i] = []hide.Reason{{Source: Source, Label: Label, Ref: note, At: index.Modified[note]}}
		}
	}
	return answer, status
}

// obsidianWarning says when dir is not inside a vault that Obsidian knows:
// most likely a copy of the vault, not the one the user writes to. It says
// nothing when there is no list to check against.
func obsidianWarning(dir, list string) string {
	if list == "" {
		return ""
	}
	raw, err := os.ReadFile(list)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "Obsidian knows no vault on this computer, so the directory is not one of its vaults"
		}
		return fmt.Sprintf("the list of Obsidian vaults could not be read: %v", err)
	}
	var known struct {
		Vaults map[string]struct {
			Path string `json:"path"`
		} `json:"vaults"`
	}
	if err := json.Unmarshal(raw, &known); err != nil {
		return fmt.Sprintf("the list of Obsidian vaults %s could not be read: %v", list, err)
	}
	target := resolve(dir)
	for _, vault := range known.Vaults {
		if vault.Path != "" && inside(target, resolve(vault.Path)) {
			return ""
		}
	}
	return "the directory is not inside any vault Obsidian knows on this computer, so it may be an old copy"
}

// resolve returns the path with its links followed, when it can.
func resolve(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return filepath.Clean(path)
}

// inside reports whether path is root or in it. The directories are
// compared as files, so a volume that ignores case finds them under any
// case.
func inside(path, root string) bool {
	want, err := os.Stat(root)
	if err != nil {
		rel, err := filepath.Rel(root, path)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
	}
	for dir := path; ; dir = filepath.Dir(dir) {
		if info, err := os.Stat(dir); err == nil && os.SameFile(info, want) {
			return true
		}
		if filepath.Dir(dir) == dir {
			return false
		}
	}
}
