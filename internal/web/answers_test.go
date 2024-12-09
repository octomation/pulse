package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.octolab.org/toolset/pulse/internal/hide"
)

// broken is a source of notes that cannot be read.
type broken struct{}

func (broken) Check(context.Context, []hide.Item) ([][]hide.Reason, hide.Status) {
	return nil, hide.Status{Source: "vault", State: hide.Failed, Error: "the directory is gone"}
}

// The flow card is the whole story: the flow page has no reader over it,
// and its bar has Back and the comment on demand.
func TestFlowIsTheStory(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/today.json")
	body := s.getFlow("/flow/")
	assert.NotContains(t, body, `id="reader"`, "no reader opens over the flow")
	assert.NotContains(t, body, `data-action="read"`)
	for _, hook := range []string{
		`data-answer="back" data-key="ArrowUp"`, `data-action="comment" data-key="c"`,
		`<form class="flow-form" id="flow-form" hidden>`, `id="flow-stage" tabindex="-1"`,
	} {
		assert.Contains(t, body, hook)
	}
}

// A card says where it stands: the hidden view groups the cards under their
// stamps, and the reader offers the answers that fit.
func TestStampsAndGroups(t *testing.T) {
	s := start(t, "testdata/vault")
	s.loader.Day(t, "testdata/today.json")
	require.Equal(t, http.StatusOK, s.mark("interesting", "ghostty-org/ghostty", "the terminal").Code)
	require.Equal(t, http.StatusOK, s.mark("skip", "ollama/ollama", "").Code)

	hidden := s.open("/hidden/")
	for group, key := range map[string]string{
		"interesting": "ghostty-org/ghostty", "skipped": "ollama/ollama", "notes": "astral-sh/uv",
	} {
		assert.Contains(t, hidden.html, `data-group="`+group+`"`)
		assert.Contains(t, hidden.card(t, key).html, `class="standing"`, key)
	}
	assert.Contains(t, hidden.card(t, "ghostty-org/ghostty").html, `data-stamp="interesting">★ Interesting</p>`)
	assert.Contains(t, hidden.card(t, "ollama/ollama").html, `data-stamp="skip">Skipped · until`)
	assert.Contains(t, hidden.card(t, "astral-sh/uv").html, `data-stamp="notes">In your notes</p>`)

	story := s.get("/repos/ghostty-org/ghostty").Body.String()
	assert.Contains(t, story, `data-action="interesting" aria-pressed="true">Save comment</button>`)
	assert.Contains(t, story, `data-action="skip">Skip</button>`, "a skip may replace the interest")
	assert.Contains(t, story, `data-action="clear"`)

	t.Run("a repository a note names offers no answer of its own", func(t *testing.T) {
		story := s.get("/repos/astral-sh/uv").Body.String()
		assert.Contains(t, story, `data-stamp="notes"`)
		assert.NotContains(t, story, `class="actions"`, "nothing to answer, nothing to take back")

		require.Equal(t, http.StatusOK, s.mark("interesting", "astral-sh/uv", "").Code)
		story = s.get("/repos/astral-sh/uv").Body.String()
		assert.NotContains(t, story, `data-action="interesting"`)
		assert.NotContains(t, story, `data-action="skip"`)
		assert.Contains(t, story, `data-action="clear"`, "the answer of pulse can be taken back")
		assert.Contains(t, story, "the note stays")
	})

	t.Run("take back makes it new", func(t *testing.T) {
		require.Equal(t, http.StatusOK, s.mark("clear", "ollama/ollama", "").Code)
		page := s.open("/")
		assert.Contains(t, page.keys("shown"), "ollama/ollama")
	})

	t.Run("a snooze that ended is remembered in the reader", func(t *testing.T) {
		require.Equal(t, http.StatusOK, s.mark("skip", "zed-industries/zed", "").Code)
		s.clock.Advance(100 * 24 * time.Hour)
		story := s.get("/repos/zed-industries/zed").Body.String()
		assert.Contains(t, story, `data-state="shown"`)
		assert.Contains(t, story, `<p class="story-note" data-lapsed>Skipped on`)
		assert.Contains(t, story, "the snooze ended on")
	})
}

// Undo after "Mark all shown as seen" takes back exactly the marks it wrote.
func TestUndoMarkAllShownAsSeen(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/today.json")
	require.Equal(t, http.StatusOK, s.mark("skip", "ollama/ollama", "").Code)
	page := s.open("/")
	shown := page.keys("shown")
	require.NotEmpty(t, shown)

	response := s.seenAll(page)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var answer struct {
		Marked int      `json:"marked"`
		Repos  []string `json:"repos"`
		Batch  *int     `json:"batch"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	assert.Equal(t, len(shown), answer.Marked)
	assert.ElementsMatch(t, page.names("shown"), answer.Repos)
	assert.Contains(t, s.open("/hidden/").card(t, shown[0]).html, `data-stamp="seen">Seen · until`)
	assert.Empty(t, s.open("/").keys("shown"))

	require.NotNil(t, answer.Batch)
	assert.Equal(t, http.StatusBadRequest, s.post("/api/seen", map[string]any{"repos": answer.Repos, "undo": true}).Code,
		"an Undo names its batch")
	response = s.post("/api/seen", map[string]any{"repos": answer.Repos, "undo": true, "batch": *answer.Batch})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `"marked":`+jsonInt(len(shown)))
	again := s.open("/")
	assert.ElementsMatch(t, shown, again.keys("shown"))
	assert.NotContains(t, again.keys("shown"), "ollama/ollama", "a skip from before stays")
}

func jsonInt(n int) string {
	raw, _ := json.Marshal(n)
	return string(raw)
}

// The export holds the interest without a note; all=true holds every
// bookmark; an unreadable vault fails it. The queue is the same list, also
// with what left the lists.
func TestExportAndQueue(t *testing.T) {
	vault := t.TempDir()
	note, err := os.ReadFile("testdata/vault/uv.md")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(vault, "uv.md"), note, 0o600))
	s := start(t, vault)
	s.loader.Day(t, "testdata/today.json")
	for _, name := range []string{"ghostty-org/ghostty", "astral-sh/uv", "gone/away"} {
		require.Equal(t, http.StatusOK, s.mark("interesting", name, "").Code)
	}
	assert.Equal(t, "https://github.com/ghostty-org/ghostty\nhttps://github.com/gone/away\n", s.get("/api/bookmarks").Body.String())
	assert.Equal(t, "https://github.com/ghostty-org/ghostty\nhttps://github.com/astral-sh/uv\nhttps://github.com/gone/away\n",
		s.get("/api/bookmarks?all=true").Body.String())
	assert.Equal(t, http.StatusBadRequest, s.get("/api/bookmarks?all=yes").Code)

	queue := s.open("/queue/")
	assert.Equal(t, []string{"ghostty-org/ghostty", "gone/away"}, queue.keys("hidden"))
	assert.Contains(t, queue.html, `data-view="queue"`)
	assert.Contains(t, queue.card(t, "gone/away").html, `data-stamp="interesting"`)
	assert.Contains(t, queue.html, `<div class="queue-item" data-repo="gone/away" data-name="gone/away">`)
	assert.Contains(t, queue.html, `data-action="clear" title="Take back the interest`, "Take back is one click on the queue")
	story := s.get("/repos/gone/away").Body.String()
	assert.Contains(t, story, `data-stamp="interesting"`, "the reader knows the answer of a repository off the lists")
	assert.Contains(t, story, `data-action="clear"`)

	require.Equal(t, http.StatusOK, s.mark("clear", "gone/away", "").Code)
	assert.Equal(t, []string{"ghostty-org/ghostty"}, s.open("/queue/").keys("hidden"))

	t.Run("a note that cannot be read fails the export: it may name a bookmark", func(t *testing.T) {
		broken, err := os.ReadFile("testdata/vault/broken.md")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(vault, "broken.md"), broken, 0o600))
		response := s.get("/api/bookmarks")
		assert.Equal(t, http.StatusServiceUnavailable, response.Code)
		assert.Contains(t, response.Body.String(), "1 note of the vault could not be read, broken.md")
		assert.Equal(t, http.StatusOK, s.get("/api/bookmarks?all=true").Code)
		require.NoError(t, os.Remove(filepath.Join(vault, "broken.md")))
	})

	t.Run("an unreadable vault fails the export and warns on the queue", func(t *testing.T) {
		s.hide = hide.Facade{broken{}}
		s.restart()
		response := s.get("/api/bookmarks")
		assert.Equal(t, http.StatusServiceUnavailable, response.Code)
		assert.Contains(t, response.Body.String(), "the vault could not be read: the directory is gone")
		assert.Equal(t, http.StatusOK, s.get("/api/bookmarks?all=true").Code)
		queue := s.open("/queue/")
		assert.Contains(t, queue.html, "so the queue holds every bookmark")
		assert.Equal(t, []string{"ghostty-org/ghostty", "astral-sh/uv"}, queue.keys("hidden"))
	})
}
