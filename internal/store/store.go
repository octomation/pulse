// Package store keeps the state of pulse as plain files in its data
// directory: a snapshot per successful load, the marks and the load attempts.
package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"go.octolab.org/toolset/pulse/internal/trend"
)

// The files of the data directory.
const (
	// SnapshotsDir holds <source>/<period>/<UTC time>.json, a file per load.
	SnapshotsDir = "snapshots"
	// MarksFile is the append-only log of marks, a JSON object per line.
	MarksFile = "marks.jsonl"
	// AttemptsFile is the append-only log of load attempts.
	AttemptsFile = "attempts.jsonl"
)

// snapshotName is the layout of a snapshot file name: UTC, sortable, and
// without the colons a file name should not have.
const snapshotName = "2006-01-02T15-04-05.000Z"

// Action is how a repository was marked as seen.
type Action string

// The actions of the page.
const (
	// Interesting marks the repository as seen and bookmarks it.
	Interesting Action = "interesting"
	// Skip marks the repository as seen.
	Skip Action = "skip"
	// Seen is the mark of "Mark all shown as seen".
	Seen Action = "seen"
)

// Valid reports whether the action is one of the three.
func (a Action) Valid() bool {
	switch a {
	case Interesting, Skip, Seen:
		return true
	default:
		return false
	}
}

// Mark is one record of marks.jsonl. A mark of any action makes its
// repository seen; an Interesting one is also its bookmark, so a bookmark
// and its seen mark cannot exist without each other.
type Mark struct {
	Time    time.Time  `json:"time"`
	Repo    trend.Repo `json:"repo"`
	Action  Action     `json:"action"`
	Comment string     `json:"comment,omitempty"`
	// Lists names the lists the repository was in at that moment.
	Lists []string `json:"lists"`
}

// Attempt is one record of attempts.jsonl: a load of a list and its outcome.
type Attempt struct {
	Time  time.Time `json:"time"`
	List  string    `json:"list"`
	OK    bool      `json:"ok"`
	Error string    `json:"error,omitempty"`
}

// Bookmark is a repository marked as interesting: the first time it was, the
// lists it was in then, and the comment of the latest mark.
type Bookmark struct {
	Repo    trend.Repo
	Comment string
	At      time.Time
	Lists   []string
}

// Store is the state in a data directory. It is safe for concurrent use by
// one process; it does not guard against a second process on the directory.
type Store struct {
	dir string

	mu       sync.RWMutex
	marks    []Mark
	attempts map[string]Attempt
	latest   map[trend.List]trend.Snapshot
	// times holds the time of every snapshot of a list, as its file name
	// writes it, oldest first.
	times map[trend.List][]time.Time
}

// Open reads the state of the directory, creating it when it is not there.
// A record that cannot be read is an error that names its file and line:
// nothing is dropped.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("data directory: %w", err)
	}
	s := &Store{
		dir: dir, attempts: map[string]Attempt{},
		latest: map[trend.List]trend.Snapshot{}, times: map[trend.List][]time.Time{},
	}

	err := readLines(filepath.Join(dir, MarksFile), func(line []byte) error {
		var mark Mark
		if err := decode(line, &mark); err != nil {
			return err
		}
		if err := mark.validate(); err != nil {
			return err
		}
		s.marks = append(s.marks, mark)
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = readLines(filepath.Join(dir, AttemptsFile), func(line []byte) error {
		var attempt Attempt
		if err := decode(line, &attempt); err != nil {
			return err
		}
		if err := attempt.validate(); err != nil {
			return err
		}
		s.attempts[attempt.List] = attempt
		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, list := range trend.Lists() {
		snapshot, found, times, err := readLatest(s.snapshotDir(list), list)
		if err != nil {
			return nil, err
		}
		if found {
			s.latest[list] = snapshot
		}
		s.times[list] = times
	}
	return s, nil
}

// Dir returns the data directory.
func (s *Store) Dir() string { return s.dir }

func (s *Store) snapshotDir(list trend.List) string {
	return filepath.Join(s.dir, SnapshotsDir, string(list.Source), string(list.Period))
}

// SaveSnapshot stores a successful load as a file of its own, written to a
// temporary file, synced and renamed into place.
func (s *Store) SaveSnapshot(snapshot trend.Snapshot) error {
	switch {
	case snapshot.FetchedAt.IsZero():
		return errors.New("snapshot: no time of the load")
	case len(snapshot.Entries) == 0:
		return errors.New("snapshot: no repositories")
	}
	snapshot.FetchedAt = snapshot.FetchedAt.UTC()
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}
	dir := s.snapshotDir(snapshot.List)
	name := snapshot.FetchedAt.Format(snapshotName) + ".json"

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}
	if err := writeAtomic(dir, name, append(raw, '\n')); err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}
	if current, found := s.latest[snapshot.List]; !found || !snapshot.FetchedAt.Before(current.FetchedAt) {
		s.latest[snapshot.List] = snapshot
	}
	if at, ok := snapshotTime(name); ok {
		times := s.times[snapshot.List]
		if i, found := slices.BinarySearchFunc(times, at, time.Time.Compare); !found {
			s.times[snapshot.List] = slices.Insert(times, i, at)
		}
	}
	return nil
}

// Times returns the time of every stored snapshot of the list, oldest
// first.
func (s *Store) Times(list trend.List) []time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.times[list])
}

// LatestIn returns the last snapshot of the list taken at from or later and
// before to. It reads the file of that snapshot: a file that cannot be read
// is an error that names it.
func (s *Store) LatestIn(list trend.List, from, to time.Time) (trend.Snapshot, bool, error) {
	s.mu.RLock()
	times := s.times[list]
	i, _ := slices.BinarySearchFunc(times, to, time.Time.Compare)
	var at time.Time
	if i > 0 && !times[i-1].Before(from) {
		at = times[i-1]
	}
	s.mu.RUnlock()
	if at.IsZero() {
		return trend.Snapshot{}, false, nil
	}
	path := filepath.Join(s.snapshotDir(list), at.Format(snapshotName)+".json")
	snapshot, err := readSnapshot(path, list)
	if err != nil {
		return trend.Snapshot{}, false, fmt.Errorf("%s: %w", path, err)
	}
	return snapshot, true, nil
}

// snapshotTime reads the time of a snapshot from the name of its file.
func snapshotTime(name string) (time.Time, bool) {
	base, found := strings.CutSuffix(name, ".json")
	if !found {
		return time.Time{}, false
	}
	at, err := time.ParseInLocation(snapshotName, base, time.UTC)
	return at, err == nil
}

// Latest returns the latest successful snapshot of the list.
func (s *Store) Latest(list trend.List) (trend.Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, found := s.latest[list]
	return snapshot, found
}

// AppendMarks adds the marks to marks.jsonl in one write and syncs the file
// before it returns: either all of them are on disk, or it fails and none is
// in the store.
func (s *Store) AppendMarks(marks ...Mark) error {
	marks = slices.Clone(marks)
	for i, mark := range marks {
		if err := mark.validate(); err != nil {
			return fmt.Errorf("mark: %w", err)
		}
		if mark.Lists == nil {
			marks[i].Lists = []string{}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := appendLines(filepath.Join(s.dir, MarksFile), marks); err != nil {
		return err
	}
	s.marks = append(s.marks, marks...)
	return nil
}

// Marks returns every mark, oldest first.
func (s *Store) Marks() []Mark {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.marks)
}

// Bookmarks returns the bookmarks, oldest first.
func (s *Store) Bookmarks() []Bookmark {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return bookmarks(s.marks)
}

func bookmarks(marks []Mark) []Bookmark {
	var list []Bookmark
	index := map[string]int{}
	for _, mark := range marks {
		if mark.Action != Interesting {
			continue
		}
		key := mark.Repo.Key()
		if i, found := index[key]; found {
			list[i].Comment = mark.Comment
			continue
		}
		index[key] = len(list)
		list = append(list, Bookmark{
			Repo: mark.Repo, Comment: mark.Comment, At: mark.Time, Lists: slices.Clone(mark.Lists),
		})
	}
	return list
}

// AppendAttempts adds the load attempts to attempts.jsonl in one synced
// write.
func (s *Store) AppendAttempts(attempts ...Attempt) error {
	for _, attempt := range attempts {
		if err := attempt.validate(); err != nil {
			return fmt.Errorf("attempt: %w", err)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := appendLines(filepath.Join(s.dir, AttemptsFile), attempts); err != nil {
		return err
	}
	for _, attempt := range attempts {
		s.attempts[attempt.List] = attempt
	}
	return nil
}

// LastAttempt returns the last recorded attempt to load the list.
func (s *Store) LastAttempt(list trend.List) (Attempt, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	attempt, found := s.attempts[list.String()]
	return attempt, found
}

func (m Mark) validate() error {
	switch {
	case m.Time.IsZero():
		return errors.New("a mark needs its time")
	case m.Repo.Owner == "" || m.Repo.Name == "":
		return errors.New("a mark needs the owner and the name of its repository")
	case !m.Action.Valid():
		return fmt.Errorf("unknown action %q", m.Action)
	}
	return nil
}

func (a Attempt) validate() error {
	switch {
	case a.Time.IsZero():
		return errors.New("an attempt needs its time")
	case a.List == "":
		return errors.New("an attempt needs its list")
	}
	return nil
}

// decode reads one JSON object and nothing after it.
func decode(line []byte, record any) error {
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(record); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("more than one value on the line")
	}
	return nil
}

// readLines calls read for every line of a JSON Lines file. A missing file
// has no lines, and a blank line holds no record. An error names the file
// and the line.
func readLines(path string, read func(line []byte) error) error {
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	reader := bufio.NewReader(file)
	for number := 1; ; number++ {
		line, err := reader.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("%s:%d: %w", path, number, err)
		}
		if line = bytes.TrimSpace(line); len(line) > 0 {
			if e := read(line); e != nil {
				return fmt.Errorf("%s:%d: %w", path, number, e)
			}
		}
		if err != nil {
			return nil
		}
	}
}

// appendLines writes the records as lines in one write and syncs the file. A
// write that fails is taken back, so the file never ends with half a record.
func appendLines[T any](path string, records []T) error {
	if len(records) == 0 {
		return nil
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			return err
		}
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	size := info.Size()
	content := buffer.Bytes()
	if size > 0 {
		// A file edited by hand may have lost the end of its last line.
		last := make([]byte, 1)
		if _, err := file.ReadAt(last, size-1); err != nil {
			_ = file.Close()
			return err
		}
		if last[0] != '\n' {
			content = append([]byte{'\n'}, content...)
		}
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Truncate(size)
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Truncate(size)
		_ = file.Close()
		return err
	}
	return file.Close()
}

// writeAtomic writes name in dir through a temporary file: synced, renamed
// into place, and the directory synced after it.
func writeAtomic(dir, name string, content []byte) error {
	temp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	discard := func(err error) error {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
		return err
	}
	if _, err := temp.Write(content); err != nil {
		return discard(err)
	}
	if err := temp.Sync(); err != nil {
		return discard(err)
	}
	if err := temp.Close(); err != nil {
		return discard(err)
	}
	if err := os.Rename(temp.Name(), filepath.Join(dir, name)); err != nil {
		return discard(err)
	}
	if parent, err := os.Open(dir); err == nil {
		_ = parent.Sync()
		_ = parent.Close()
	}
	return nil
}

// readLatest reads the newest snapshot of a list, the names sort by time,
// and returns the times of all of them, oldest first. What a cut write left
// behind, a temporary file, is not a snapshot; a file whose name is not a
// time is read as the newest when it sorts last, but has no day.
func readLatest(dir string, list trend.List) (trend.Snapshot, bool, []time.Time, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return trend.Snapshot{}, false, nil, nil
	}
	if err != nil {
		return trend.Snapshot{}, false, nil, err
	}
	var (
		times    []time.Time
		snapshot trend.Snapshot
		found    bool
	)
	for _, entry := range slices.Backward(entries) {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".json") {
			continue
		}
		if at, ok := snapshotTime(name); ok {
			times = append(times, at)
		}
		if found {
			continue
		}
		path := filepath.Join(dir, name)
		if snapshot, err = readSnapshot(path, list); err != nil {
			return trend.Snapshot{}, false, nil, fmt.Errorf("%s: %w", path, err)
		}
		found = true
	}
	slices.Reverse(times)
	return snapshot, found, times, nil
}

func readSnapshot(path string, list trend.List) (trend.Snapshot, error) {
	var snapshot trend.Snapshot
	raw, err := os.ReadFile(path)
	if err != nil {
		return snapshot, err
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return snapshot, err
	}
	switch {
	case snapshot.List != list:
		return snapshot, fmt.Errorf("the snapshot is of %s, not of %s", snapshot.List, list)
	case snapshot.FetchedAt.IsZero():
		return snapshot, errors.New("the snapshot has no time")
	case len(snapshot.Entries) == 0:
		return snapshot, errors.New("the snapshot has no repositories")
	}
	return snapshot, nil
}
