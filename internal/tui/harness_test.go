package tui_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jleitaopv/claudebar/internal/tui"
	"github.com/jleitaopv/claudebar/internal/usage"
)

// startTime is the fake clock's initial reading. Times are in UTC so the
// absolute Reset Times rendered by the app don't depend on the machine's zone.
var startTime = time.Date(2026, 10, 3, 11, 15, 0, 0, time.UTC)

// fakeClock is a manually advanced clock. Its Tick never fires on its own;
// tests deliver tui.TickMsg explicitly after calling Advance.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) Tick() tea.Cmd { return nil }

func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

// fakeUsageEndpoint stands in for the OAuth usage endpoint. Each request is
// answered by respond, which tests can swap between polls.
type fakeUsageEndpoint struct {
	server   *httptest.Server
	respond  func(w http.ResponseWriter, r *http.Request)
	requests int
}

func newFakeUsageEndpoint(t *testing.T, body string) *fakeUsageEndpoint {
	t.Helper()
	e := &fakeUsageEndpoint{}
	e.respondWith(http.StatusOK, body)
	e.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.requests++
		e.respond(w, r)
	}))
	t.Cleanup(e.server.Close)
	return e
}

func (e *fakeUsageEndpoint) respondWith(status int, body string) {
	e.respond = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}
}

// usageBody builds an endpoint response in the shape captured from a real
// Pro account (see issue #1), with the given Session and Weekly windows.
// Pass "null" for a window object or resets_at to mimic an unstarted window.
func usageBody(sessionUtil float64, sessionReset string, weeklyUtil float64, weeklyReset string) string {
	return fmt.Sprintf(`{
  "five_hour": {"utilization": %v, "resets_at": %s, "limit_dollars": null},
  "seven_day": {"utilization": %v, "resets_at": %s, "limit_dollars": null},
  "seven_day_opus": null,
  "seven_day_sonnet": null,
  "extra_usage": {"is_enabled": false, "monthly_limit": 500},
  "limits": [{"kind": "session", "percent": 1, "severity": "normal"}]
}`, sessionUtil, quoteUnlessNull(sessionReset), weeklyUtil, quoteUnlessNull(weeklyReset))
}

func quoteUnlessNull(s string) string {
	if s == "null" {
		return s
	}
	return `"` + s + `"`
}

// writeCredentials writes a Claude Code credentials file whose token expires
// at expiresAt, and returns its path.
func writeCredentials(t *testing.T, plan string, expiresAt time.Time) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".credentials.json")
	writeCredentialsAt(t, path, "test-token", plan, expiresAt)
	return path
}

func writeCredentialsAt(t *testing.T, path, token, plan string, expiresAt time.Time) {
	t.Helper()
	content := fmt.Sprintf(`{"claudeAiOauth": {
  "accessToken": %q,
  "refreshToken": "test-refresh",
  "expiresAt": %d,
  "scopes": ["user:inference"],
  "subscriptionType": %q
}}`, token, expiresAt.UnixMilli(), plan)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing credentials file %s: %v", path, err)
	}
}

// app is a running claudebar under test: the model plus its fakes.
type app struct {
	t               *testing.T
	model           tea.Model
	clock           *fakeClock
	endpoint        *fakeUsageEndpoint
	credentialsPath string
	quit            bool
}

// startApp builds claudebar against a fake endpoint serving body, runs Init
// (which performs the startup fetch), and returns it.
func startApp(t *testing.T, body string) *app {
	t.Helper()
	a := newApp(t, body)
	a.run(a.model.Init())
	return a
}

func newApp(t *testing.T, body string) *app {
	t.Helper()
	clock := &fakeClock{now: startTime}
	endpoint := newFakeUsageEndpoint(t, body)
	credentialsPath := writeCredentials(t, "pro", startTime.Add(8*time.Hour))
	model, err := tui.New(tui.Deps{
		Usage: usage.NewClient(usage.ClientConfig{
			BaseURL:         endpoint.server.URL,
			CredentialsPath: credentialsPath,
			HTTPClient:      endpoint.server.Client(),
			Now:             clock.Now,
		}),
		CredentialsPath: credentialsPath,
		Clock:           clock,
	})
	if err != nil {
		t.Fatalf("tui.New: %v", err)
	}
	return &app{t: t, model: model, clock: clock, endpoint: endpoint, credentialsPath: credentialsPath}
}

// run executes cmd the way the Bubble Tea runtime would, synchronously,
// feeding every resulting message back into the model.
func (a *app) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			a.run(c)
		}
	case tea.QuitMsg:
		a.quit = true
	default:
		a.send(msg)
	}
}

func (a *app) send(msg tea.Msg) {
	var cmd tea.Cmd
	a.model, cmd = a.model.Update(msg)
	a.run(cmd)
}

func (a *app) press(key rune) {
	a.send(tea.KeyPressMsg{Code: key, Text: string(key)})
}

// wait advances the clock by d and delivers the one-second tick that the real
// clock would have sent at that moment.
func (a *app) wait(d time.Duration) {
	a.clock.Advance(d)
	a.send(tui.TickMsg(a.clock.Now()))
}

func (a *app) resize(width int) {
	a.send(tea.WindowSizeMsg{Width: width, Height: 12})
}

// screen is the rendered view as styled text (including ANSI colour codes).
func (a *app) screen() string {
	return a.model.View().Content
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

// text is the rendered view with colours stripped.
func (a *app) text() string {
	return ansiEscape.ReplaceAllString(a.screen(), "")
}

// line returns the first rendered line containing substr, colours stripped.
func (a *app) line(substr string) string {
	a.t.Helper()
	for l := range strings.SplitSeq(a.text(), "\n") {
		if strings.Contains(l, substr) {
			return l
		}
	}
	a.t.Fatalf("no line containing %q on screen:\n%s", substr, a.text())
	return ""
}

func (a *app) styledLine(substr string) string {
	a.t.Helper()
	for l := range strings.SplitSeq(a.screen(), "\n") {
		if strings.Contains(ansiEscape.ReplaceAllString(l, ""), substr) {
			return l
		}
	}
	a.t.Fatalf("no line containing %q on screen:\n%s", substr, a.text())
	return ""
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected to find %q in:\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("expected not to find %q in:\n%s", needle, haystack)
	}
}

// pressDeferred delivers a key press but holds back the resulting command,
// standing in for a fetch that is still in flight. Run it later with run.
func (a *app) pressDeferred(key rune) tea.Cmd {
	var cmd tea.Cmd
	a.model, cmd = a.model.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
	return cmd
}
