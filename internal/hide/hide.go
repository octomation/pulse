// Package hide is the one place that answers "may this repository be
// hidden, and why": a facade over the providers that know what the user has
// already seen elsewhere, such as the notes of the vault.
package hide

import (
	"context"
	"fmt"
	"time"

	"go.octolab.org/toolset/pulse/internal/trend"
)

// Item is a repository of the view, as a provider is asked about it.
type Item struct {
	Repo trend.Repo
}

// Reason says why a repository may be hidden.
type Reason struct {
	// Source names the provider's source, such as "vault".
	Source string
	// Label says what the reason means on the page, such as "In the vault".
	Label string
	// Ref points at what holds the repository there, such as the note.
	Ref string
	// At is the time of that entry; zero when the source does not know it.
	At time.Time
}

// State is the state of a provider.
type State string

// The states of a provider.
const (
	// Off says that the source is not configured: it hides nothing, and that
	// is expected.
	Off State = "off"
	// OK says that the source answered.
	OK State = "ok"
	// Failed says that the source did not answer: it hides nothing, and the
	// view may include what is already there.
	Failed State = "failed"
)

// Status is how a provider answered.
type Status struct {
	// Source names the source, such as "vault".
	Source string
	// Provider names the implementation that answered, such as "dir".
	Provider string
	State    State
	// Origin is what the provider read: a path or a URL.
	Origin string
	// Entries is the number of repositories the source holds.
	Entries int
	// Newest is the time of the newest entry of the source; zero when it is
	// not known.
	Newest time.Time
	// Error says why the provider failed.
	Error string
	// Warning says what looks wrong although the provider answered.
	Warning string

	// Documents is the number of documents the provider read, such as the
	// notes of the vault; zero when it does not count them.
	Documents int
	// Broken names the documents that could not be read.
	Broken []string
}

// Provider answers for a source. It is given all the repositories of a view
// at once and returns, for each, the reasons it may be hidden: the answer
// has the length and the order of items. A provider that fails returns the
// Failed state and no reasons.
type Provider interface {
	Check(ctx context.Context, items []Item) ([][]Reason, Status)
}

// Facade asks every provider and joins their answers.
type Facade []Provider

// Check returns, for each item, the reasons of every provider that
// answered, and the status of each provider in their order. A provider that
// failed, or answered for another number of repositories, hides nothing:
// the facade fails open.
func (f Facade) Check(ctx context.Context, items []Item) ([][]Reason, []Status) {
	reasons := make([][]Reason, len(items))
	statuses := make([]Status, 0, len(f))
	for _, provider := range f {
		answer, status := provider.Check(ctx, items)
		if status.Source == "" {
			status.Source = "source"
		}
		switch {
		case status.State != OK && status.State != Off && status.State != Failed:
			status.Error = fmt.Sprintf("the %s answered with the state %q", status.Source, status.State)
			status.State = Failed
			answer = nil
		case status.State != OK:
			answer = nil
		case len(answer) != len(items):
			status.State = Failed
			status.Error = fmt.Sprintf("the %s answered for %d of %d repositories", status.Source, len(answer), len(items))
			answer = nil
		}
		for i, found := range answer {
			reasons[i] = append(reasons[i], found...)
		}
		statuses = append(statuses, status)
	}
	return reasons, statuses
}
