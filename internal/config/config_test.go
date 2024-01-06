package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "go.octolab.org/toolset/pulse/internal/config"
)

func env(pairs map[string]string) func(string) string {
	return func(key string) string { return pairs[key] }
}

func TestPath(t *testing.T) {
	home := filepath.FromSlash("/home/me")

	tests := map[string]struct {
		flag string
		env  map[string]string
		want string
	}{
		"default":          {want: "/home/me/.config/pulse/config.toml"},
		"xdg":              {env: map[string]string{"XDG_CONFIG_HOME": "/xdg"}, want: "/xdg/pulse/config.toml"},
		"relative xdg":     {env: map[string]string{"XDG_CONFIG_HOME": "xdg"}, want: "/home/me/.config/pulse/config.toml"},
		"env":              {env: map[string]string{EnvConfig: "/etc/pulse.toml", "XDG_CONFIG_HOME": "/xdg"}, want: "/etc/pulse.toml"},
		"env with a tilde": {env: map[string]string{EnvConfig: "~/pulse.toml"}, want: "/home/me/pulse.toml"},
		"flag wins":        {flag: "/tmp/flag.toml", env: map[string]string{EnvConfig: "/etc/pulse.toml"}, want: "/tmp/flag.toml"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := Path(test.flag, env(test.env), home)
			require.NoError(t, err)
			assert.Equal(t, filepath.FromSlash(test.want), got)
		})
	}

	t.Run("no home", func(t *testing.T) {
		_, err := Path("", env(nil), "")
		assert.Error(t, err)
	})
}

func TestDataDir(t *testing.T) {
	home := filepath.FromSlash("/home/me")

	tests := map[string]struct {
		flag string
		env  map[string]string
		want string
	}{
		"default":   {want: "/home/me/.local/share/pulse"},
		"xdg":       {env: map[string]string{"XDG_DATA_HOME": "/xdg"}, want: "/xdg/pulse"},
		"env":       {env: map[string]string{EnvDataDir: "/var/pulse", "XDG_DATA_HOME": "/xdg"}, want: "/var/pulse"},
		"flag wins": {flag: "/tmp/data", env: map[string]string{EnvDataDir: "/var/pulse"}, want: "/tmp/data"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := DataDir(test.flag, env(test.env), home)
			require.NoError(t, err)
			assert.Equal(t, filepath.FromSlash(test.want), got)
		})
	}
}

func TestLoad(t *testing.T) {
	home := filepath.FromSlash("/home/me")
	write := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "config.toml")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}

	t.Run("a missing file is an empty configuration", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.toml")
		file, err := Load(path, home)
		require.NoError(t, err)
		assert.False(t, file.Found)
		assert.Equal(t, path, file.Path)
		assert.Empty(t, file.Vault.Bookmarks)
	})

	t.Run("the tilde is expanded", func(t *testing.T) {
		file, err := Load(write(t, "[vault]\nbookmarks = \"~/Vault/Bookmarks/GitHub\"\n"), home)
		require.NoError(t, err)
		assert.True(t, file.Found)
		assert.Equal(t, filepath.FromSlash("/home/me/Vault/Bookmarks/GitHub"), file.Vault.Bookmarks)
	})

	t.Run("a relative directory is resolved against the file", func(t *testing.T) {
		path := write(t, "[vault]\nbookmarks = \"notes\"\n")
		file, err := Load(path, home)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(filepath.Dir(path), "notes"), file.Vault.Bookmarks)
	})

	t.Run("an empty file turns the filter off", func(t *testing.T) {
		file, err := Load(write(t, ""), home)
		require.NoError(t, err)
		assert.True(t, file.Found)
		assert.Empty(t, file.Vault.Bookmarks)
	})

	t.Run("invalid TOML is an error that names the file and the line", func(t *testing.T) {
		path := write(t, "[vault]\nbookmarks = \n")
		_, err := Load(path, home)
		require.Error(t, err)
		assert.Contains(t, err.Error(), path)
		assert.Contains(t, err.Error(), "line 2")
	})

	t.Run("an unknown key is an error", func(t *testing.T) {
		_, err := Load(write(t, "[vault]\nbookmark = \"~/notes\"\n"), home)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "vault.bookmark")
	})

	t.Run("the token of GitHub is read without the space around it", func(t *testing.T) {
		file, err := Load(write(t, "[github]\ntoken = \" a-token \"\n"), home)
		require.NoError(t, err)
		assert.Equal(t, "a-token", file.GitHub.Token)
		assert.Empty(t, file.Vault.Bookmarks)
	})

	t.Run("a wrong type is an error", func(t *testing.T) {
		_, err := Load(write(t, "[vault]\nbookmarks = 42\n"), home)
		assert.Error(t, err)
	})

	t.Run("the defaults: the dir provider and a quarter of snooze", func(t *testing.T) {
		file, err := Load(write(t, "[vault]\nbookmarks = \"~/notes\"\n"), home)
		require.NoError(t, err)
		assert.Equal(t, "dir", file.Vault.Provider)
		assert.Equal(t, Period{Months: 3}, file.Seen.Period)

		missing, err := Load(filepath.Join(t.TempDir(), "config.toml"), home)
		require.NoError(t, err)
		assert.Equal(t, "dir", missing.Vault.Provider)
		assert.Equal(t, DefaultSnooze, missing.Seen.Period)
	})

	t.Run("a period of snooze and a provider of the vault", func(t *testing.T) {
		file, err := Load(write(t, "[vault]\nprovider = \"indexit\"\n[seen]\nsnooze = \"1 month\"\n"), home)
		require.NoError(t, err)
		assert.Equal(t, "indexit", file.Vault.Provider)
		assert.Equal(t, Period{Months: 1}, file.Seen.Period)
	})

	t.Run("a snooze that is not a period is an error", func(t *testing.T) {
		_, err := Load(write(t, "[seen]\nsnooze = \"a while\"\n"), home)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "seen.snooze")
	})
}

func TestPeriod(t *testing.T) {
	start := time.Date(2026, time.November, 20, 8, 15, 0, 0, time.UTC)
	for text, want := range map[string]Period{
		"3 months": {Months: 3}, "1 month": {Months: 1}, "1 Month": {Months: 1},
		"90 days": {Days: 90}, "1 day": {Days: 1}, "2 weeks": {Days: 14}, "1 year": {Months: 12},
	} {
		got, err := ParsePeriod(text)
		require.NoError(t, err, text)
		assert.Equal(t, want, got, text)
	}
	for _, text := range []string{"", "3", "months", "0 months", "-1 month", "1.5 months", "3 fortnights", "3 months ago"} {
		_, err := ParsePeriod(text)
		assert.Error(t, err, text)
	}

	assert.Equal(t, time.Date(2027, time.February, 20, 8, 15, 0, 0, time.UTC), DefaultSnooze.After(start, time.UTC))
	// A month without the day ends on its last day.
	assert.Equal(t, time.Date(2027, time.February, 28, 9, 0, 0, 0, time.UTC),
		DefaultSnooze.After(time.Date(2026, time.November, 30, 9, 0, 0, 0, time.UTC), time.UTC))
	assert.Equal(t, time.Date(2028, time.February, 29, 9, 0, 0, 0, time.UTC),
		Period{Months: 1}.After(time.Date(2028, time.January, 31, 9, 0, 0, 0, time.UTC), time.UTC))
	// The calendar of the zone: 00:30 on December 1 in UTC+3 is November 30
	// in UTC, and three months from it are March 1, 00:30, in UTC+3.
	moscow := time.FixedZone("UTC+3", 3*60*60)
	skip := time.Date(2026, time.November, 30, 21, 30, 0, 0, time.UTC)
	assert.Equal(t, time.Date(2027, time.March, 1, 0, 30, 0, 0, moscow).Unix(), DefaultSnooze.After(skip, moscow).Unix())
	assert.Equal(t, time.Date(2026, time.December, 15, 0, 30, 0, 0, moscow).Unix(), Period{Days: 14}.After(skip, moscow).Unix())
	assert.Equal(t, "3 months", DefaultSnooze.String())
	assert.Equal(t, "1 month", Period{Months: 1}.String())
	assert.Equal(t, "2 weeks", Period{Days: 14}.String())
	assert.Equal(t, "1 year", Period{Months: 12}.String())
	assert.Equal(t, "10 days", Period{Days: 10}.String())
}

func TestCheckLoopback(t *testing.T) {
	for _, address := range append(Listen(), "127.0.0.1:0", "127.0.0.2:8080", "[::1]:0") {
		assert.NoError(t, CheckLoopback(address), address)
	}
	for _, address := range []string{
		"0.0.0.0:17842", "[::]:17842", ":17842", "192.168.1.10:17842", "localhost:17842",
		"pulse.localhost:17842", "127.0.0.1", "127.0.0.1:http", "127.0.0.1:70000",
	} {
		assert.Error(t, CheckLoopback(address), address)
	}
}
