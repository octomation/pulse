package vault_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.octolab.org/toolset/pulse/internal/hide"
	"go.octolab.org/toolset/pulse/internal/trend"
	. "go.octolab.org/toolset/pulse/internal/vault"
)

func items(names ...string) []hide.Item {
	var list []hide.Item
	for _, name := range names {
		repo, _ := trend.ParseRepo(name)
		list = append(list, hide.Item{Repo: repo})
	}
	return list
}

// The provider of the directory finds what Read finds, and answers in the
// order of the items with the note and its time.
func TestDirProvider(t *testing.T) {
	at := time.Date(2026, time.November, 20, 12, 5, 0, 0, time.UTC)
	dir := t.TempDir()
	note := filepath.Join(dir, "uv.md")
	require.NoError(t, os.WriteFile(note, []byte("---\naliases: [astral-sh/uv]\n---\n"), 0o600))
	require.NoError(t, os.Chtimes(note, at, at))

	answer, status := Dir{Settings: Settings{Bookmarks: dir}}.Check(t.Context(), items("golang/go", "Astral-SH/UV"))
	assert.Equal(t, [][]hide.Reason{nil, {{Source: "vault", Label: "In the vault", Ref: "uv.md", At: answer[1][0].At}}}, answer)
	assert.True(t, answer[1][0].At.Equal(at))
	assert.Equal(t, hide.OK, status.State)
	assert.Equal(t, dir, status.Origin)
	assert.Equal(t, "dir", status.Provider)
	assert.Equal(t, 1, status.Entries)
	assert.Equal(t, 1, status.Documents)
	assert.True(t, status.Newest.Equal(at))

	t.Run("the reasons of the fixtures are those of Read", func(t *testing.T) {
		index, err := Read("testdata/notes")
		require.NoError(t, err)
		answer, status := Dir{Settings: Settings{Bookmarks: "testdata/notes"}}.Check(t.Context(), items(
			"snouzy/workout-cool", "astral-sh/uv", "charmbracelet/bubbletea", "caddyserver/caddy", "golang/go"))
		require.Equal(t, hide.OK, status.State)
		assert.Equal(t, []string{"broken.md", "unclosed.md"}, status.Broken)
		for i, item := range items("snouzy/workout-cool", "astral-sh/uv", "charmbracelet/bubbletea", "caddyserver/caddy") {
			note, _ := index.Note(item.Repo)
			assert.Equal(t, note, answer[i][0].Ref)
		}
		assert.Empty(t, answer[4])
	})

	t.Run("without a directory the vault is off", func(t *testing.T) {
		answer, status := Dir{}.Check(t.Context(), items("golang/go"))
		assert.Nil(t, answer)
		assert.Equal(t, hide.Off, status.State)
	})

	t.Run("a directory that cannot be read fails with its error", func(t *testing.T) {
		answer, status := Dir{Settings: Settings{Bookmarks: filepath.Join(dir, "nowhere")}}.Check(t.Context(), items("golang/go"))
		assert.Nil(t, answer)
		assert.Equal(t, hide.Failed, status.State)
		assert.Contains(t, status.Error, "nowhere")
	})
}

func TestObsidianCheck(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "Mindset")
	inside := filepath.Join(vault, "Bookmarks")
	outside := filepath.Join(root, "Mindset-old", "Bookmarks")
	linked := filepath.Join(root, "link")
	for _, dir := range []string{inside, outside} {
		require.NoError(t, os.MkdirAll(dir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.md"), []byte("---\nurl: https://github.com/golang/go\n---\n"), 0o600))
	}
	require.NoError(t, os.Symlink(inside, linked))
	list := filepath.Join(root, "obsidian.json")
	require.NoError(t, os.WriteFile(list, []byte(`{"vaults":{"a":{"path":"`+vault+`","ts":1},"b":{"path":""}}}`), 0o600))

	warning := func(dir, list string) string {
		_, status := Dir{Settings: Settings{Bookmarks: dir, Obsidian: list}}.Check(t.Context(), items("golang/go"))
		return status.Warning
	}
	assert.Empty(t, warning(inside, list))
	assert.Empty(t, warning(vault, list), "the vault itself, although no note there names a repository")
	assert.Empty(t, warning(linked, list), "a link into the vault")
	assert.Contains(t, warning(outside, list), "not inside any vault Obsidian knows")
	assert.Contains(t, warning(inside, filepath.Join(root, "missing.json")), "Obsidian knows no vault")
	assert.Empty(t, warning(outside, ""), "no list, no check")

	broken := filepath.Join(root, "broken.json")
	require.NoError(t, os.WriteFile(broken, []byte("{"), 0o600))
	assert.Contains(t, warning(inside, broken), "could not be read")
}

func TestObsidianConfig(t *testing.T) {
	path := ObsidianConfig("/home/me", func(string) string { return "" })
	assert.True(t, filepath.IsAbs(path))
	assert.Equal(t, "obsidian.json", filepath.Base(path))
	assert.Empty(t, ObsidianConfig("", func(string) string { return "" }))
}

// On a volume that ignores case, a directory written in another case is
// still inside the vault.
func TestObsidianCheckIgnoresCaseWhereTheVolumeDoes(t *testing.T) {
	root := t.TempDir()
	vault := filepath.Join(root, "Mindset")
	bookmarks := filepath.Join(vault, "Bookmarks")
	require.NoError(t, os.MkdirAll(bookmarks, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(bookmarks, "go.md"), []byte("---\nurl: https://github.com/golang/go\n---\n"), 0o600))
	other := filepath.Join(root, "mindset", "bookmarks")
	if _, err := os.Stat(other); err != nil {
		t.Skip("the volume tells the cases apart")
	}
	list := filepath.Join(root, "obsidian.json")
	require.NoError(t, os.WriteFile(list, []byte(`{"vaults":{"a":{"path":"`+vault+`"}}}`), 0o600))
	_, status := Dir{Settings: Settings{Bookmarks: other, Obsidian: list}}.Check(t.Context(), items("golang/go"))
	assert.Empty(t, status.Warning)
}
