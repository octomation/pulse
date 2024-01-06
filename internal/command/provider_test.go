package command_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "go.octolab.org/toolset/pulse/internal/command"
	"go.octolab.org/toolset/pulse/internal/hide"
	"go.octolab.org/toolset/pulse/internal/watch/watchtest"
)

// shelf is a vault provider of a test: it holds the repositories it was
// given, each with its own reference.
type shelf map[string]string

func (s shelf) Check(_ context.Context, items []hide.Item) ([][]hide.Reason, hide.Status) {
	answer := make([][]hide.Reason, len(items))
	for i, item := range items {
		if ref, found := s[item.Repo.Key()]; found {
			answer[i] = []hide.Reason{{Source: "vault", Label: "On the shelf", Ref: ref}}
		}
	}
	return answer, hide.Status{Source: "vault", Provider: "shelf", State: hide.OK, Origin: "shelf://test", Entries: len(s)}
}

// The configuration chooses what answers for the vault: a provider swapped
// in by name hides its repositories with its own reasons, with no change to
// the page.
func TestTheConfigurationChoosesTheVaultProvider(t *testing.T) {
	isolate(t)
	loader := watchtest.NewLoader()
	loader.Day(t, "testdata/today.json")
	t.Cleanup(SetLoader(loader))
	t.Cleanup(SetVaultProvider("shelf", shelf{"zed-industries/zed": "shelf 3"}))

	config := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(config, []byte("[vault]\nprovider = \"shelf\"\n"), 0o600))
	service := serve(t, "--listen", "127.0.0.1:0", "--config", config, "--data-dir", t.TempDir())
	assert.Contains(t, service.stdout.String(), "vault: through shelf (config "+config+")\n")

	status, page := request(t, http.MethodGet, service.loopback()+"/hidden/", "")
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, page, `data-repo="zed-industries/zed" data-state="hidden"`)
	assert.Contains(t, page, `<li class="reason" data-reason="vault">On the shelf: shelf 3</li>`)
	assert.Contains(t, page, "shelf://test")

	t.Run("an unknown provider is an error before anything starts", func(t *testing.T) {
		unknown := filepath.Join(t.TempDir(), "config.toml")
		require.NoError(t, os.WriteFile(unknown, []byte("[vault]\nprovider = \"nowhere\"\n"), 0o600))
		root := NewServer()
		root.SetArgs([]string{"--listen", "127.0.0.1:0", "--config", unknown, "--data-dir", t.TempDir()})
		root.SetOut(&output{})
		root.SetErr(&output{})
		err := root.ExecuteContext(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), `vault.provider "nowhere" is not known`)
	})
}
