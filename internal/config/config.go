// Package config resolves where pulse keeps its configuration and its data,
// and reads the configuration file.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	// Host is the name the service answers at: .localhost names are loopback.
	Host = "pulse.localhost"
	// Port is the port of the service; AGENTS.md records the ports of the
	// local services.
	Port = 17842

	// EnvConfig names the configuration file instead of the default one.
	EnvConfig = "PULSE_CONFIG"
	// EnvDataDir names the data directory instead of the default one.
	EnvDataDir = "PULSE_DATA_DIR"
)

// Listen returns the default addresses: pulse.localhost resolves to both.
func Listen() []string {
	return []string{
		net.JoinHostPort("127.0.0.1", strconv.Itoa(Port)),
		net.JoinHostPort("::1", strconv.Itoa(Port)),
	}
}

// Path returns the configuration file: the flag, else PULSE_CONFIG, else
// $XDG_CONFIG_HOME/pulse/config.toml, else ~/.config/pulse/config.toml.
func Path(flag string, getenv func(string) string, home string) (string, error) {
	return resolve(flag, getenv(EnvConfig), getenv("XDG_CONFIG_HOME"), home, ".config", "config.toml")
}

// DataDir returns the data directory: the flag, else PULSE_DATA_DIR, else
// $XDG_DATA_HOME/pulse, else ~/.local/share/pulse.
func DataDir(flag string, getenv func(string) string, home string) (string, error) {
	return resolve(flag, getenv(EnvDataDir), getenv("XDG_DATA_HOME"), home, filepath.Join(".local", "share"), "")
}

func resolve(flag, env, xdg, home, fallback, file string) (string, error) {
	for _, explicit := range []string{flag, env} {
		if explicit != "" {
			return filepath.Abs(Expand(explicit, home))
		}
	}
	// The XDG specification ignores a relative base directory.
	if filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "pulse", file), nil
	}
	if home == "" {
		return "", errors.New("the home directory is unknown")
	}
	return filepath.Join(home, fallback, "pulse", file), nil
}

// Expand replaces a leading ~ with the home directory.
func Expand(path, home string) string {
	if path == "~" {
		return home
	}
	if rest, ok := strings.CutPrefix(path, "~/"); ok && home != "" {
		return filepath.Join(home, rest)
	}
	return path
}

// Vault is the [vault] table of the configuration.
type Vault struct {
	// Provider names what answers for the vault: "dir", the reader of the
	// bookmarks directory inside pulse, by default.
	Provider string `toml:"provider"`
	// Bookmarks is the directory of the vault bookmark notes; pulse only
	// reads it. Empty turns the vault filter off.
	Bookmarks string `toml:"bookmarks"`
}

// DefaultProvider answers for the vault when the configuration names none.
const DefaultProvider = "dir"

// GitHub is the [github] table of the configuration.
type GitHub struct {
	// Token is sent to api.github.com by the reader. The token of the
	// environment comes first; empty asks anonymously.
	Token string `toml:"token"`
}

// File is the configuration of pulse.
type File struct {
	// Path is where the configuration was looked for.
	Path string `toml:"-"`
	// Found says whether a file was there; pulse runs without one.
	Found bool `toml:"-"`

	Vault  Vault  `toml:"vault"`
	GitHub GitHub `toml:"github"`
}

// Load reads the configuration file. A missing file is an empty
// configuration; a file that is not valid TOML, or has a key pulse does not
// know, is an error. A ~ in the bookmarks directory is expanded, and a
// relative one is resolved against the directory of the file.
func Load(path, home string) (File, error) {
	file := File{Path: path}
	file.Vault.Provider = DefaultProvider
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return file, nil
	}
	if err != nil {
		return file, fmt.Errorf("config: %w", err)
	}
	file.Found = true
	if err := toml.NewDecoder(bytes.NewReader(raw)).DisallowUnknownFields().Decode(&file); err != nil {
		return file, fmt.Errorf("config %s: %s", path, explain(err))
	}
	file.GitHub.Token = strings.TrimSpace(file.GitHub.Token)
	if file.Vault.Provider = strings.TrimSpace(file.Vault.Provider); file.Vault.Provider == "" {
		file.Vault.Provider = DefaultProvider
	}
	if dir := strings.TrimSpace(file.Vault.Bookmarks); dir != "" {
		dir = Expand(dir, home)
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(filepath.Dir(path), dir)
		}
		file.Vault.Bookmarks = filepath.Clean(dir)
	} else {
		file.Vault.Bookmarks = ""
	}
	return file, nil
}

// explain adds the position to a TOML error and names the unknown keys.
func explain(err error) string {
	var strict *toml.StrictMissingError
	if errors.As(err, &strict) {
		keys := make([]string, 0, len(strict.Errors))
		for _, e := range strict.Errors {
			row, _ := e.Position()
			keys = append(keys, fmt.Sprintf("%s (line %d)", strings.Join(e.Key(), "."), row))
		}
		return "unknown key " + strings.Join(keys, ", ")
	}
	var decode *toml.DecodeError
	if errors.As(err, &decode) {
		row, column := decode.Position()
		return fmt.Sprintf("line %d, column %d: %s", row, column, decode.Error())
	}
	return err.Error()
}

// CheckLoopback accepts an address the service may listen on: a literal
// loopback IP with a port. A name, such as localhost, is not accepted: what
// it resolves to is not known before binding.
func CheckLoopback(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%q must be an address with a port: %w", address, err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("%q is not a loopback address: pulse listens on 127.0.0.1 and ::1 only", address)
	}
	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return fmt.Errorf("%q must use a port number from 0 to 65535", address)
	}
	return nil
}
