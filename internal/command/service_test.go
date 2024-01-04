package command_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "go.octolab.org/toolset/pulse/internal/command"
	"go.octolab.org/toolset/pulse/internal/watch/watchtest"
)

// output collects what a command prints while it runs.
type output struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (o *output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buffer.Write(p)
}

func (o *output) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buffer.String()
}

// isolate keeps a command away from the configuration and the data of the
// user: whatever it resolves, it resolves inside a temporary directory.
func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("PULSE_CONFIG", "")
	t.Setenv("PULSE_DATA_DIR", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	return home
}

// server is the server command as it runs.
type server struct {
	t      *testing.T
	stdout *output
	stderr *output
	cancel context.CancelFunc
	done   chan error
	// addr is the address the service says it listens on, without the
	// trailing slash.
	addr string
}

var listens = regexp.MustCompile(`(?m)^pulse listens on (http://pulse\.localhost:\d+)/$`)

// serve runs the server command with the arguments and waits until it says
// where it listens.
func serve(t *testing.T, args ...string) *server {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	s := &server{t: t, stdout: &output{}, stderr: &output{}, cancel: cancel, done: make(chan error, 1)}
	root := NewServer()
	root.SetArgs(args)
	root.SetOut(s.stdout)
	root.SetErr(s.stderr)
	go func() { s.done <- root.ExecuteContext(ctx) }()
	t.Cleanup(func() { _ = s.stop() })

	require.Eventually(t, func() bool {
		select {
		case err := <-s.done:
			s.done <- err
			require.NoError(t, err, "the service stopped: %s", s.stderr.String())
			return false
		default:
		}
		match := listens.FindStringSubmatch(s.stdout.String())
		if match == nil {
			return false
		}
		s.addr = match[1]
		return true
	}, 10*time.Second, 5*time.Millisecond, "the service did not say where it listens")
	return s
}

// stop ends the service as SIGINT does and returns what the command
// returned.
func (s *server) stop() error {
	s.cancel()
	select {
	case err := <-s.done:
		s.done <- err
		return err
	case <-time.After(10 * time.Second):
		s.t.Fatal("the service did not stop")
		return nil
	}
}

// loopback returns the loopback address behind the name of the service.
func (s *server) loopback() string {
	return "http://127.0.0.1:" + s.addr[strings.LastIndex(s.addr, ":")+1:]
}

// client runs the client command and returns what it printed.
func client(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errs bytes.Buffer
	root := NewClient()
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(&errs)
	err = root.ExecuteContext(t.Context())
	return out.String(), errs.String(), err
}

func request(t *testing.T, method, url, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, url, strings.NewReader(body))
	require.NoError(t, err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, string(raw)
}

// "Interesting" with a comment creates a bookmark that survives a restart
// and comes out of `pulsectl bookmarks` as a URL line and as JSONL with the
// comment: the two commands as the binaries run them, the service on a port
// of its own and a temporary data directory.
func TestBookmarksThroughTheCommands(t *testing.T) {
	home := isolate(t)
	loader := watchtest.NewLoader()
	loader.Day(t, "testdata/today.json")
	t.Cleanup(SetLoader(loader))

	data := filepath.Join(t.TempDir(), "pulse-data")
	config := filepath.Join(t.TempDir(), "config.toml")
	vault := filepath.Join(t.TempDir(), "Bookmarks")
	require.NoError(t, os.Mkdir(vault, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(vault, "ollama.md"), []byte("---\nurl: https://github.com/ollama/ollama\n---\n"), 0o600))
	require.NoError(t, os.WriteFile(config, []byte("[vault]\nbookmarks = \""+vault+"\"\n"), 0o600))
	args := []string{"--listen", "127.0.0.1:0", "--data-dir", data, "--config", config}

	service := serve(t, args...)
	assert.Equal(t, "pulse listens on "+service.addr+"/\ndata: "+data+"\nvault: "+vault+" (config "+config+")\n"+
		"github: the reader asks api.github.com without a token, about 30 repositories an hour\n",
		service.stdout.String())

	// Nothing is bookmarked yet: the export is empty, and it is a success.
	stdout, _, err := client(t, "bookmarks", "--addr", service.addr)
	require.NoError(t, err)
	assert.Empty(t, stdout)

	// The page shows the lists of the loader, without what is in the vault.
	status, page := request(t, http.MethodGet, service.loopback()+"/", "")
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, page, `data-repo="ghostty-org/ghostty" data-state="shown"`)
	assert.NotContains(t, page, `data-repo="ollama/ollama"`)
	assert.Contains(t, page, "Vault: 1 repository in 1 note")
	assert.Equal(t, 6, loader.Total())

	status, body := request(t, http.MethodPost, service.loopback()+"/api/marks",
		`{"action":"interesting","repo":"ghostty-org/ghostty","comment":"try for the hub"}`)
	require.Equal(t, http.StatusOK, status, body)
	status, body = request(t, http.MethodPost, service.loopback()+"/api/marks",
		`{"action":"skip","repo":"zed-industries/zed","comment":""}`)
	require.Equal(t, http.StatusOK, status, body)

	const (
		wantURLs  = "https://github.com/ghostty-org/ghostty\n"
		wantJSONL = `^\{"url":"https://github\.com/ghostty-org/ghostty","repo":"ghostty-org/ghostty","comment":"try for the hub",` +
			`"bookmarked_at":"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ","lists":\["trendshift/daily","github/daily"\]\}\n$`
	)
	export := func(addr string) {
		t.Helper()
		stdout, stderr, err := client(t, "bookmarks", "--addr", addr)
		require.NoError(t, err, stderr)
		assert.Equal(t, wantURLs, stdout)

		stdout, stderr, err = client(t, "bookmarks", "--format", "urls", "--addr", addr+"/")
		require.NoError(t, err, stderr)
		assert.Equal(t, wantURLs, stdout)

		stdout, stderr, err = client(t, "bookmarks", "--format", "jsonl", "--addr", addr)
		require.NoError(t, err, stderr)
		assert.Regexp(t, wantJSONL, stdout)
	}
	export(service.addr)
	export(service.loopback())

	require.NoError(t, service.stop(), "the service stops cleanly")

	t.Run("without a running service the client says so", func(t *testing.T) {
		stdout, stderr, err := client(t, "bookmarks", "--addr", service.addr)
		require.Error(t, err)
		assert.Empty(t, stdout)
		assert.Equal(t, "the pulse service is not running at "+service.addr+": start it with `pulse`", err.Error())
		assert.Equal(t, "Error: "+err.Error()+"\n", stderr, "one line, and no usage")
	})

	t.Run("the bookmark survives a restart", func(t *testing.T) {
		restarted := serve(t, args...)
		export(restarted.addr)

		status, page := request(t, http.MethodGet, restarted.loopback()+"/?hidden=1", "")
		require.Equal(t, http.StatusOK, status)
		assert.Contains(t, page, `data-repo="ghostty-org/ghostty" data-state="hidden"`)
		assert.Contains(t, page, "Bookmarked in pulse: <q>try for the hub</q>")
		assert.Contains(t, page, `data-repo="zed-industries/zed" data-state="hidden"`)
		assert.Equal(t, 6, loader.Total(), "within the hour the restarted service loads nothing")
		require.NoError(t, restarted.stop())
	})

	t.Run("nothing was written outside the data directory", func(t *testing.T) {
		entries, err := os.ReadDir(home)
		require.NoError(t, err)
		assert.Empty(t, entries, "the home directory")
		entries, err = os.ReadDir(vault)
		require.NoError(t, err)
		assert.Len(t, entries, 1, "the vault directory")
		assert.FileExists(t, filepath.Join(data, "marks.jsonl"))
		assert.FileExists(t, filepath.Join(data, "attempts.jsonl"))
		assert.DirExists(t, filepath.Join(data, "snapshots", "trendshift", "daily"))
	})
}

func TestServerWithoutConfig(t *testing.T) {
	home := isolate(t)
	loader := watchtest.NewLoader()
	loader.Day(t, "testdata/today.json")
	t.Cleanup(SetLoader(loader))

	// The defaults of the environment: no flag names the files.
	service := serve(t, "--listen", "127.0.0.1:0", "--listen", "127.0.0.1:0")
	lines := strings.Split(strings.TrimSpace(service.stdout.String()), "\n")
	require.Len(t, lines, 5, service.stdout.String())
	assert.Equal(t, "github: the reader asks api.github.com without a token, about 30 repositories an hour", lines[4])
	assert.Regexp(t, `^pulse listens on http://pulse\.localhost:\d+/$`, lines[0])
	assert.Regexp(t, `^pulse listens on http://pulse\.localhost:\d+/$`, lines[1])
	assert.NotEqual(t, lines[0], lines[1])
	assert.Equal(t, "data: "+filepath.Join(home, "data", "pulse"), lines[2])
	assert.Equal(t, "vault: not configured (no config at "+filepath.Join(home, "config", "pulse", "config.toml")+")", lines[3])

	status, page := request(t, http.MethodGet, service.loopback()+"/", "")
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, page, "The vault filter is off")

	// The name of the service is what the browser sends as Host.
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, service.loopback()+"/api/bookmarks", http.NoBody)
	require.NoError(t, err)
	req.Host = "evil.example"
	response, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	assert.Equal(t, http.StatusMisdirectedRequest, response.StatusCode)

	require.NoError(t, service.stop())
	assert.NoFileExists(t, filepath.Join(home, "config", "pulse", "config.toml"), "pulse writes no configuration")
}

// pulse refuses to listen on an address that is not loopback, before it
// reads, writes or binds anything.
func TestServerRefusesWhatIsNotLoopback(t *testing.T) {
	isolate(t)
	loader := watchtest.NewLoader()
	t.Cleanup(SetLoader(loader))

	for _, address := range []string{"0.0.0.0:17899", "[::]:17899", ":17899", "192.0.2.1:17899", "localhost:17899", "pulse.localhost:17899"} {
		t.Run(address, func(t *testing.T) {
			data := filepath.Join(t.TempDir(), "data")
			var stdout, stderr bytes.Buffer
			root := NewServer()
			// A loopback address next to it does not make the other one
			// acceptable.
			root.SetArgs([]string{"--listen", "127.0.0.1:0", "--listen", address, "--data-dir", data, "--config", filepath.Join(t.TempDir(), "config.toml")})
			root.SetOut(&stdout)
			root.SetErr(&stderr)

			err := root.ExecuteContext(t.Context())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--listen")
			assert.Contains(t, err.Error(), address)
			assert.Empty(t, stdout.String(), "it never said that it listens")
			assert.NoDirExists(t, data, "it wrote nothing")
		})
	}
	assert.Zero(t, loader.Total())
}

func TestServerFailsOnInvalidState(t *testing.T) {
	isolate(t)
	t.Cleanup(SetLoader(watchtest.NewLoader()))
	run := func(t *testing.T, args ...string) error {
		t.Helper()
		var stdout, stderr bytes.Buffer
		root := NewServer()
		root.SetArgs(append([]string{"--listen", "127.0.0.1:0"}, args...))
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		err := root.ExecuteContext(t.Context())
		assert.Empty(t, stdout.String())
		return err
	}

	t.Run("an invalid configuration", func(t *testing.T) {
		config := filepath.Join(t.TempDir(), "config.toml")
		require.NoError(t, os.WriteFile(config, []byte("[vault\n"), 0o600))
		err := run(t, "--config", config, "--data-dir", t.TempDir())
		require.Error(t, err)
		assert.Contains(t, err.Error(), config)
	})

	t.Run("a corrupt mark", func(t *testing.T) {
		data := t.TempDir()
		marks := filepath.Join(data, "marks.jsonl")
		require.NoError(t, os.WriteFile(marks, []byte("{oops\n"), 0o600))
		err := run(t, "--config", filepath.Join(t.TempDir(), "config.toml"), "--data-dir", data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), marks+":1: ")
	})

	t.Run("a port that is taken", func(t *testing.T) {
		listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer func() { _ = listener.Close() }()
		err = run(t, "--listen", listener.Addr().String(), "--config", filepath.Join(t.TempDir(), "config.toml"), "--data-dir", t.TempDir())
		assert.Error(t, err)
	})
}

func TestClientArguments(t *testing.T) {
	for name, args := range map[string][]string{
		"an unknown format":  {"bookmarks", "--format", "csv"},
		"not an address":     {"bookmarks", "--addr", "pulse.localhost:17842"},
		"another scheme":     {"bookmarks", "--addr", "https://pulse.localhost:17842"},
		"a path":             {"bookmarks", "--addr", "http://pulse.localhost:17842/api"},
		"an argument":        {"bookmarks", "all"},
		"an unknown command": {"export"},
	} {
		t.Run(name, func(t *testing.T) {
			stdout, _, err := client(t, args...)
			require.Error(t, err)
			assert.Empty(t, stdout)
		})
	}

	t.Run("without a command it shows what it can do", func(t *testing.T) {
		stdout, _, err := client(t)
		require.NoError(t, err)
		assert.Contains(t, stdout, "bookmarks")
	})

	t.Run("what answers is not pulse", func(t *testing.T) {
		listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		other := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.NotFoundHandler()}
		go func() { _ = other.Serve(listener) }()
		defer func() { _ = other.Close() }()

		stdout, _, err := client(t, "bookmarks", "--addr", "http://"+listener.Addr().String())
		require.Error(t, err)
		assert.Empty(t, stdout)
		assert.Contains(t, err.Error(), "404 Not Found")
	})

	t.Run("help", func(t *testing.T) {
		stdout, _, err := client(t, "bookmarks", "--help")
		require.NoError(t, err)
		assert.Contains(t, stdout, "--format")
		assert.Contains(t, stdout, "--addr")
		assert.Contains(t, stdout, "http://pulse.localhost:17842")
		assert.NotContains(t, stdout, "%template%")
	})
}

// The token of the environment is named by its variable and never printed.
func TestServerNamesTheTokenWithoutPrintingIt(t *testing.T) {
	isolate(t)
	t.Setenv("GH_TOKEN", "a-secret-of-the-test")
	t.Cleanup(SetLoader(watchtest.NewLoader()))

	service := serve(t, "--listen", "127.0.0.1:0")
	assert.Contains(t, service.stdout.String(), "github: the reader asks api.github.com with the token of GH_TOKEN\n")
	assert.NotContains(t, service.stdout.String(), "a-secret-of-the-test")
}
