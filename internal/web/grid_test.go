package web_test

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sectionTitle = regexp.MustCompile(`<header class="section-head"><h2>([^<]+)</h2>`)

// titles returns the titles of the sections of the page, in their order.
func (p page) titles() []string {
	var titles []string
	for _, match := range sectionTitle.FindAllStringSubmatch(p.html, -1) {
		titles = append(titles, match[1])
	}
	return titles
}

// A card holds text only: its kicker, name, description and lists. The
// answers are in the reader, which walks the cards in the order of the page.
func TestCardsHoldTextOnly(t *testing.T) {
	s := start(t, "")
	threeDays(s)

	page := s.open("/days/2026-11-17/")
	order := page.order()
	// both/days leads the front page as the first of trendshift/daily; in the
	// daily section github daily #1 comes before trendshift daily #2.
	assert.Equal(t, []string{"both/days", "second/two", "second/one"}, order, "the order of the ranks")
	for _, key := range order {
		card := page.card(t, key)
		assert.NotContains(t, card.html, "data-action=\"interesting\"", key)
		assert.NotContains(t, card.html, "data-action=\"skip\"", key)
		assert.NotContains(t, card.html, `name="comment"`, key)
		assert.Contains(t, s.get("/days/2026-11-17/repos/"+key).Body.String(), `data-action="interesting"`,
			"the reader answers %s", key)
	}

	// A repository in one list does not say so: only a card in several
	// lists names how many (TestOneCardForSeveralListsAndLetterCases).
	assert.Regexp(t, `<p class="kicker">[^<]*</p>`, page.card(t, "second/one").html)
	assert.NotContains(t, page.card(t, "second/one").html, "list</p>")

	// The reader answers a card of that day, and the card leaves the page.
	require.Equal(t, http.StatusOK, s.markOn("2026-11-17", "skip", "second/one", "").Code)
	assert.Equal(t, []string{"both/days", "second/two"}, s.open("/days/2026-11-17/").order())
}

// A past day names its sections for that day; today names them for today.
func TestPastDayNamesItsSections(t *testing.T) {
	s := start(t, "")
	threeDays(s)

	assert.Equal(t, []string{"Front page", "That week"}, s.open("/days/2026-11-18/").titles())
	assert.Contains(t, s.open("/days/2026-11-17/").titles(), "That day")
	for _, title := range s.open("/").titles() {
		assert.NotContains(t, title, "That ", "today")
	}
}
