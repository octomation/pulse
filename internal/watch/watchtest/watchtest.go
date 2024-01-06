// Package watchtest holds what the tests of trend-watch share: a loader
// that never uses the network and a clock that moves when told.
package watchtest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"go.octolab.org/toolset/pulse/internal/trend"
)

// Loader is a watch.Loader that answers with what the test gave it and
// counts the loads.
type Loader struct {
	mu      sync.Mutex
	entries map[trend.List][]trend.Entry
	errors  map[trend.List]error
	calls   map[trend.List]int
}

// NewLoader returns a loader that fails every list until it is given one.
func NewLoader() *Loader {
	return &Loader{
		entries: map[trend.List][]trend.Entry{},
		errors:  map[trend.List]error{},
		calls:   map[trend.List]int{},
	}
}

// Set makes the list answer with the entries; none is an empty answer.
func (l *Loader) Set(list trend.List, entries ...trend.Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries[list] = entries
	delete(l.errors, list)
}

// Fail makes the list fail with the error.
func (l *Loader) Fail(list trend.List, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors[list] = err
}

// Day makes every list answer as the fixture file says: a JSON object that
// maps the name of a list, such as "trendshift/daily", to its entries. A
// list the file does not have fails.
func (l *Loader) Day(t testing.TB, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	var day map[string][]trend.Entry
	if err := json.Unmarshal(raw, &day); err != nil {
		t.Fatalf("fixture %s: %v", path, err)
	}
	known := map[string]bool{}
	for _, list := range trend.Lists() {
		known[list.String()] = true
		if entries, found := day[list.String()]; found {
			l.Set(list, entries...)
		} else {
			l.Fail(list, fmt.Errorf("the fixture %s has no %s", path, list))
		}
	}
	for name := range day {
		if !known[name] {
			t.Fatalf("fixture %s: unknown list %q", path, name)
		}
	}
}

// Calls returns how many times the list was loaded.
func (l *Loader) Calls(list trend.List) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.calls[list]
}

// Total returns how many loads there were in all.
func (l *Loader) Total() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	var total int
	for _, calls := range l.calls {
		total += calls
	}
	return total
}

// Load implements watch.Loader. The snapshot has no time: the service gives
// it the time of its clock.
func (l *Loader) Load(_ context.Context, list trend.List) (trend.Snapshot, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls[list]++
	if err := l.errors[list]; err != nil {
		return trend.Snapshot{}, err
	}
	entries, found := l.entries[list]
	if !found {
		return trend.Snapshot{}, fmt.Errorf("%s: the test gave no answer", list)
	}
	request, marker := Request(list)
	return trend.Snapshot{List: list, Request: request, Marker: marker, Entries: entries}, nil
}

// Request returns the request of a list and the period marker its source
// returns, as the issue of trend-watch records them.
func Request(list trend.List) (request, marker string) {
	if list.Source == trend.GitHub {
		ranges := map[trend.Period]string{trend.Daily: "Today", trend.Weekly: "This week", trend.Monthly: "This month"}
		return "https://github.com/trending?since=" + string(list.Period), "Date range: " + ranges[list.Period]
	}
	if list.Period == trend.Daily {
		return "https://trendshift.io/", "https://trendshift.io Daily"
	}
	names := map[trend.Period]string{trend.Weekly: "Weekly", trend.Monthly: "Monthly"}
	address := "https://trendshift.io/" + string(list.Period)
	return address, address + " " + names[list.Period]
}

// Entries ranks the repositories, given as owner/name, in their order.
func Entries(t testing.TB, names ...string) []trend.Entry {
	t.Helper()
	entries := make([]trend.Entry, 0, len(names))
	for i, name := range names {
		repo, err := trend.ParseRepo(name)
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, trend.Entry{Rank: i + 1, Repo: repo})
	}
	return entries
}

// Clock is a clock that moves when told.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

// NewClock returns a clock that shows the time.
func NewClock(now time.Time) *Clock { return &Clock{now: now} }

// Now returns the time of the clock.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Set moves the clock to the time.
func (c *Clock) Set(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = now
}

// Advance moves the clock forward.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
