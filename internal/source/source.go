// Package source loads the lists pulse watches from their sites.
package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.octolab.org/toolset/pulse/internal/source/github"
	"go.octolab.org/toolset/pulse/internal/source/trendshift"
	"go.octolab.org/toolset/pulse/internal/trend"
)

const (
	// DefaultTimeout is the time one load of a list may take, from the
	// request to the last byte of the answer.
	DefaultTimeout = 25 * time.Second

	// DefaultMaxBody is the size of the largest answer a load reads. The
	// pages of the sources are several times smaller.
	DefaultMaxBody = 8 << 20

	// UserAgent is what a load sends as its User-Agent: a common desktop
	// browser, nothing about pulse or its user.
	UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 " +
		"(KHTML, like Gecko) Version/18.0 Safari/605.1.15"

	accept   = "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8"
	language = "en-US,en;q=0.9"
)

// Sites returns the sites of the six lists.
func Sites() []trend.Site {
	return []trend.Site{trendshift.New(), github.New()}
}

// Loader loads a list from its site and reads the answer into a snapshot.
// It is safe for concurrent use.
type Loader struct {
	sites   map[trend.Source]trend.Site
	client  *http.Client
	now     func() time.Time
	timeout time.Duration
	maxBody int64
}

// Option changes how a Loader works.
type Option func(*Loader)

// WithClient sets the HTTP client of the loads. The timeout of a load is
// set by WithTimeout, not by the client.
func WithClient(client *http.Client) Option {
	return func(l *Loader) {
		if client != nil {
			l.client = client
		}
	}
}

// WithClock sets the clock that gives a snapshot its time.
func WithClock(now func() time.Time) Option {
	return func(l *Loader) {
		if now != nil {
			l.now = now
		}
	}
}

// WithTimeout sets the time one load may take in total, the reading of the
// answer included.
func WithTimeout(timeout time.Duration) Option {
	return func(l *Loader) {
		if timeout > 0 {
			l.timeout = timeout
		}
	}
}

// WithMaxBody sets the size in bytes of the largest answer a load reads.
func WithMaxBody(size int64) Option {
	return func(l *Loader) {
		if size > 0 {
			l.maxBody = size
		}
	}
}

// NewLoader returns a Loader of the lists of the sites. Without options it
// uses its own HTTP client, the system clock, DefaultTimeout and
// DefaultMaxBody.
func NewLoader(sites []trend.Site, options ...Option) *Loader {
	loader := &Loader{
		sites:   make(map[trend.Source]trend.Site, len(sites)),
		client:  &http.Client{},
		now:     time.Now,
		timeout: DefaultTimeout,
		maxBody: DefaultMaxBody,
	}
	for _, site := range sites {
		loader.sites[site.Source()] = site
	}
	for _, option := range options {
		option(loader)
	}
	return loader
}

// Load asks the site of the list for its period and returns the snapshot
// of the answer: the request, the period marker the page returned, the
// time and the repositories in the order of the list.
//
// An error names the list and says in plain words what went wrong: the
// request failed or took too long, the site answered with another status,
// the page shows another period or no repository.
func (l *Loader) Load(ctx context.Context, list trend.List) (trend.Snapshot, error) {
	snapshot, err := l.load(ctx, list)
	if err != nil {
		return trend.Snapshot{}, fmt.Errorf("%s: %w", list, err)
	}
	return snapshot, nil
}

func (l *Loader) load(ctx context.Context, list trend.List) (trend.Snapshot, error) {
	site, known := l.sites[list.Source]
	if !known {
		return trend.Snapshot{}, fmt.Errorf("pulse does not know the source %q", list.Source)
	}
	address, err := site.Request(list.Period)
	if err != nil {
		return trend.Snapshot{}, err
	}

	body, err := l.fetch(ctx, address)
	if err != nil {
		return trend.Snapshot{}, err
	}
	fetched := l.now().UTC()

	page, err := site.Parse(body, list.Period)
	if err != nil {
		return trend.Snapshot{}, err
	}
	return trend.Snapshot{
		List:      list,
		Request:   address,
		Marker:    page.Marker,
		FetchedAt: fetched,
		Entries:   page.Entries,
	}, nil
}

// fetch returns the body of the answer to a GET of the address. The timeout
// of the load covers the reading of the body too: a site that stalls in the
// middle of its answer is a failed load.
func (l *Loader) fetch(parent context.Context, address string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, l.timeout)
	defer cancel()

	explain := func(err error) error {
		switch {
		case parent.Err() != nil:
			return fmt.Errorf("the load of %s was stopped: %w", address, context.Cause(parent))
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			return fmt.Errorf("%s gave no complete answer in %s", address, l.timeout)
		default:
			return fmt.Errorf("the request failed: %w", err)
		}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("the request failed: %w", err)
	}
	request.Header.Set("User-Agent", UserAgent)
	request.Header.Set("Accept", accept)
	request.Header.Set("Accept-Language", language)

	response, err := l.client.Do(request)
	if err != nil {
		return nil, explain(err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %q, not 200 OK", address, response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, l.maxBody+1))
	if err != nil {
		return nil, explain(fmt.Errorf("the answer of %s broke off: %w", address, err))
	}
	if int64(len(body)) > l.maxBody {
		return nil, fmt.Errorf("the answer of %s is larger than %d bytes", address, l.maxBody)
	}
	return body, nil
}
