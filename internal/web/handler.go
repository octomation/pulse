package web

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.octolab.org/toolset/pulse/internal/config"
	"go.octolab.org/toolset/pulse/internal/details"
	"go.octolab.org/toolset/pulse/internal/store"
	"go.octolab.org/toolset/pulse/internal/trend"
	"go.octolab.org/toolset/pulse/internal/watch"
)

//go:embed templates/page.html templates/reader.html templates/flow.html templates/missing.html templates/queue.html
//go:embed assets/app.css assets/app.js assets/flow.js assets/favicon.svg
var files embed.FS

// MaxBodyBytes limits the body of a request.
const MaxBodyBytes = 1 << 20

// MaxRepos limits the repositories of one "Mark all shown as seen".
const MaxRepos = 2000

// The formats of the bookmarks export.
const (
	// FormatURLs is a URL per line, as copied from a Safari tab group.
	FormatURLs = "urls"
	// FormatJSONL is a JSON object per line, with the comment.
	FormatJSONL = "jsonl"
)

// Options wire the handler.
type Options struct {
	// Service is trend-watch.
	Service *watch.Service
	// Context bounds the loads the handler starts: they outlive the request
	// that started them, so a closed tab does not fail a list, and end with
	// the service. Nil is context.Background.
	Context context.Context
	// Details asks GitHub about a repository for the reader; nil leaves the
	// reader with what the lists know.
	Details Details
	// Location is the time zone of the times on the page and of the days of
	// the calendar; nil is local.
	Location *time.Location
	// Home is the home directory, which the page writes as ~; empty writes
	// paths in full.
	Home string
}

// Details is what the reader asks GitHub through.
type Details interface {
	Details(ctx context.Context, repo trend.Repo) (details.Details, error)
}

type handler struct {
	Options
	mux  *http.ServeMux
	page *template.Template
}

// NewHandler returns the handler of the service: the page, the assets and
// the API, behind the Host check and the rules for requests that change
// something.
func NewHandler(o Options) (http.Handler, error) {
	h := &handler{Options: o, mux: http.NewServeMux()}
	if h.Context == nil {
		h.Context = context.Background()
	}
	if h.Location == nil {
		h.Location = time.Local
	}
	page, err := template.New("page.html").Funcs(h.funcs()).ParseFS(files, "templates/*.html")
	if err != nil {
		return nil, err
	}
	h.page = page

	h.mux.HandleFunc("GET /{$}", h.index(modePage))
	h.mux.HandleFunc("GET /hidden/{$}", h.index(modeHidden))
	h.mux.HandleFunc("GET /flow/{$}", h.index(modeFlow))
	h.mux.HandleFunc("GET /days/{date}/{$}", h.index(modePage))
	h.mux.HandleFunc("GET /days/{date}/hidden/{$}", h.index(modeHidden))
	h.mux.HandleFunc("GET /days/{date}/flow/{$}", h.index(modeFlow))
	h.mux.HandleFunc("GET /queue/{$}", h.queue)
	h.mux.HandleFunc("GET /assets/{name}", h.asset)
	h.mux.HandleFunc("GET /repos/{owner}/{name}", h.story)
	h.mux.HandleFunc("GET /days/{date}/repos/{owner}/{name}", h.story)
	h.mux.HandleFunc("GET /repos/{owner}/{name}/readme", h.readme)
	h.mux.HandleFunc("GET /api/bookmarks", h.bookmarks)
	h.mux.HandleFunc("POST /api/marks", h.mark)
	h.mux.HandleFunc("POST /api/seen", h.seen)
	h.mux.HandleFunc("POST /api/refresh", h.refresh)
	return h, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; script-src 'self'; "+
		"connect-src 'self'; img-src 'self'; frame-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("Cache-Control", "no-store")

	if !HostAllowed(r.Host) {
		writeError(w, http.StatusMisdirectedRequest, "the Host of the request is not this service")
		return
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
	case http.MethodPost:
		// A page of another site cannot send JSON here without asking
		// first, and the service never says yes.
		if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(origin, r.Host) {
			writeError(w, http.StatusForbidden, "the Origin of the request is not this service")
			return
		}
		if media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || media != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, "Content-Type: application/json is required")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	default:
		header.Set("Allow", "GET, HEAD, POST")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	h.mux.ServeHTTP(w, r)
}

// HostAllowed reports whether a Host header names this service: the name of
// the service, localhost or a literal loopback address, with any port. A
// name that only resolves to loopback is another site.
func HostAllowed(host string) bool {
	name := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		name = h
	}
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	if name == config.Host || name == "localhost" {
		return true
	}
	if strings.HasPrefix(name, "[") && strings.HasSuffix(name, "]") {
		name = name[1 : len(name)-1]
	}
	ip := net.ParseIP(name)
	return ip != nil && ip.IsLoopback()
}

// sameOrigin reports whether an Origin header is the origin the request was
// sent to, whose Host is already checked.
func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	return err == nil && u.Scheme == "http" && strings.EqualFold(u.Host, host)
}

// The modes of a page.
const (
	modePage   = ""
	modeHidden = "hidden/"
	modeFlow   = "flow/"
)

// place is the day a page shows: today, or a past day with snapshots.
type place struct {
	// Day is the date as the path writes it; empty for today, also when the
	// path names today's date.
	Day string
	// Base is the path of the day: / or /days/YYYY-MM-DD/.
	Base string
	// From and To bound a past day.
	From, To time.Time
}

// today reports whether the place is today.
func (p place) today() bool { return p.Day == "" }

// scope is the day the marks of the place are made on.
func (p place) scope() watch.Scope { return watch.Scope{From: p.From, To: p.To} }

// placeOfPage reads the page an action comes from: its date and whether it
// was today's page, which shows the latest snapshot of each list. A today's
// page left open past midnight keeps what it showed: the latest snapshots
// before the end of its date, whatever day they were taken.
func (h *handler) placeOfPage(date string, latest bool) (place, bool) {
	if !latest || date == "" {
		return h.placeOf(date)
	}
	parsed, err := time.Parse(dateLayout, date)
	if err != nil || parsed.Format(dateLayout) != date {
		return place{}, false
	}
	today := h.Service.Now().In(h.Location).Format(dateLayout)
	switch {
	case date == today:
		return place{Base: "/"}, true
	case date > today:
		return place{}, false
	}
	day := watch.DayStart(parsed.Year(), parsed.Month(), parsed.Day(), h.Location)
	return place{Base: "/days/" + date + "/", Day: date, To: watch.NextDay(day, h.Location)}, true
}

// placeOf reads the date of a path: empty is today, and so is today's date;
// any other date has to be a day with snapshots. The days are calendar dates
// in the zone of the handler.
func (h *handler) placeOf(date string) (place, bool) {
	if date == "" {
		return place{Base: "/"}, true
	}
	parsed, err := time.Parse(dateLayout, date)
	if err != nil || parsed.Format(dateLayout) != date {
		return place{}, false
	}
	day := watch.DayStart(parsed.Year(), parsed.Month(), parsed.Day(), h.Location)
	p := place{Base: "/days/" + date + "/"}
	if date == h.Service.Now().In(h.Location).Format(dateLayout) {
		return p, true
	}
	known := func(d time.Time) bool { return d.In(h.Location).Format(dateLayout) == date }
	if !slices.ContainsFunc(h.Service.Days(h.Location), known) {
		return place{}, false
	}
	p.Day, p.From, p.To = date, day, watch.NextDay(day, h.Location)
	return p, true
}

// viewOf builds the view of a place. Today's page loads the lists that are
// due when load is set; a past day loads nothing.
func (h *handler) viewOf(ctx context.Context, p place, load bool) (watch.View, error) {
	switch {
	case !p.today():
		return h.Service.Day(ctx, p.From, p.To)
	case load:
		return h.Service.Open(h.Context)
	default:
		return h.Service.View(ctx), nil
	}
}

// index serves a page of a day in a mode: the paper, the paper with the
// hidden cards, or the flow. Today's page loads the lists that are due; no
// page marks anything, and none takes a query parameter.
func (h *handler) index(mode string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		at, found := h.placeOf(r.PathValue("date"))
		if !found {
			h.missing(w, r)
			return
		}
		view, err := h.viewOf(r.Context(), at, true)
		if err != nil {
			if !at.today() {
				http.Error(w, "the snapshots of the day could not be read: "+err.Error(), http.StatusInternalServerError)
				return
			}
			http.Error(w, "pulse is stopping: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		p := newPage(view, mode == modeHidden)
		// The page names its date, today's too, so that its marks stay on
		// the day it showed when they are made after midnight.
		p.Day, p.Base = p.Date.In(h.Location).Format(dateLayout), at.Base
		today := watch.StartOfDay(view.Now, h.Location)
		current := today
		if view.Past() {
			current = view.Day
		}
		days := h.Service.Days(h.Location)
		p.Calendar = calendar(days, current, today, h.Location)
		prev, next := around(days, current)
		if !prev.IsZero() {
			p.Prev = dayPath(prev, h.Location) + mode
		}
		if !next.IsZero() {
			p.Next = dayPath(next, h.Location) + mode
		}
		name := "page.html"
		if mode == modeFlow {
			name = "flow"
		}
		var buffer bytes.Buffer
		if err := h.page.ExecuteTemplate(&buffer, name, p); err != nil {
			http.Error(w, "the page could not be rendered: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(buffer.Bytes())
	}
}

// queue serves the queue: the repositories marked as interesting that no
// note of the vault names yet, oldest first, also those that left the
// lists. It loads nothing and marks nothing.
func (h *handler) queue(w http.ResponseWriter, r *http.Request) {
	var q queuePage
	bookmarks, err := h.Service.Pending(r.Context())
	if err != nil {
		q.Problem = err.Error() + ", so the queue holds every bookmark, also those with a note"
		bookmarks = h.Service.Bookmarks()
	}
	// The lists know the description of a repository they have now.
	view := h.Service.View(r.Context())
	repos := make([]trend.Repo, len(bookmarks))
	for i, bookmark := range bookmarks {
		repos[i] = bookmark.Repo
	}
	for _, c := range h.Service.LookupAll(r.Context(), repos) {
		if known, found := view.Card(c.Key()); found {
			c.Description, c.Language, c.Lists = known.Description, known.Language, known.Lists
		}
		q.Cards = append(q.Cards, newCard(c))
	}
	var buffer bytes.Buffer
	if err := h.page.ExecuteTemplate(&buffer, "queue", q); err != nil {
		http.Error(w, "the queue could not be rendered: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buffer.Bytes())
}

// missing answers a path of a day that has no page: a malformed date, or a
// day without snapshots. It links to today.
func (h *handler) missing(w http.ResponseWriter, r *http.Request) {
	var buffer bytes.Buffer
	if err := h.page.ExecuteTemplate(&buffer, "missing", r.PathValue("date")); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(buffer.Bytes())
}

func (h *handler) asset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	types := map[string]string{
		"app.css":     "text/css; charset=utf-8",
		"app.js":      "text/javascript; charset=utf-8",
		"flow.js":     "text/javascript; charset=utf-8",
		"favicon.svg": "image/svg+xml",
	}
	raw, err := files.ReadFile("assets/" + name)
	if types[name] == "" || err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", types[name])
	_, _ = w.Write(raw)
}

// story serves a repository for the reader: its card, and what GitHub
// knows. It marks nothing, and a failure of GitHub is a line of the story,
// not of the answer.
func (h *handler) story(w http.ResponseWriter, r *http.Request) {
	at, found := h.placeOf(r.PathValue("date"))
	if !found {
		h.missing(w, r)
		return
	}
	repo, err := parseRepo(r.PathValue("owner") + "/" + r.PathValue("name"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	view, err := h.viewOf(r.Context(), at, false)
	if err != nil {
		http.Error(w, "the snapshots of the day could not be read: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var s story
	if c, found := view.Card(repo.Key()); found {
		// GitHub is asked for the name as the lists write it.
		s.card, s.Known, repo = newCard(c), true, c.Repo
	} else {
		// A repository that left the lists, as the queue links it, still
		// has its answer and its notes.
		s.card = newCard(h.Service.Lookup(r.Context(), repo))
	}
	switch found, err := h.details(r.Context(), repo); {
	case err != nil:
		s.Error = err.Error()
	case found != nil:
		s.Details = found
	}
	var buffer bytes.Buffer
	if err := h.page.ExecuteTemplate(&buffer, "story", s); err != nil {
		http.Error(w, "the story could not be rendered: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buffer.Bytes())
}

// readme serves the README of a repository as a page for the frame of the
// reader. The page may run nothing and load only pictures.
func (h *handler) readme(w http.ResponseWriter, r *http.Request) {
	repo, err := parseRepo(r.PathValue("owner") + "/" + r.PathValue("name"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var data struct {
		Repo   trend.Repo
		Readme template.HTML
		Error  string
	}
	data.Repo = repo
	switch found, err := h.details(r.Context(), repo); {
	case err != nil:
		data.Error = err.Error()
	case found != nil:
		// Cleaned by details.Clean; the policy below holds whatever is left.
		data.Readme = template.HTML(found.Readme) //nolint:gosec
	}
	var buffer bytes.Buffer
	if err := h.page.ExecuteTemplate(&buffer, "readme", data); err != nil {
		http.Error(w, "the README could not be rendered: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src https: data:; media-src https:; "+
		"style-src 'unsafe-inline'; sandbox allow-popups allow-popups-to-escape-sandbox; "+
		"base-uri 'none'; form-action 'none'; frame-ancestors 'self'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buffer.Bytes())
}

// details asks GitHub; nil without an error means that nobody is there to
// ask.
func (h *handler) details(ctx context.Context, repo trend.Repo) (*details.Details, error) {
	if h.Details == nil {
		return nil, nil
	}
	found, err := h.Details.Details(ctx, repo)
	if err != nil {
		return nil, err
	}
	return &found, nil
}

// repoName is owner/name as GitHub allows it: nothing that could break out
// of a line of the export or of a URL.
var repoName = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

func parseRepo(name string) (trend.Repo, error) {
	if !repoName.MatchString(name) {
		return trend.Repo{}, fmt.Errorf("repository %q: want owner/name", name)
	}
	return trend.ParseRepo(name)
}

// mark serves "Interesting" and "Skip". The answer says so only after the
// mark is on disk.
func (h *handler) mark(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Action  store.Action `json:"action"`
		Repo    string       `json:"repo"`
		Comment string       `json:"comment"`
		// Day is the date of the page, YYYY-MM-DD; empty is today.
		Day string `json:"day"`
		// Latest says that the page was today's: it showed the latest
		// snapshot of each list as of its date.
		Latest bool `json:"latest"`
	}
	if !readJSON(w, r, &request) {
		return
	}
	repo, err := parseRepo(request.Repo)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	at, found := h.placeOfPage(request.Day, request.Latest)
	if !found {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("day %q: want the date of a day with snapshots", request.Day))
		return
	}
	comment := strings.Join(strings.Fields(request.Comment), " ")
	if err := h.Service.Mark(request.Action, repo, comment, at.scope()); err != nil {
		if errors.Is(err, watch.ErrInvalid) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "the mark could not be written: "+err.Error())
		return
	}

	view, err := h.viewOf(r.Context(), at, false)
	if err != nil {
		// The mark is on disk: the answer says so, and the page loads again.
		writeJSON(w, http.StatusOK, counts{Problem: "the mark is written, but the day could not be read: " + err.Error(), Reload: true})
		return
	}
	writeJSON(w, http.StatusOK, h.countsOf(view))
}

// seen serves "Mark all shown as seen" for the repositories the page names,
// and its Undo, which takes back the marks it wrote for them.
func (h *handler) seen(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Repos []string `json:"repos"`
		// Undo takes back the marks of "Mark all shown as seen" for the
		// repositories, those whose answer is still that mark; Batch is what
		// it answered.
		Undo  bool `json:"undo"`
		Batch *int `json:"batch"`
		// Day and Latest name the page, as for a mark.
		Day    string `json:"day"`
		Latest bool   `json:"latest"`
	}
	if !readJSON(w, r, &request) {
		return
	}
	at, found := h.placeOfPage(request.Day, request.Latest)
	if !found {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("day %q: want the date of a day with snapshots", request.Day))
		return
	}
	if len(request.Repos) > MaxRepos {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("more than %d repositories", MaxRepos))
		return
	}
	repos := make([]trend.Repo, 0, len(request.Repos))
	for _, name := range request.Repos {
		repo, err := parseRepo(name)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		repos = append(repos, repo)
	}
	var marked int
	var batch *int
	names := []string{}
	if request.Undo {
		if request.Batch == nil {
			writeError(w, http.StatusBadRequest, "undo needs the batch that Mark all shown as seen answered")
			return
		}
		undone, err := h.Service.UndoSeen(watch.Batch{Repos: repos, From: *request.Batch}, at.scope())
		if errors.Is(err, watch.ErrInvalid) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "the marks could not be taken back: "+err.Error())
			return
		}
		marked = undone
	} else {
		done, err := h.Service.MarkSeen(repos, at.scope())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "the marks could not be written: "+err.Error())
			return
		}
		marked, batch = len(done.Repos), &done.From
		for _, repo := range done.Repos {
			names = append(names, repo.String())
		}
	}
	view, err := h.viewOf(r.Context(), at, false)
	if err != nil {
		writeJSON(w, http.StatusOK, counts{Marked: &marked, Repos: names, Batch: batch, Problem: "the marks are written, but the day could not be read: " + err.Error(), Reload: true})
		return
	}
	answer := h.countsOf(view)
	answer.Marked, answer.Repos, answer.Batch = &marked, names, batch
	writeJSON(w, http.StatusOK, answer)
}

// refresh loads all six lists now; the page shows the outcome of each.
func (h *handler) refresh(w http.ResponseWriter, r *http.Request) {
	// The body carries nothing; it is read so that the connection can be
	// used again.
	_, _ = io.Copy(io.Discard, r.Body)
	problem, err := h.Service.Refresh(h.Context)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "pulse is stopping: "+err.Error())
		return
	}
	// The lists are loaded even when their outcome could not be recorded:
	// the page that follows shows them, and the problem with them.
	view := h.Service.View(r.Context())
	answer := h.countsOf(view)
	failed, loaded := view.FailedLists(), view.LoadedLists()
	answer.Failed, answer.Loaded, answer.Problem = &failed, &loaded, problem
	writeJSON(w, http.StatusOK, answer)
}

// bookmarks serves the export: by default the bookmarks that no note of the
// vault names yet, with all=true every bookmark. It reads the marks and
// writes nothing.
func (h *handler) bookmarks(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	format := query.Get("format")
	if format == "" {
		format = FormatURLs
	}
	var all bool
	switch query.Get("all") {
	case "", "false":
	case "true":
		all = true
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("all %q: want true or false", query.Get("all")))
		return
	}
	if format != FormatURLs && format != FormatJSONL {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("format %q: want %s or %s", format, FormatURLs, FormatJSONL))
		return
	}
	list := h.Service.Bookmarks()
	if !all {
		pending, err := h.Service.Pending(r.Context())
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error()+"; the bookmarks that have a note cannot be told apart: pulsectl bookmarks --all prints every bookmark")
			return
		}
		list = pending
	}
	var buffer bytes.Buffer
	switch format {
	case FormatURLs:
		for _, bookmark := range list {
			buffer.WriteString(bookmark.Repo.URL())
			buffer.WriteByte('\n')
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	case FormatJSONL:
		encoder := json.NewEncoder(&buffer)
		encoder.SetEscapeHTML(false)
		for _, bookmark := range list {
			lists := bookmark.Lists
			if lists == nil {
				lists = []string{}
			}
			err := encoder.Encode(Bookmark{
				URL: bookmark.Repo.URL(), Repo: bookmark.Repo.String(), Comment: bookmark.Comment,
				BookmarkedAt: bookmark.At.UTC().Format(time.RFC3339), Lists: lists,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("format %q: want %s or %s", format, FormatURLs, FormatJSONL))
		return
	}
	_, _ = w.Write(buffer.Bytes())
}

// Bookmark is a line of the JSONL export.
type Bookmark struct {
	URL          string   `json:"url"`
	Repo         string   `json:"repo"`
	Comment      string   `json:"comment"`
	BookmarkedAt string   `json:"bookmarked_at"`
	Lists        []string `json:"lists"`
}

// counts is the answer to an action: what the header of the page shows now.
type counts struct {
	New    int `json:"new"`
	Hidden int `json:"hidden"`
	// Empty is what the page says in place of the cards when none is new.
	Empty string `json:"empty,omitempty"`
	// Marked is the number of repositories "Mark all shown as seen" marked,
	// or its Undo took back.
	Marked *int `json:"marked,omitempty"`
	// Repos names the repositories "Mark all shown as seen" marked, for its
	// Undo.
	Repos []string `json:"repos,omitempty"`
	// Batch tells the marks of this "Mark all shown as seen" from later ones,
	// for its Undo.
	Batch *int `json:"batch,omitempty"`
	// Failed and Loaded count the lists after a refresh.
	Failed *int `json:"failed,omitempty"`
	Loaded *int `json:"loaded,omitempty"`
	// Problem says what a refresh could not record, as the page does, or
	// what could not be read after a mark was written.
	Problem string `json:"problem,omitempty"`
	// Reload says that the page has to load again to show the outcome.
	Reload bool `json:"reload,omitempty"`
}

func (h *handler) countsOf(view watch.View) counts {
	shown := len(view.Shown())
	answer := counts{New: shown, Hidden: len(view.Cards) - shown}
	if shown == 0 {
		answer.Empty = h.fragment("empty", emptyOf(view))
	}
	return answer
}

// fragment renders a part of the page for the script to put in place.
func (h *handler) fragment(name string, data any) string {
	var buffer bytes.Buffer
	if err := h.page.ExecuteTemplate(&buffer, name, data); err != nil {
		return ""
	}
	return strings.TrimSpace(buffer.String())
}

func readJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		writeError(w, http.StatusBadRequest, "the request is not the JSON this service takes: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
