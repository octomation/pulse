// Package command holds the commands of the two binaries: the service pulse
// and its client pulsectl.
package command

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"go.octolab.org/toolset/pulse/internal/config"
	"go.octolab.org/toolset/pulse/internal/details"
	"go.octolab.org/toolset/pulse/internal/source"
	"go.octolab.org/toolset/pulse/internal/store"
	"go.octolab.org/toolset/pulse/internal/watch"
	"go.octolab.org/toolset/pulse/internal/web"
)

// grace is how long the service waits for the responses in flight when it
// stops.
const grace = 5 * time.Second

// newLoader returns what loads the lists: the sites of trendshift.io and
// github.com/trending. The tests replace it, so they never use the network.
var newLoader = func() watch.Loader { return source.NewLoader(source.Sites()) }

// newDetails returns what asks GitHub about a repository for the reader,
// with the token of the environment when there is one. The tests replace
// it, so they never use the network.
var newDetails = func(token string) web.Details { return details.New(details.Options{Token: token}) }

// token returns the GitHub token of the environment and the variable that
// holds it.
func token(getenv func(string) string) (value, name string) {
	for _, name := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if value := getenv(name); value != "" {
			return value, name
		}
	}
	return "", ""
}

// NewServer returns the new server command: pulse, which runs the service.
func NewServer() *cobra.Command {
	var (
		listen     []string
		configPath string
		dataDir    string
	)
	address := web.URL(config.Port)
	command := cobra.Command{
		Use:   "pulse",
		Short: "serve trend-watch: the GitHub trends you have not seen yet",
		Long: "pulse serves trend-watch at " + address + ": one page with the\n" +
			"repositories of six lists, the daily, weekly and monthly trends of trendshift.io\n" +
			"and github.com/trending, that you have not seen and have not bookmarked yet.\n" +
			"\n" +
			"Opening the page loads a list from the network when its last attempt is more\n" +
			"than an hour old; \"Refresh\" loads all six now. \"Interesting\" bookmarks a\n" +
			"repository, with an optional comment, \"Skip\" and \"Mark all shown as seen\" hide\n" +
			"what you have looked at, and `pulsectl bookmarks` prints the bookmarks.\n" +
			"\n" +
			"A click on a repository opens it in the reader of the same page, with the\n" +
			"facts and the README of GitHub. pulse asks api.github.com for them when a\n" +
			"repository is opened, with the token of GITHUB_TOKEN or GH_TOKEN when one is\n" +
			"set; without a token GitHub allows about 30 repositories an hour.\n" +
			"\n" +
			"The service runs as your user and listens on loopback only, 127.0.0.1 and ::1;\n" +
			"it refuses any other address. A default address the system lacks, as ::1 with\n" +
			"IPv6 turned off, is skipped. SIGINT or SIGTERM stops it.\n" +
			"\n" +
			"Configuration: the file " + config.EnvConfig + " names, else\n" +
			"$XDG_CONFIG_HOME/pulse/config.toml, else ~/.config/pulse/config.toml. It names\n" +
			"the bookmarks directory of your notes vault, whose repositories the page hides;\n" +
			"pulse only reads that directory:\n" +
			"\n" +
			"  [vault]\n" +
			"  bookmarks = \"~/path/to/Bookmarks/GitHub\"\n" +
			"\n" +
			"Without the file the vault filter is off. Snapshots, marks and bookmarks are\n" +
			"plain files in the directory " + config.EnvDataDir + " names, else\n" +
			"$XDG_DATA_HOME/pulse, else ~/.local/share/pulse.",

		Args: cobra.NoArgs,

		SilenceErrors: false,
		SilenceUsage:  true,

		RunE: func(cmd *cobra.Command, _ []string) error {
			// Nothing is read, written or bound for an address that is not
			// loopback.
			for _, address := range listen {
				if err := config.CheckLoopback(address); err != nil {
					return fmt.Errorf("--listen %w", err)
				}
			}

			home, _ := os.UserHomeDir()
			path, err := config.Path(configPath, os.Getenv, home)
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}
			file, err := config.Load(path, home)
			if err != nil {
				return err
			}
			dir, err := config.DataDir(dataDir, os.Getenv, home)
			if err != nil {
				return fmt.Errorf("data directory: %w", err)
			}
			state, err := store.Open(dir)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			// Only a default address may be missing on the system.
			optional := !cmd.Flags().Changed("listen")
			listeners, err := web.Listen(ctx, listen, optional, func(address string, err error) {
				cmd.PrintErrf("pulse: skipping %s: %v\n", address, err)
			})
			if err != nil {
				return err
			}
			service := watch.New(watch.Options{Store: state, Loader: newLoader(), Vault: file.Vault.Bookmarks})
			secret, variable := token(os.Getenv)
			handler, err := web.NewHandler(web.Options{Service: service, Context: ctx, Details: newDetails(secret)})
			if err != nil {
				for _, listener := range listeners {
					_ = listener.Close()
				}
				return err
			}

			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "pulse listens on %s\n", web.URLFor(listeners[0].Addr()))
			for _, listener := range listeners[1:] {
				if other := web.URLFor(listener.Addr()); other != web.URLFor(listeners[0].Addr()) {
					_, _ = fmt.Fprintf(out, "pulse listens on %s\n", other)
				}
			}
			_, _ = fmt.Fprintf(out, "data: %s\n", dir)
			switch {
			case file.Vault.Bookmarks != "":
				_, _ = fmt.Fprintf(out, "vault: %s (config %s)\n", file.Vault.Bookmarks, file.Path)
			case file.Found:
				_, _ = fmt.Fprintf(out, "vault: not configured (config %s names no bookmarks directory)\n", file.Path)
			default:
				_, _ = fmt.Fprintf(out, "vault: not configured (no config at %s)\n", file.Path)
			}
			if variable != "" {
				_, _ = fmt.Fprintf(out, "github: the reader asks api.github.com with the token of %s\n", variable)
			} else {
				_, _ = fmt.Fprintln(out, "github: the reader asks api.github.com without a token, about 30 repositories an hour")
			}

			return web.Serve(ctx, listeners, handler, grace)
		},
	}
	flags := command.Flags()
	flags.StringArrayVar(&listen, "listen", config.Listen(),
		"loopback address to listen on, repeatable; replaces the defaults")
	flags.StringVar(&configPath, "config", "",
		"configuration file (default: $"+config.EnvConfig+", else $XDG_CONFIG_HOME/pulse/config.toml)")
	flags.StringVar(&dataDir, "data-dir", "",
		"data directory (default: $"+config.EnvDataDir+", else $XDG_DATA_HOME/pulse)")
	return &command
}
