package trend_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "go.octolab.org/toolset/pulse/internal/trend"
)

func TestParseRepo(t *testing.T) {
	tests := map[string]struct {
		input string
		want  Repo
		key   string
	}{
		"owner and name": {
			input: "Owner/Name",
			want:  Repo{Owner: "Owner", Name: "Name"},
			key:   "owner/name",
		},
		"owner and name in spaces": {
			input: "  Owner/Name\n",
			want:  Repo{Owner: "Owner", Name: "Name"},
			key:   "owner/name",
		},
		"URL": {
			input: "https://github.com/Owner/Name",
			want:  Repo{Owner: "Owner", Name: "Name"},
			key:   "owner/name",
		},
		"URL with a trailing slash": {
			input: "https://github.com/Owner/Name/",
			want:  Repo{Owner: "Owner", Name: "Name"},
			key:   "owner/name",
		},
		"URL of the clone": {
			input: "https://github.com/Owner/Name.git",
			want:  Repo{Owner: "Owner", Name: "Name"},
			key:   "owner/name",
		},
		"URL with a query": {
			input: "https://github.com/Owner/Name?utm_source=trendshift.io",
			want:  Repo{Owner: "Owner", Name: "Name"},
			key:   "owner/name",
		},
		"URL on www and in capitals": {
			input: "HTTPS://WWW.GitHub.com/Owner/Name",
			want:  Repo{Owner: "Owner", Name: "Name"},
			key:   "owner/name",
		},
		"name with dots and dashes": {
			input: "vercel-labs/next.js-app_2",
			want:  Repo{Owner: "vercel-labs", Name: "next.js-app_2"},
			key:   "vercel-labs/next.js-app_2",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			repo, err := ParseRepo(test.input)
			require.NoError(t, err)
			assert.Equal(t, test.want, repo)
			assert.Equal(t, test.key, repo.Key())
			assert.Equal(t, test.want.Owner+"/"+test.want.Name, repo.String())
			assert.Equal(t, "https://github.com/"+test.want.Owner+"/"+test.want.Name, repo.URL())
		})
	}
}

func TestParseRepo_rejects(t *testing.T) {
	tests := map[string]string{
		"nothing":                  "",
		"owner only":               "owner",
		"owner with a slash":       "owner/",
		"name only":                "/name",
		"deeper path":              "owner/name/issues",
		"space inside":             "owner/na me",
		"URL of an owner":          "https://github.com/owner",
		"URL of an owner, slashed": "https://github.com/owner/",
		"URL of the site":          "https://github.com/",
		"URL of a deeper page":     "https://github.com/owner/name/tree/main",
		"another host":             "https://gitlab.com/owner/name",
		"a host that looks alike":  "https://github.com.example.org/owner/name",
		"a subdomain":              "https://gist.github.com/owner/name",
		"broken URL":               "https://github.com/%zz/name",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			repo, err := ParseRepo(input)
			require.Error(t, err)
			assert.Zero(t, repo)
		})
	}
}

func TestRepo_Key(t *testing.T) {
	upper := Repo{Owner: "BaaRai", Name: "ARTEX"}
	lower := Repo{Owner: "baarai", Name: "artex"}

	assert.Equal(t, "baarai/artex", upper.Key())
	assert.Equal(t, upper.Key(), lower.Key())
	assert.NotEqual(t, upper.String(), lower.String())
	assert.NotEqual(t, upper.Key(), Repo{Owner: "baarai", Name: "artex2"}.Key())
}

func TestLists(t *testing.T) {
	lists := Lists()
	names := make([]string, 0, len(lists))
	for _, list := range lists {
		names = append(names, list.String())
	}
	assert.Equal(t, []string{
		"trendshift/daily", "trendshift/weekly", "trendshift/monthly",
		"github/daily", "github/weekly", "github/monthly",
	}, names)
}
