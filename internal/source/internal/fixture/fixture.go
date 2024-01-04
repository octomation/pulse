// Package fixture reads the saved pages of the lists for the tests: the
// answer of a source as it came, and what is claimed about it.
package fixture

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.octolab.org/toolset/pulse/internal/trend"
)

// Claim is what the file next to a saved page says about it.
type Claim struct {
	// List is the name of the list, such as "github/weekly".
	List string `json:"list"`
	// Request is the URL the page was fetched from.
	Request string `json:"request"`
	// FetchedAt is the time of the fetch.
	FetchedAt time.Time `json:"fetched_at"`
	// Marker is what the page says about its period.
	Marker string `json:"marker"`
	// Repos is the whole ranking of the page, owner/name in its order.
	Repos []string `json:"repos"`
}

// Fixture is a saved page with its claim.
type Fixture struct {
	Claim
	// Body is the page, byte for byte as the source answered.
	Body []byte
}

// Load reads the saved page of the list and its claim from dir.
func Load(t testing.TB, dir string, list trend.List) Fixture {
	t.Helper()
	name := string(list.Source) + "-" + string(list.Period)

	var fixture Fixture
	claim, err := os.ReadFile(filepath.Join(dir, name+".json"))
	require.NoError(t, err)
	decoder := json.NewDecoder(bytes.NewReader(claim))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&fixture.Claim))
	require.Equal(t, list.String(), fixture.List, "the claim is about another list")

	packed, err := os.ReadFile(filepath.Join(dir, name+".html.gz"))
	require.NoError(t, err)
	reader, err := gzip.NewReader(bytes.NewReader(packed))
	require.NoError(t, err)
	fixture.Body, err = io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	return fixture
}

// Keys returns the keys of the entries in their order.
func Keys(entries []trend.Entry) []string {
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.Repo.Key())
	}
	return keys
}

// Names returns owner/name of the entries in their order, in the case of
// the list.
func Names(entries []trend.Entry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Repo.String())
	}
	return names
}
