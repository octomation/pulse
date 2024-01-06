// Package vault reads the bookmark notes of the vault: Markdown files whose
// frontmatter names a repository. It only reads.
package vault

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"

	"go.octolab.org/toolset/pulse/internal/trend"
)

// ErrNoRepositories says that no note of the directory names a repository:
// the path is most likely wrong, so the filter would hide nothing.
var ErrNoRepositories = errors.New("no note names a repository")

// maxFrontmatter bounds what is read of a note: a frontmatter is small, and
// the body of a note is never read.
const maxFrontmatter = 256 << 10

// Index is what the notes of a directory say.
type Index struct {
	// Repos maps the key of a repository to the file name of its note.
	Repos map[string]string
	// Notes is the number of Markdown files in the directory.
	Notes int
	// Broken names the notes whose frontmatter could not be read.
	Broken []string
	// Modified maps the file name of a note to the time it was last
	// changed.
	Modified map[string]time.Time
	// Newest is the time of the note changed last.
	Newest time.Time
}

// Note returns the note that names the repository.
func (i Index) Note(repo trend.Repo) (string, bool) {
	note, found := i.Repos[repo.Key()]
	return note, found
}

// Read reads the frontmatter of every *.md file directly in dir. A
// repository is in the vault when a note names it in its url, as
// https://github.com/owner/name, or in an alias of the form owner/name. A
// note that cannot be read is counted in Broken. A directory that cannot be
// read is an error, and so is one without a single repository:
// ErrNoRepositories, returned with the index.
func Read(dir string) (Index, error) {
	index := Index{Repos: map[string]string{}, Modified: map[string]time.Time{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return index, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			continue
		}
		names = append(names, entry.Name())
		if info, err := entry.Info(); err == nil {
			index.Modified[entry.Name()] = info.ModTime()
			if info.ModTime().After(index.Newest) {
				index.Newest = info.ModTime()
			}
		}
	}
	index.Notes = len(names)

	type result struct {
		repos []trend.Repo
		err   error
	}
	results := make([]result, len(names))
	var next int
	var mu sync.Mutex
	var group sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), 8, len(names)) {
		group.Go(func() {
			for {
				mu.Lock()
				i := next
				next++
				mu.Unlock()
				if i >= len(names) {
					return
				}
				repos, err := readNote(filepath.Join(dir, names[i]))
				results[i] = result{repos: repos, err: err}
			}
		})
	}
	group.Wait()

	for i, name := range names {
		if results[i].err != nil {
			index.Broken = append(index.Broken, name)
			continue
		}
		for _, repo := range results[i].repos {
			if _, found := index.Repos[repo.Key()]; !found {
				index.Repos[repo.Key()] = name
			}
		}
	}
	if len(index.Repos) == 0 {
		return index, ErrNoRepositories
	}
	return index, nil
}

// frontmatter is the part of a note pulse reads.
type frontmatter struct {
	URL     values `yaml:"url"`
	Aliases values `yaml:"aliases"`
}

// values reads a scalar or a list of scalars: notes written by hand have
// both, and what is neither holds no repository.
type values []string

// UnmarshalYAML implements yaml.Unmarshaler.
func (v *values) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		*v = append(*v, node.Value)
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if item.Kind == yaml.ScalarNode {
				*v = append(*v, item.Value)
			}
		}
	case yaml.DocumentNode, yaml.MappingNode, yaml.AliasNode:
	}
	return nil
}

// readNote returns the repositories the frontmatter of a note names. A note
// without a frontmatter names none.
func readNote(path string) ([]trend.Repo, error) {
	block, err := readFrontmatter(path)
	if err != nil || block == nil {
		return nil, err
	}
	var meta frontmatter
	if err := yaml.Unmarshal(block, &meta); err != nil {
		return nil, err
	}
	var repos []trend.Repo
	for _, url := range meta.URL {
		// A URL of an owner, or of another site, is not a repository.
		if !strings.Contains(url, "://") {
			continue
		}
		if repo, err := trend.ParseRepo(url); err == nil {
			repos = append(repos, repo)
		}
	}
	for _, alias := range meta.Aliases {
		// Only owner/name: a URL among the aliases is another address of
		// the bookmark, not its repository.
		if strings.Contains(alias, "://") {
			continue
		}
		if repo, err := trend.ParseRepo(alias); err == nil {
			repos = append(repos, repo)
		}
	}
	return repos, nil
}

// readFrontmatter returns the block between the leading --- lines of a file,
// or nil when the file does not start with one.
func readFrontmatter(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	reader := bufio.NewReader(io.LimitReader(file, maxFrontmatter))
	var block bytes.Buffer
	for first := true; ; first = false {
		line, err := reader.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		fence := isFence(line, first)
		switch {
		case first && !fence:
			return nil, nil
		case !first && fence:
			return block.Bytes(), nil
		case !first:
			block.Write(line)
		}
		if err != nil {
			return nil, fmt.Errorf("the frontmatter is not closed")
		}
	}
}

// isFence reports whether the line is the --- that opens or closes a
// frontmatter; the first line of a file may start with a byte order mark.
func isFence(line []byte, first bool) bool {
	if first {
		line = bytes.TrimPrefix(line, []byte("\xef\xbb\xbf"))
	}
	return string(bytes.TrimRight(line, " \t\r\n")) == "---"
}
