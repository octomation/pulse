package vault_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.octolab.org/toolset/pulse/internal/trend"
	. "go.octolab.org/toolset/pulse/internal/vault"
)

func TestRead(t *testing.T) {
	index, err := Read("testdata/notes")
	require.NoError(t, err)

	assert.Equal(t, map[string]string{
		// By the url of the note.
		"snouzy/workout-cool": "workout-cool.md",
		// Only by an alias of the form owner/name, in another letter case.
		"astral-sh/uv": "uv.md",
		// The url with a trailing slash and the alias name one repository;
		// a URL among the aliases names none.
		"charmbracelet/bubbletea": "two repositories.md",
		// A scalar in the place of a list, and a list in the place of a
		// scalar.
		"caddyserver/caddy": "scalar alias.md",
	}, index.Repos)

	// Every *.md directly in the directory, and nothing else.
	assert.Equal(t, 11, index.Notes)
	assert.Equal(t, []string{"broken.md", "unclosed.md"}, index.Broken)

	note, found := index.Note(trend.Repo{Owner: "ASTRAL-SH", Name: "Uv"})
	assert.True(t, found)
	assert.Equal(t, "uv.md", note)

	for _, hidden := range []trend.Repo{
		{Owner: "anthropics", Name: "claude-code"},    // the note is of the owner
		{Owner: "tailscale", Name: "tailscale"},       // a broken note, and the body of a note
		{Owner: "golang", Name: "go"},                 // a deeper URL
		{Owner: "charmbracelet", Name: "lipgloss"},    // a URL among the aliases
		{Owner: "junegunn", Name: "fzf"},              // the frontmatter is not closed
		{Owner: "ignored", Name: "text-file"},         // not a Markdown file
		{Owner: "ignored", Name: "nested"},            // not directly in the directory
		{Owner: "and", Name: "or"},                    // an alias with a deeper path
		{Owner: "docs.astral.sh", Name: "uv"},         // a URL of another site
		{Owner: "workout-cool", Name: "workout-cool"}, // an alias without an owner
	} {
		_, found := index.Note(hidden)
		assert.False(t, found, hidden)
	}
}

// A note saved with Windows line ends or a byte order mark is a note.
func TestReadLineEndsAndByteOrderMark(t *testing.T) {
	dir := t.TempDir()
	for name, note := range map[string]string{
		"crlf.md": "---\r\nurl: https://github.com/denoland/deno\r\n---\r\nThe body.\r\n",
		"bom.md":  "\xef\xbb\xbf---\nurl: https://github.com/rust-lang/rust\n---\n",
		"MD.MD":   "---\naliases: [golang/go]\n---",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(note), 0o600))
	}
	index, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"denoland/deno": "crlf.md", "rust-lang/rust": "bom.md", "golang/go": "MD.MD",
	}, index.Repos)
	assert.Empty(t, index.Broken)
}

func TestReadErrors(t *testing.T) {
	t.Run("a missing directory", func(t *testing.T) {
		_, err := Read(filepath.Join(t.TempDir(), "nowhere"))
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrNoRepositories)
	})

	t.Run("a file in the place of the directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "notes")
		require.NoError(t, os.WriteFile(path, nil, 0o600))
		_, err := Read(path)
		assert.Error(t, err)
	})

	t.Run("an empty directory", func(t *testing.T) {
		index, err := Read(t.TempDir())
		require.ErrorIs(t, err, ErrNoRepositories)
		assert.Zero(t, index.Notes)
	})

	t.Run("notes without a repository", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "owner.md"), []byte("---\nurl: https://github.com/golang\n---\n"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.md"), []byte("---\nurl: [\n---\n"), 0o600))
		index, err := Read(dir)
		require.ErrorIs(t, err, ErrNoRepositories)
		assert.Equal(t, 2, index.Notes)
		assert.Equal(t, []string{"broken.md"}, index.Broken)
	})
}

// The notes are read for every page: the body of a note, however long, is
// not.
func TestReadDoesNotReadTheBody(t *testing.T) {
	dir := t.TempDir()
	body := make([]byte, 1<<20)
	for i := range body {
		body[i] = 'x'
	}
	note := append([]byte("---\nurl: https://github.com/golang/go\n---\n"), body...)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "long.md"), note, 0o600))
	// A frontmatter that never ends is given up, not read to its end.
	endless := append([]byte("---\nnote: |\n"), body...)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "endless.md"), endless, 0o600))

	index, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"golang/go": "long.md"}, index.Repos)
	assert.Equal(t, []string{"endless.md"}, index.Broken)
}

func BenchmarkRead(b *testing.B) {
	dir := b.TempDir()
	for i := range 2600 {
		note := fmt.Sprintf("---\nuid: 2fb819c2-%04d\naliases:\n  - name-%d\n  - owner-%d/name-%d\ntags:\n  - type/bookmark\n"+
			"url: https://github.com/owner-%d/name-%d\nalt:\n  - https://example.com/\nowner: \"[[Owner at GitHub]]\"\n---\n\nThe body.\n", i, i, i, i, i, i)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("note-%04d.md", i)), []byte(note), 0o600); err != nil {
			b.Fatal(err)
		}
	}
	for b.Loop() {
		index, err := Read(dir)
		if err != nil || len(index.Repos) != 2600 {
			b.Fatal(err, len(index.Repos))
		}
	}
}
