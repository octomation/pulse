package web_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getFlow renders a flow page.
func (s *service) getFlow(path string) string {
	s.t.Helper()
	response := s.get(path)
	require.Equal(s.t, http.StatusOK, response.Code, response.Body.String())
	return response.Body.String()
}

// The flow holds exactly the cards the page shows by default, in the order
// of the page: the front page first, then the sections.
func TestFlowHoldsThePageInItsOrder(t *testing.T) {
	s := start(t, "testdata/vault")
	s.loader.Day(t, "testdata/today.json")
	require.Equal(t, http.StatusOK, s.mark("skip", "rust-lang/rust", "").Code)

	page := s.open("/")
	require.NotEmpty(t, page.keys("shown"))
	body := s.getFlow("/flow/")
	assert.Equal(t, page.order(), flow(body))
	assert.NotContains(t, flow(body), "rust-lang/rust", "a hidden repository is not in the flow")
	assert.NotContains(t, flow(body), "astral-sh/uv", "nor one in the vault")

	// Only the first card is on the screen; the others wait.
	assert.Contains(t, body, `data-place="0">`)
	assert.Contains(t, body, `data-place="1" hidden>`)
	assert.Contains(t, body, `<b data-flow="place">1</b> of <b data-flow="total">`)
	assert.Contains(t, body, `id="flow-end" data-flow="end" hidden>`)

	// The answers and their keys, always in the bar.
	for _, hook := range []string{
		`data-answer="skip" data-key="ArrowLeft"`, `data-answer="later" data-key="ArrowDown"`,
		`data-answer="interesting" data-key="ArrowRight"`, `name="comment"`, `src="/assets/flow.js"`,
	} {
		assert.Contains(t, body, hook)
	}
	assert.NotContains(t, body, `data-action="seen-all"`, "no Mark all shown as seen in the flow")
	assert.Equal(t, "text/javascript; charset=utf-8", s.get("/assets/flow.js").Header().Get("Content-Type"))
}

// Left skips, right is interesting with the comment, down marks nothing: the
// flow sends what the page sends, and a new flow holds the card left for
// later in its place.
func TestFlowAnswers(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/today.json")
	cards := flow(s.getFlow("/flow/"))
	require.GreaterOrEqual(t, len(cards), 4)
	first, second, third := cards[0], cards[1], cards[2]

	require.Equal(t, http.StatusOK, s.mark("skip", first, "").Code)
	require.Equal(t, http.StatusOK, s.mark("interesting", second, "for the hub").Code)
	// Later on the third: nothing is sent.

	marks := s.store.Marks()
	require.Len(t, marks, 2)
	assert.Equal(t, "skip", string(marks[0].Action))
	assert.Equal(t, "interesting", string(marks[1].Action))
	assert.Equal(t, "for the hub", marks[1].Comment)

	// The paper lays the cards out again without the two answered ones, so
	// the others may move between sections; the one left for later is still
	// first, and the flow is the page.
	again := flow(s.getFlow("/flow/"))
	assert.ElementsMatch(t, cards[2:], again)
	assert.Equal(t, third, again[0], "the card left for later is in the next flow, in its place")
	assert.Equal(t, s.open("/").order(), again)
}

// The flow of a past day holds the cards of that day, and its answers are
// made on that day.
func TestFlowOfAPastDay(t *testing.T) {
	s := start(t, "")
	threeDays(s)
	page := s.open("/days/2026-11-17/")
	body := s.getFlow("/days/2026-11-17/flow/")
	assert.Equal(t, page.order(), flow(body))
	assert.Contains(t, body, `data-base="/days/2026-11-17/" data-day="2026-11-17"`)
	assert.Contains(t, body, `href="/days/2026-11-17/" data-flow="back"`)
}

// A flow without a card says so and links back to the page.
func TestFlowWithoutCards(t *testing.T) {
	s := start(t, "")
	s.loader.Day(t, "testdata/yesterday.json")
	require.Equal(t, http.StatusOK, s.seenAll(s.open("/")).Code)

	body := s.getFlow("/flow/")
	assert.Empty(t, flow(body))
	assert.Contains(t, body, `id="flow-end" data-flow="end">`)
	assert.Contains(t, body, `data-empty="clear"><h2>Nothing new</h2>`)
	assert.Contains(t, body, `href="/" data-flow="page">Back to the page</a>`)
	assert.Contains(t, body, `id="flow-bar" hidden>`)
}
