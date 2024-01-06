package hide_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	. "go.octolab.org/toolset/pulse/internal/hide"
	"go.octolab.org/toolset/pulse/internal/trend"
)

// answer is a provider that answers as it is told.
type answer struct {
	reasons [][]Reason
	status  Status
}

func (a answer) Check(context.Context, []Item) ([][]Reason, Status) { return a.reasons, a.status }

func TestFacade(t *testing.T) {
	items := []Item{{Repo: trend.Repo{Owner: "a", Name: "one"}}, {Repo: trend.Repo{Owner: "b", Name: "two"}}}
	vault := Reason{Source: "vault", Label: "In the vault", Ref: "one.md"}
	stars := Reason{Source: "stars", Label: "Starred"}

	t.Run("the reasons of every provider that answered, in their order", func(t *testing.T) {
		reasons, statuses := Facade{
			answer{reasons: [][]Reason{{vault}, nil}, status: Status{Source: "vault", State: OK}},
			answer{reasons: [][]Reason{{stars}, {stars}}, status: Status{Source: "stars", State: OK}},
		}.Check(t.Context(), items)
		assert.Equal(t, [][]Reason{{vault, stars}, {stars}}, reasons)
		assert.Equal(t, []Status{{Source: "vault", State: OK}, {Source: "stars", State: OK}}, statuses)
	})

	t.Run("a provider that failed, is off or answers for others hides nothing", func(t *testing.T) {
		reasons, statuses := Facade{
			answer{reasons: [][]Reason{{vault}, {vault}}, status: Status{Source: "vault", State: Failed, Error: "gone"}},
			answer{reasons: [][]Reason{{stars}, {stars}}, status: Status{Source: "off", State: Off}},
			answer{reasons: [][]Reason{{stars}}, status: Status{Source: "stars", State: OK}},
		}.Check(t.Context(), items)
		assert.Equal(t, [][]Reason{nil, nil}, reasons)
		assert.Equal(t, Failed, statuses[0].State)
		assert.Equal(t, Off, statuses[1].State)
		assert.Equal(t, Failed, statuses[2].State)
		assert.Equal(t, "the stars answered for 1 of 2 repositories", statuses[2].Error)
	})

	t.Run("a state the facade does not know is a failure", func(t *testing.T) {
		reasons, statuses := Facade{answer{reasons: [][]Reason{{stars}, nil}, status: Status{}}}.Check(t.Context(), items)
		assert.Equal(t, [][]Reason{nil, nil}, reasons)
		assert.Equal(t, Failed, statuses[0].State)
		assert.Equal(t, "source", statuses[0].Source)
		assert.Contains(t, statuses[0].Error, `the state ""`)
	})

	t.Run("no provider hides nothing", func(t *testing.T) {
		reasons, statuses := Facade(nil).Check(t.Context(), items)
		assert.Equal(t, [][]Reason{nil, nil}, reasons)
		assert.Empty(t, statuses)
	})
}
