package source_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "go.octolab.org/toolset/pulse/internal/source"
	"go.octolab.org/toolset/pulse/internal/source/internal/fixture"
	"go.octolab.org/toolset/pulse/internal/trend"
)

const testdata = "testdata"

// web stands for the sites of the lists: a local server that answers for
// their addresses, and the transport that leads every request to it.
type web struct {
	target *url.URL
	inner  http.RoundTripper

	mu       sync.Mutex
	routes   map[string]http.HandlerFunc
	requests []*http.Request
}

// serve starts the local server. Until a test changes a route, it answers
// every list with its saved page.
func serve(t *testing.T) *web {
	t.Helper()
	sites := &web{routes: make(map[string]http.HandlerFunc)}
	for _, list := range trend.Lists() {
		saved := fixture.Load(t, testdata, list)
		sites.routes[saved.Request] = page(saved.Body)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		address := "https://" + r.Host + r.URL.RequestURI()
		sites.mu.Lock()
		handler, known := sites.routes[address]
		sites.mu.Unlock()
		if !known {
			http.NotFound(w, r)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)

	target, err := url.Parse(server.URL)
	require.NoError(t, err)
	sites.target = target

	transport := &http.Transport{}
	t.Cleanup(transport.CloseIdleConnections)
	sites.inner = transport
	return sites
}

// RoundTrip records the request as the loader made it and sends it to the
// local server under the name of its site.
func (sites *web) RoundTrip(request *http.Request) (*http.Response, error) {
	sites.mu.Lock()
	sites.requests = append(sites.requests, request)
	sites.mu.Unlock()

	local := request.Clone(request.Context())
	local.Host = request.URL.Host
	local.URL.Scheme = sites.target.Scheme
	local.URL.Host = sites.target.Host
	return sites.inner.RoundTrip(local)
}

func (sites *web) client() *http.Client { return &http.Client{Transport: sites} }

func (sites *web) route(address string, handler http.HandlerFunc) {
	sites.mu.Lock()
	defer sites.mu.Unlock()
	sites.routes[address] = handler
}

func (sites *web) requested() []string {
	sites.mu.Lock()
	defer sites.mu.Unlock()
	addresses := make([]string, 0, len(sites.requests))
	for _, request := range sites.requests {
		addresses = append(addresses, request.URL.String())
	}
	return addresses
}

func page(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(body)
	}
}

func clock(now time.Time) func() time.Time { return func() time.Time { return now } }

func TestSites(t *testing.T) {
	sources := make([]trend.Source, 0, 2)
	for _, site := range Sites() {
		sources = append(sources, site.Source())
	}
	assert.Equal(t, []trend.Source{trend.Trendshift, trend.GitHub}, sources)

	for _, list := range trend.Lists() {
		assert.Contains(t, sources, list.Source, "no site for %s", list)
	}
}

func TestLoader_Load(t *testing.T) {
	sites := serve(t)
	now := time.Date(2026, time.November, 19, 19, 45, 0, 0, time.FixedZone("MSK", 3*60*60))
	loader := NewLoader(Sites(), WithClient(sites.client()), WithClock(clock(now)))

	requests := make(map[string]trend.List)
	for _, list := range trend.Lists() {
		t.Run(list.String(), func(t *testing.T) {
			saved := fixture.Load(t, testdata, list)
			before := len(sites.requested())

			snapshot, err := loader.Load(t.Context(), list)
			require.NoError(t, err)

			assert.Equal(t, list, snapshot.List)
			assert.Equal(t, saved.Request, snapshot.Request)
			assert.Equal(t, saved.Marker, snapshot.Marker)
			assert.True(t, now.Equal(snapshot.FetchedAt), snapshot.FetchedAt)
			assert.Equal(t, time.UTC, snapshot.FetchedAt.Location())
			assert.Equal(t, saved.Repos, fixture.Names(snapshot.Entries))
			for i, entry := range snapshot.Entries {
				assert.Equal(t, i+1, entry.Rank)
			}

			assert.Equal(t, []string{saved.Request}, sites.requested()[before:], "one request, to the address of the list")
			requests[snapshot.Request] = list
		})
	}
	assert.Len(t, requests, len(trend.Lists()), "every list has its own request")
	assert.Equal(t, []string{
		"https://trendshift.io/",
		"https://trendshift.io/weekly",
		"https://trendshift.io/monthly",
		"https://github.com/trending?since=daily",
		"https://github.com/trending?since=weekly",
		"https://github.com/trending?since=monthly",
	}, sites.requested())
}

func TestLoader_Load_request(t *testing.T) {
	sites := serve(t)
	loader := NewLoader(Sites(), WithClient(sites.client()))

	_, err := loader.Load(t.Context(), trend.List{Source: trend.GitHub, Period: trend.Weekly})
	require.NoError(t, err)
	require.Len(t, sites.requests, 1)

	request := sites.requests[0]
	assert.Equal(t, http.MethodGet, request.Method)
	assert.Equal(t, "https://github.com/trending?since=weekly", request.URL.String())
	assert.Empty(t, request.Header.Get("Referer"))
	assert.Empty(t, request.Header.Get("Cookie"))
	assert.Empty(t, request.Header.Get("Authorization"))
	assert.Empty(t, request.Header.Get("From"))
	assert.True(t, strings.HasPrefix(request.Header.Get("Accept-Language"), "en"))
	_, deadline := request.Context().Deadline()
	assert.True(t, deadline, "the request has the timeout of the load")

	agent := request.Header.Get("User-Agent")
	assert.Equal(t, UserAgent, agent)
	assert.True(t, strings.HasPrefix(agent, "Mozilla/5.0 "))
	for _, private := range []string{"pulse", "tablo", "octo", "go-http-client", "http://", "https://", "@"} {
		assert.NotContains(t, strings.ToLower(agent), private)
	}
}

func TestLoader_Load_failures(t *testing.T) {
	const timeout = time.Second

	weekly := fixture.Load(t, testdata, trend.List{Source: trend.GitHub, Period: trend.Weekly})

	// stall answers with the start of a page, or with nothing, and then
	// holds the connection until the request is given up or the test ends.
	stall := func(start []byte) func(<-chan struct{}) http.HandlerFunc {
		return func(release <-chan struct{}) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				if len(start) > 0 {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					_, _ = w.Write(start)
					if flusher, ok := w.(http.Flusher); ok {
						flusher.Flush()
					}
				}
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}
		}
	}
	always := func(handler http.HandlerFunc) func(<-chan struct{}) http.HandlerFunc {
		return func(<-chan struct{}) http.HandlerFunc { return handler }
	}

	tests := map[string]struct {
		list    trend.List
		handler func(release <-chan struct{}) http.HandlerFunc
		want    []string
	}{
		"the site answers 500": {
			list: trend.List{Source: trend.GitHub, Period: trend.Daily},
			handler: always(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "<html><body>Unicorn!</body></html>", http.StatusInternalServerError)
			}),
			want: []string{
				"github/daily: ",
				`https://github.com/trending?since=daily answered "500 Internal Server Error", not 200 OK`,
			},
		},
		"the site answers 429": {
			list: trend.List{Source: trend.Trendshift, Period: trend.Daily},
			handler: always(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "slow down", http.StatusTooManyRequests)
			}),
			want: []string{
				"trendshift/daily: ",
				`https://trendshift.io/ answered "429 Too Many Requests", not 200 OK`,
			},
		},
		"the page of another period": {
			list:    trend.List{Source: trend.GitHub, Period: trend.Monthly},
			handler: always(page(weekly.Body)),
			want: []string{
				"github/monthly: ",
				"the page shows the weekly list (Date range: This week), not the monthly one",
			},
		},
		"the page of another period of trendshift": {
			list:    trend.List{Source: trend.Trendshift, Period: trend.Weekly},
			handler: always(page(fixture.Load(t, testdata, trend.List{Source: trend.Trendshift, Period: trend.Daily}).Body)),
			want: []string{
				"trendshift/weekly: ",
				"the page shows the daily list (https://trendshift.io Daily), not the weekly one",
			},
		},
		"a page without repositories": {
			list: trend.List{Source: trend.Trendshift, Period: trend.Monthly},
			handler: always(page([]byte(`<html><head><link rel="canonical" href="https://trendshift.io/monthly"/></head>` +
				`<body><nav><a aria-current="page" href="/monthly">Monthly</a></nav></body></html>`))),
			want: []string{
				"trendshift/monthly: ",
				"the page has no repositories (https://trendshift.io/monthly Monthly)",
			},
		},
		"an empty answer": {
			list:    trend.List{Source: trend.GitHub, Period: trend.Weekly},
			handler: always(page(nil)),
			want:    []string{"github/weekly: ", "the answer is empty"},
		},
		"the site stalls in the middle of the page": {
			list:    trend.List{Source: trend.GitHub, Period: trend.Weekly},
			handler: stall(weekly.Body[:len(weekly.Body)/2]),
			want: []string{
				"github/weekly: ",
				"https://github.com/trending?since=weekly gave no complete answer in 1s",
			},
		},
		"the site does not answer": {
			list:    trend.List{Source: trend.Trendshift, Period: trend.Weekly},
			handler: stall(nil),
			want: []string{
				"trendshift/weekly: ",
				"https://trendshift.io/weekly gave no complete answer in 1s",
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			sites := serve(t)
			address, known := fixture.Load(t, testdata, test.list).Request, false
			for _, site := range Sites() {
				if request, err := site.Request(test.list.Period); err == nil && request == address {
					known = true
				}
			}
			require.True(t, known, "the route is not a request of a list: %s", address)
			release := make(chan struct{})
			t.Cleanup(func() { close(release) })
			sites.route(address, test.handler(release))
			loader := NewLoader(Sites(), WithClient(sites.client()), WithTimeout(timeout))

			for _, list := range trend.Lists() {
				snapshot, err := loader.Load(t.Context(), list)
				if list != test.list {
					require.NoError(t, err, "the failure of %s is not the failure of %s", test.list, list)
					assert.NotEmpty(t, snapshot.Entries)
					continue
				}
				require.Error(t, err)
				for _, want := range test.want {
					assert.ErrorContains(t, err, want)
				}
				assert.True(t, strings.HasPrefix(err.Error(), list.String()+": "), err.Error())
				assert.Zero(t, snapshot)
			}
		})
	}
}

func TestLoader_Load_tooLarge(t *testing.T) {
	sites := serve(t)
	loader := NewLoader(Sites(), WithClient(sites.client()), WithMaxBody(64<<10))

	snapshot, err := loader.Load(t.Context(), trend.List{Source: trend.Trendshift, Period: trend.Daily})
	require.ErrorContains(t, err, "trendshift/daily: the answer of https://trendshift.io/ is larger than 65536 bytes")
	assert.Zero(t, snapshot)
}

func TestLoader_Load_stopped(t *testing.T) {
	sites := serve(t)
	loader := NewLoader(Sites(), WithClient(sites.client()))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	snapshot, err := loader.Load(ctx, trend.List{Source: trend.GitHub, Period: trend.Daily})
	require.ErrorContains(t, err, "github/daily: the load of https://github.com/trending?since=daily was stopped")
	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, snapshot)
}

func TestLoader_Load_unreachable(t *testing.T) {
	refused := errors.New("connection refused")
	client := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		return nil, refused
	})}
	loader := NewLoader(Sites(), WithClient(client))

	snapshot, err := loader.Load(t.Context(), trend.List{Source: trend.Trendshift, Period: trend.Monthly})
	require.ErrorContains(t, err, "trendshift/monthly: the request failed: ")
	require.ErrorIs(t, err, refused)
	assert.Zero(t, snapshot)
}

func TestLoader_Load_unknown(t *testing.T) {
	requested := false
	client := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		requested = true
		return nil, errors.New("no request is expected")
	})}

	tests := map[string]struct {
		sites []trend.Site
		list  trend.List
		want  string
	}{
		"a source without a site": {
			sites: Sites(),
			list:  trend.List{Source: "gitlab", Period: trend.Daily},
			want:  `gitlab/daily: pulse does not know the source "gitlab"`,
		},
		"a loader without sites": {
			sites: nil,
			list:  trend.List{Source: trend.GitHub, Period: trend.Daily},
			want:  `github/daily: pulse does not know the source "github"`,
		},
		"a period the site does not have": {
			sites: Sites(),
			list:  trend.List{Source: trend.GitHub, Period: "yearly"},
			want:  `github/yearly: github has no "yearly" list`,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			snapshot, err := NewLoader(test.sites, WithClient(client)).Load(t.Context(), test.list)
			require.EqualError(t, err, test.want)
			assert.Zero(t, snapshot)
			assert.False(t, requested)
		})
	}
}

func TestNewLoader_defaults(t *testing.T) {
	assert.Equal(t, 25*time.Second, DefaultTimeout)

	sites := serve(t)
	loader := NewLoader(Sites(),
		WithClient(nil), WithClient(sites.client()),
		WithClock(nil), WithTimeout(0), WithTimeout(-time.Second), WithMaxBody(0),
	)

	before := time.Now()
	snapshot, err := loader.Load(t.Context(), trend.List{Source: trend.Trendshift, Period: trend.Weekly})
	require.NoError(t, err)
	assert.WithinRange(t, snapshot.FetchedAt, before, time.Now())
	require.Len(t, sites.requests, 1)
	deadline, _ := sites.requests[0].Context().Deadline()
	assert.WithinDuration(t, before.Add(DefaultTimeout), deadline, 5*time.Second)
}

type roundTrip func(*http.Request) (*http.Response, error)

func (fn roundTrip) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }
