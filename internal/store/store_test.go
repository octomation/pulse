package store_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "go.octolab.org/toolset/pulse/internal/store"
	"go.octolab.org/toolset/pulse/internal/trend"
)

var (
	morning = time.Date(2026, time.November, 19, 8, 15, 0, 0, time.UTC)
	daily   = trend.List{Source: trend.Trendshift, Period: trend.Daily}
	weekly  = trend.List{Source: trend.GitHub, Period: trend.Weekly}
)

func snapshot(list trend.List, at time.Time, names ...string) trend.Snapshot {
	s := trend.Snapshot{
		List: list, Request: "https://trendshift.io/", Marker: "https://trendshift.io Daily", FetchedAt: at,
	}
	for i, name := range names {
		owner, repo, _ := strings.Cut(name, "/")
		s.Entries = append(s.Entries, trend.Entry{
			Rank: i + 1, Repo: trend.Repo{Owner: owner, Name: repo}, Description: "about " + name, Language: "Go",
		})
	}
	return s
}

func mark(at time.Time, action Action, name, comment string, lists ...string) Mark {
	owner, repo, _ := strings.Cut(name, "/")
	return Mark{Time: at, Repo: trend.Repo{Owner: owner, Name: repo}, Action: action, Comment: comment, Lists: lists}
}

func TestSnapshotsAndMarksSurviveReopen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pulse")
	s, err := Open(dir)
	require.NoError(t, err)
	assert.Equal(t, dir, s.Dir())

	_, found := s.Latest(daily)
	assert.False(t, found)
	_, found = s.LastAttempt(daily)
	assert.False(t, found)

	older := snapshot(daily, morning, "golang/go", "ollama/ollama")
	newer := snapshot(daily, morning.Add(time.Hour), "astral-sh/uv")
	other := snapshot(weekly, morning, "denoland/deno")
	for _, snap := range []trend.Snapshot{older, newer, other} {
		require.NoError(t, s.SaveSnapshot(snap))
	}
	marks := []Mark{
		mark(morning, Skip, "golang/go", "", "trendshift/daily"),
		mark(morning.Add(time.Minute), Interesting, "ollama/ollama", "try for the hub", "trendshift/daily", "github/weekly"),
		mark(morning.Add(2*time.Minute), Seen, "astral-sh/uv", ""),
	}
	require.NoError(t, s.AppendMarks(marks[0]))
	require.NoError(t, s.AppendMarks(marks[1:]...))
	attempts := []Attempt{
		{Time: morning, List: daily.String(), OK: true},
		{Time: morning, List: weekly.String(), OK: true},
		{Time: morning.Add(time.Hour), List: weekly.String(), Error: "the site answered 503"},
	}
	require.NoError(t, s.AppendAttempts(attempts[:2]...))
	require.NoError(t, s.AppendAttempts(attempts[2]))

	check := func(t *testing.T, s *Store) {
		t.Helper()
		got, found := s.Latest(daily)
		require.True(t, found)
		assert.Equal(t, newer, got)
		got, found = s.Latest(weekly)
		require.True(t, found)
		assert.Equal(t, other, got)
		_, found = s.Latest(trend.List{Source: trend.GitHub, Period: trend.Monthly})
		assert.False(t, found)

		want := []Mark{marks[0], marks[1], marks[2]}
		want[2].Lists = []string{}
		assert.Equal(t, want, s.Marks())

		last, found := s.LastAttempt(daily)
		require.True(t, found)
		assert.Equal(t, attempts[0], last)
		last, found = s.LastAttempt(weekly)
		require.True(t, found)
		assert.Equal(t, attempts[2], last)

		assert.Equal(t, []Bookmark{{
			Repo: trend.Repo{Owner: "ollama", Name: "ollama"}, Comment: "try for the hub",
			At: morning.Add(time.Minute), Lists: []string{"trendshift/daily", "github/weekly"},
		}}, s.Bookmarks())
	}
	check(t, s)

	reopened, err := Open(dir)
	require.NoError(t, err)
	check(t, reopened)

	t.Run("the files are plain and readable", func(t *testing.T) {
		names, err := filepath.Glob(filepath.Join(dir, SnapshotsDir, "trendshift", "daily", "*"))
		require.NoError(t, err)
		assert.Equal(t, []string{
			filepath.Join(dir, SnapshotsDir, "trendshift", "daily", "2026-11-19T08-15-00.000Z.json"),
			filepath.Join(dir, SnapshotsDir, "trendshift", "daily", "2026-11-19T09-15-00.000Z.json"),
		}, names, "a file per load, and no temporary file left")

		raw, err := os.ReadFile(filepath.Join(dir, MarksFile))
		require.NoError(t, err)
		assert.Equal(t, ""+
			`{"time":"2026-11-19T08:15:00Z","repo":{"owner":"golang","name":"go"},"action":"skip","lists":["trendshift/daily"]}`+"\n"+
			`{"time":"2026-11-19T08:16:00Z","repo":{"owner":"ollama","name":"ollama"},"action":"interesting","comment":"try for the hub","lists":["trendshift/daily","github/weekly"]}`+"\n"+
			`{"time":"2026-11-19T08:17:00Z","repo":{"owner":"astral-sh","name":"uv"},"action":"seen","lists":[]}`+"\n",
			string(raw))

		raw, err = os.ReadFile(filepath.Join(dir, AttemptsFile))
		require.NoError(t, err)
		assert.Equal(t, ""+
			`{"time":"2026-11-19T08:15:00Z","list":"trendshift/daily","ok":true}`+"\n"+
			`{"time":"2026-11-19T08:15:00Z","list":"github/weekly","ok":true}`+"\n"+
			`{"time":"2026-11-19T09:15:00Z","list":"github/weekly","ok":false,"error":"the site answered 503"}`+"\n",
			string(raw))
	})
}

func TestBookmarks(t *testing.T) {
	s, err := Open(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, s.AppendMarks(
		mark(morning, Interesting, "ollama/ollama", "first", "trendshift/daily"),
		mark(morning.Add(time.Minute), Skip, "golang/go", ""),
		mark(morning.Add(2*time.Minute), Interesting, "astral-sh/uv", ""),
		mark(morning.Add(3*time.Minute), Seen, "ollama/ollama", ""),
		mark(morning.Add(4*time.Minute), Interesting, "Ollama/Ollama", "second", "github/daily"),
	))

	// The last answer counts: the seen mark ended the first bookmark of
	// ollama, and the next Interesting starts another one.
	assert.Equal(t, []Bookmark{
		{Repo: trend.Repo{Owner: "astral-sh", Name: "uv"}, At: morning.Add(2 * time.Minute), Lists: []string{}},
		{Repo: trend.Repo{Owner: "Ollama", Name: "Ollama"}, Comment: "second", At: morning.Add(4 * time.Minute), Lists: []string{"github/daily"}},
	}, s.Bookmarks())

	// Interesting again keeps the place and the time of the first mark of
	// the run, with the latest comment; Clear takes the answer back.
	require.NoError(t, s.AppendMarks(
		mark(morning.Add(5*time.Minute), Interesting, "astral-sh/uv", "the fast one", "github/weekly"),
		mark(morning.Add(6*time.Minute), Clear, "ollama/ollama", ""),
	))
	assert.Equal(t, []Bookmark{
		{Repo: trend.Repo{Owner: "astral-sh", Name: "uv"}, Comment: "the fast one", At: morning.Add(2 * time.Minute), Lists: []string{}},
	}, s.Bookmarks())
	assert.Equal(t, []Standing{
		{Repo: trend.Repo{Owner: "golang", Name: "go"}, Action: Skip, At: morning.Add(time.Minute), Lists: []string{}, Line: 1},
		{Repo: trend.Repo{Owner: "astral-sh", Name: "uv"}, Action: Interesting, Comment: "the fast one", At: morning.Add(2 * time.Minute), Lists: []string{}, Line: 2},
	}, s.Standings())

	// A record of Clear survives a restart, and an older pulse would refuse
	// it: it is a fourth action.
	again, err := Open(s.Dir())
	require.NoError(t, err)
	assert.Equal(t, s.Standings(), again.Standings())
	assert.True(t, Clear.Valid())
}

func TestCorruptRecordIsAnErrorThatNamesFileAndLine(t *testing.T) {
	valid := `{"time":"2026-11-19T08:15:00Z","repo":{"owner":"golang","name":"go"},"action":"skip","lists":[]}`
	attempt := `{"time":"2026-11-19T08:15:00Z","list":"trendshift/daily","ok":true}`

	tests := map[string]struct {
		file    string
		content string
		line    int
	}{
		"marks: not JSON":           {MarksFile, valid + "\n" + "{oops\n" + valid + "\n", 2},
		"marks: a cut record":       {MarksFile, valid + "\n" + valid + "\n" + valid[:40], 3},
		"marks: unknown action":     {MarksFile, strings.Replace(valid, "skip", "star", 1) + "\n", 1},
		"marks: no repository":      {MarksFile, valid + "\n\n" + `{"time":"2026-11-19T08:15:00Z","repo":{"owner":"golang"},"action":"skip"}` + "\n", 3},
		"marks: no time":            {MarksFile, `{"repo":{"owner":"golang","name":"go"},"action":"skip"}` + "\n", 1},
		"marks: an unknown field":   {MarksFile, valid + "\n" + strings.Replace(valid, `"lists"`, `"list"`, 1) + "\n", 2},
		"marks: two records glued":  {MarksFile, valid + valid + "\n", 1},
		"marks: not an object":      {MarksFile, valid + "\n" + valid + "\n" + valid + "\n" + `"skip"` + "\n", 4},
		"attempts: not JSON":        {AttemptsFile, attempt + "\n" + attempt + "\n" + "nope\n", 3},
		"attempts: no list":         {AttemptsFile, `{"time":"2026-11-19T08:15:00Z","ok":true}` + "\n", 1},
		"attempts: a wrong type":    {AttemptsFile, attempt + "\n" + strings.Replace(attempt, "true", `"yes"`, 1) + "\n", 2},
		"attempts: a mark in there": {AttemptsFile, valid + "\n", 1},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, test.file)
			require.NoError(t, os.WriteFile(path, []byte(test.content), 0o600))

			s, err := Open(dir)
			require.Error(t, err)
			assert.Nil(t, s)
			assert.Contains(t, err.Error(), fmt.Sprintf("%s:%d: ", path, test.line))

			raw, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, test.content, string(raw), "the file is left as it was")
		})
	}

	t.Run("a corrupt snapshot names its file", func(t *testing.T) {
		dir := t.TempDir()
		s, err := Open(dir)
		require.NoError(t, err)
		require.NoError(t, s.SaveSnapshot(snapshot(daily, morning, "golang/go")))

		path := filepath.Join(dir, SnapshotsDir, "trendshift", "daily", "2026-11-19T09-15-00.000Z.json")
		for name, content := range map[string]string{
			"not JSON":        `{"list":`,
			"another list":    `{"list":{"source":"github","period":"daily"},"fetched_at":"2026-11-19T09:15:00Z","entries":[{"rank":1,"repo":{"owner":"a","name":"b"}}]}`,
			"no repositories": `{"list":{"source":"trendshift","period":"daily"},"fetched_at":"2026-11-19T09:15:00Z","entries":[]}`,
			"no time":         `{"list":{"source":"trendshift","period":"daily"},"entries":[{"rank":1,"repo":{"owner":"a","name":"b"}}]}`,
		} {
			require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
			_, err = Open(dir)
			require.Error(t, err, name)
			assert.Contains(t, err.Error(), path, name)
		}
	})

	t.Run("a file edited by hand may lack the last line end", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, MarksFile), []byte(valid+"\n\n"+valid), 0o600))
		s, err := Open(dir)
		require.NoError(t, err)
		require.Len(t, s.Marks(), 2)

		require.NoError(t, s.AppendMarks(mark(morning, Seen, "astral-sh/uv", "")))
		reopened, err := Open(dir)
		require.NoError(t, err)
		assert.Len(t, reopened.Marks(), 3)
	})

	t.Run("what a cut write of a snapshot left is not a snapshot", func(t *testing.T) {
		dir := t.TempDir()
		s, err := Open(dir)
		require.NoError(t, err)
		saved := snapshot(daily, morning, "golang/go")
		require.NoError(t, s.SaveSnapshot(saved))
		require.NoError(t, os.WriteFile(filepath.Join(dir, SnapshotsDir, "trendshift", "daily", ".tmp-123"), []byte(`{"list"`), 0o600))

		reopened, err := Open(dir)
		require.NoError(t, err)
		got, found := reopened.Latest(daily)
		require.True(t, found)
		assert.Equal(t, saved, got)
	})
}

func TestAppendThatFailsLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	require.NoError(t, err)
	require.NoError(t, s.AppendMarks(mark(morning, Skip, "golang/go", "")))

	t.Run("an invalid mark takes the whole write back", func(t *testing.T) {
		err := s.AppendMarks(
			mark(morning, Seen, "astral-sh/uv", ""),
			mark(morning, Action("star"), "ollama/ollama", ""),
		)
		require.Error(t, err)
		assert.Len(t, s.Marks(), 1)
	})

	t.Run("a file that cannot be written", func(t *testing.T) {
		path := filepath.Join(dir, MarksFile)
		backup := filepath.Join(dir, "marks.backup")
		require.NoError(t, os.Rename(path, backup))
		require.NoError(t, os.Mkdir(path, 0o700))

		require.Error(t, s.AppendMarks(mark(morning, Interesting, "ollama/ollama", "try")))
		assert.Len(t, s.Marks(), 1)
		assert.Empty(t, s.Bookmarks())

		require.NoError(t, os.Remove(path))
		require.NoError(t, os.Rename(backup, path))
		reopened, err := Open(dir)
		require.NoError(t, err)
		assert.Len(t, reopened.Marks(), 1)
	})

	t.Run("a snapshot without a time or a repository is not stored", func(t *testing.T) {
		assert.Error(t, s.SaveSnapshot(snapshot(daily, time.Time{}, "golang/go")))
		assert.Error(t, s.SaveSnapshot(snapshot(daily, morning)))
		_, found := s.Latest(daily)
		assert.False(t, found)
		assert.NoDirExists(t, filepath.Join(dir, SnapshotsDir, "trendshift", "daily"))
	})
}

func TestOlderSnapshotDoesNotReplaceTheLatest(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	require.NoError(t, err)
	newer := snapshot(daily, morning.Add(time.Hour), "astral-sh/uv")
	require.NoError(t, s.SaveSnapshot(newer))
	require.NoError(t, s.SaveSnapshot(snapshot(daily, morning, "golang/go")))

	got, _ := s.Latest(daily)
	assert.Equal(t, newer, got)
	reopened, err := Open(dir)
	require.NoError(t, err)
	got, _ = reopened.Latest(daily)
	assert.Equal(t, newer, got)
}

// Every snapshot is indexed by its time, across a reopen, and the last one
// within a span is read from its file.
func TestSnapshotsOfASpan(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	require.NoError(t, err)
	first := snapshot(daily, morning, "golang/go")
	evening := snapshot(daily, morning.Add(12*time.Hour), "astral-sh/uv")
	next := snapshot(daily, morning.Add(24*time.Hour), "denoland/deno")
	for _, snap := range []trend.Snapshot{evening, first, next, evening} {
		require.NoError(t, s.SaveSnapshot(snap))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, SnapshotsDir, "trendshift", "daily", ".tmp-1"), nil, 0o600))

	day := time.Date(2026, time.November, 19, 0, 0, 0, 0, time.UTC)
	for _, store := range []*Store{s, reopen(t, dir)} {
		assert.Equal(t, []time.Time{morning, morning.Add(12 * time.Hour), morning.Add(24 * time.Hour)}, store.Times(daily))
		assert.Empty(t, store.Times(weekly))

		got, found, err := store.LatestIn(daily, day, day.AddDate(0, 0, 1))
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, evening, got, "the last one of the day")

		got, found, err = store.LatestIn(daily, day, morning.Add(12*time.Hour))
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, first, got, "the end is not in the span")

		_, found, err = store.LatestIn(daily, day.AddDate(0, 0, -1), day)
		require.NoError(t, err)
		assert.False(t, found)
		_, found, err = store.LatestIn(weekly, day, day.AddDate(0, 0, 1))
		require.NoError(t, err)
		assert.False(t, found)
	}

	t.Run("a snapshot that cannot be read is an error that names its file", func(t *testing.T) {
		path := filepath.Join(dir, SnapshotsDir, "trendshift", "daily", morning.Format("2006-01-02T15-04-05.000Z")+".json")
		require.NoError(t, os.WriteFile(path, []byte("{"), 0o600))
		_, _, err := s.LatestIn(daily, day, morning.Add(time.Hour))
		require.Error(t, err)
		assert.Contains(t, err.Error(), path)
	})
}

func reopen(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir)
	require.NoError(t, err)
	return s
}
