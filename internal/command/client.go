package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"go.octolab.org/toolset/pulse/internal/config"
	"go.octolab.org/toolset/pulse/internal/web"
)

// NewClient returns the new client command: pulsectl, which asks the running
// service.
func NewClient() *cobra.Command {
	command := cobra.Command{
		Use:   "pulsectl",
		Short: "ask the running pulse service",
		Long: "pulsectl is the client of pulse: it asks the service that runs on this\n" +
			"computer, at " + web.URL(config.Port) + " by default, and prints the answer.\n" +
			"Start the service with `pulse`.",

		Args: cobra.NoArgs,

		SilenceErrors: false,
		SilenceUsage:  true,

		// Without a command it shows what it can do; with a word that is
		// not a command, the arguments fail first.
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	command.AddCommand(newBookmarks())
	return &command
}

func newBookmarks() *cobra.Command {
	var format, addr string
	command := cobra.Command{
		Use:   "bookmarks",
		Short: "print the repositories marked as interesting",
		Long: "Prints the pulse bookmarks, oldest first: the repositories marked as\n" +
			"\"Interesting\" on the page of the running service.\n" +
			"\n" +
			"With --format urls, the default, it prints https://github.com/owner/name, one\n" +
			"per line, as copied from a Safari tab group. With --format jsonl it prints one\n" +
			"JSON object per line: url, repo, comment, bookmarked_at and lists, the lists\n" +
			"the repository was in when it was bookmarked.\n" +
			"\n" +
			"It reads from the service and changes nothing. Without a running service it\n" +
			"fails and says so.",
		Example: "  pulsectl bookmarks\n" +
			"  pulsectl bookmarks --format jsonl",

		Args: cobra.NoArgs,

		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != web.FormatURLs && format != web.FormatJSONL {
				return fmt.Errorf("--format %q: want %s or %s", format, web.FormatURLs, web.FormatJSONL)
			}
			base, err := serviceURL(addr)
			if err != nil {
				return fmt.Errorf("--addr %w", err)
			}
			base.Path = "/api/bookmarks"
			base.RawQuery = url.Values{"format": {format}}.Encode()

			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), http.NoBody)
			if err != nil {
				return err
			}
			client := newHTTPClient()
			defer client.CloseIdleConnections()
			response, err := client.Do(request)
			if err != nil {
				var dial *net.OpError
				if errors.As(err, &dial) && dial.Op == "dial" {
					return fmt.Errorf("the pulse service is not running at %s: start it with `pulse`", addr)
				}
				return fmt.Errorf("the pulse service at %s did not answer: %w", addr, err)
			}
			defer func() { _ = response.Body.Close() }()

			if response.StatusCode != http.StatusOK {
				var failure struct {
					Error string `json:"error"`
				}
				raw, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
				if json.Unmarshal(raw, &failure) != nil || failure.Error == "" {
					failure.Error = "is it pulse that listens there?"
				}
				return fmt.Errorf("%s answered %s: %s", addr, response.Status, failure.Error)
			}
			_, err = io.Copy(cmd.OutOrStdout(), response.Body)
			return err
		},
	}
	flags := command.Flags()
	flags.StringVar(&format, "format", web.FormatURLs, "output format: "+web.FormatURLs+" or "+web.FormatJSONL)
	flags.StringVar(&addr, "addr", strings.TrimSuffix(web.URL(config.Port), "/"), "address of the running service")
	return &command
}

// serviceURL reads the address of the service: http://host:port.
func serviceURL(addr string) (*url.URL, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, fmt.Errorf("%q: %w", addr, err)
	}
	if u.Scheme != "http" || u.Host == "" || strings.Trim(u.Path, "/") != "" || u.RawQuery != "" {
		return nil, fmt.Errorf("%q: want http://host:port", addr)
	}
	return u, nil
}

// newHTTPClient returns a client that reaches localhost and the .localhost
// names on loopback, 127.0.0.1 and then ::1, without asking the resolver of
// the system: not every resolver knows that such a name is this computer.
// It uses no proxy.
func newHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	return &http.Client{
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(address)
				if err != nil {
					return nil, err
				}
				name := strings.ToLower(strings.TrimSuffix(host, "."))
				if name != "localhost" && !strings.HasSuffix(name, ".localhost") {
					return dialer.DialContext(ctx, network, address)
				}
				var failures []error
				for _, loopback := range []string{"127.0.0.1", "::1"} {
					conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(loopback, port))
					if err == nil {
						return conn, nil
					}
					failures = append(failures, err)
				}
				return nil, errors.Join(failures...)
			},
		},
		// The service is asked, never followed elsewhere.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
