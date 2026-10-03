package tui_test

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jleitaopv/claudebar/internal/tui"
	"github.com/jleitaopv/claudebar/internal/usage"
)

// ANSI foreground colour codes for the bar thresholds.
const (
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
)

// barCells counts the filled and total cells of the bar on a rendered line.
func barCells(line string) (filled, total int) {
	filled = strings.Count(line, "█")
	return filled, filled + strings.Count(line, "░")
}

func TestShowsSessionAndWeeklyUtilizationAfterStartup(t *testing.T) {
	a := startApp(t, usageBody(48, "2026-10-03T13:29:59Z", 15, "2026-10-04T21:59:59Z"))

	assertContains(t, a.line("Session"), "48%")
	assertContains(t, a.line("Weekly"), "15%")
}

func TestBarsStretchToTerminalWidth(t *testing.T) {
	a := startApp(t, usageBody(50, "2026-10-03T13:29:59Z", 25, "2026-10-04T21:59:59Z"))

	for _, width := range []int{40, 80, 120} {
		a.resize(width)
		for _, label := range []string{"Session", "Weekly"} {
			if got := utf8.RuneCountInString(a.line(label)); got != width {
				t.Errorf("width %d: %s line is %d cells wide, want %d:\n%s", width, label, got, width, a.text())
			}
		}
	}
}

func TestBarFillIsProportionalToUtilization(t *testing.T) {
	a := startApp(t, usageBody(50, "2026-10-03T13:29:59Z", 25, "2026-10-04T21:59:59Z"))
	a.resize(80)

	sessionFilled, sessionTotal := barCells(a.line("Session"))
	if sessionTotal == 0 {
		t.Fatalf("no bar drawn on Session line:\n%s", a.text())
	}
	if sessionFilled*2 != sessionTotal {
		t.Errorf("Session at 50%%: %d of %d bar cells filled, want half", sessionFilled, sessionTotal)
	}
	weeklyFilled, weeklyTotal := barCells(a.line("Weekly"))
	if weeklyFilled*4 != weeklyTotal {
		t.Errorf("Weekly at 25%%: %d of %d bar cells filled, want a quarter", weeklyFilled, weeklyTotal)
	}
}

func TestBarColourReflectsHowCloseTheWindowIsToItsLimit(t *testing.T) {
	cases := []struct {
		utilization float64
		colour      string
		name        string
	}{
		{0, ansiGreen, "green"},
		{49, ansiGreen, "green"},
		{50, ansiYellow, "yellow"},
		{79, ansiYellow, "yellow"},
		{80, ansiRed, "red"},
		{100, ansiRed, "red"},
	}
	for _, tc := range cases {
		a := startApp(t, usageBody(tc.utilization, "2026-10-03T13:29:59Z", 0, "2026-10-04T21:59:59Z"))

		line := a.styledLine("Session")
		for _, other := range []string{ansiGreen, ansiYellow, ansiRed} {
			if other == tc.colour {
				continue
			}
			assertNotContains(t, line, other)
		}
		if !strings.Contains(line, tc.colour) {
			t.Errorf("Session at %v%%: want %s bar, got line %q", tc.utilization, tc.name, line)
		}
	}
}

func TestUtilizationOverTheLimitShowsAFullRedBarAndTheTrueNumber(t *testing.T) {
	a := startApp(t, usageBody(104, "2026-10-03T13:29:59Z", 0, "2026-10-04T21:59:59Z"))

	line := a.line("Session")
	assertContains(t, line, "104%")
	if filled, total := barCells(line); filled != total || total == 0 {
		t.Errorf("Session at 104%%: %d of %d bar cells filled, want all", filled, total)
	}
	assertContains(t, a.styledLine("Session"), ansiRed)
}

func TestShowsCountdownAndLocalTimeUntilEachWindowResets(t *testing.T) {
	// startTime is Sat 2026-10-03 11:15:00 UTC.
	a := startApp(t, usageBody(48, "2026-10-03T13:29:59Z", 15, "2026-10-04T21:59:59Z"))

	assertContains(t, a.text(), "resets in 2h 14m (13:29)")
	assertContains(t, a.text(), "resets in 1d 10h (Sun 21:59)")
}

func TestCountdownUsesTheTwoLargestUnits(t *testing.T) {
	cases := []struct {
		resetsAt string
		want     string
	}{
		{"2026-10-05T12:15:00Z", "resets in 2d 1h"},
		{"2026-10-03T14:15:00Z", "resets in 3h 0m"},
		{"2026-10-03T11:20:30Z", "resets in 5m 30s"},
		{"2026-10-03T11:15:45Z", "resets in 45s"},
	}
	for _, tc := range cases {
		a := startApp(t, usageBody(10, tc.resetsAt, 10, "2026-10-09T11:15:00Z"))

		assertContains(t, a.text(), tc.want)
	}
}

func TestCountdownTicksWithTheClockWithoutRefetching(t *testing.T) {
	a := startApp(t, usageBody(48, "2026-10-03T11:20:30Z", 15, "2026-10-04T21:59:59Z"))
	requestsAfterStartup := a.endpoint.requests

	a.wait(1 * time.Second)
	assertContains(t, a.text(), "resets in 5m 29s")
	a.wait(1 * time.Second)
	assertContains(t, a.text(), "resets in 5m 28s")

	if a.endpoint.requests != requestsAfterStartup {
		t.Errorf("endpoint called %d times during ticks, want 0", a.endpoint.requests-requestsAfterStartup)
	}
}

func TestResetTimeIsShownInTheClocksTimeZone(t *testing.T) {
	a := startApp(t, usageBody(48, "2026-10-03T13:29:59Z", 15, "2026-10-04T21:59:59Z"))
	lisbon := time.FixedZone("WEST", 1*60*60)
	a.clock.now = a.clock.now.In(lisbon)
	a.wait(0)

	assertContains(t, a.text(), "(14:29)")
	assertContains(t, a.text(), "(Sun 22:59)")
}

func TestWindowWithoutResetTimeIsShownAsNotStarted(t *testing.T) {
	a := startApp(t, usageBody(0, "null", 15, "2026-10-04T21:59:59Z"))

	assertContains(t, a.text(), "0% · starts with your next message")
	if got := strings.Count(a.text(), "resets in"); got != 1 {
		t.Errorf("want a countdown only for the Weekly Window, found %d:\n%s", got, a.text())
	}
}

func TestHeaderShowsThePlan(t *testing.T) {
	a := startApp(t, usageBody(48, "2026-10-03T13:29:59Z", 15, "2026-10-04T21:59:59Z"))

	assertContains(t, a.line("claudebar"), "pro")
}

func TestStatusLineShowsHowLongAgoTheReadingWasFetched(t *testing.T) {
	a := startApp(t, usageBody(48, "2026-10-03T13:29:59Z", 15, "2026-10-04T21:59:59Z"))

	assertContains(t, a.text(), "updated 0s ago")
	a.wait(12 * time.Second)
	assertContains(t, a.text(), "updated 12s ago")
	assertContains(t, a.line("updated"), "r refresh  q quit")
}

func TestStartupFailsClearlyWithoutASubscriptionLogin(t *testing.T) {
	dir := t.TempDir()
	apiKeyOnly := filepath.Join(dir, "api-key-only.json")
	if err := os.WriteFile(apiKeyOnly, []byte(`{"primaryApiKey": "sk-test"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"no credentials file":     filepath.Join(dir, "missing.json"),
		"API key, no OAuth login": apiKeyOnly,
	}
	for name, path := range cases {
		_, err := tui.New(tui.Deps{CredentialsPath: path, Clock: &fakeClock{now: startTime}})

		if !errors.Is(err, usage.ErrNoLogin) {
			t.Errorf("%s: got error %v, want usage.ErrNoLogin", name, err)
			continue
		}
		assertContains(t, err.Error(), "Claude subscription login")
	}
}

func TestFetchesANewReadingEverySixtySeconds(t *testing.T) {
	a := startApp(t, usageBody(48, "2026-10-03T13:29:59Z", 15, "2026-10-04T21:59:59Z"))
	a.endpoint.respondWith(200, usageBody(52, "2026-10-03T13:29:59Z", 16, "2026-10-04T21:59:59Z"))

	a.wait(59 * time.Second)
	assertContains(t, a.line("Session"), "48%")

	a.wait(1 * time.Second)
	assertContains(t, a.line("Session"), "52%")
	assertContains(t, a.line("Weekly"), "16%")
	assertContains(t, a.text(), "updated 0s ago")
}

func TestPressingRFetchesImmediately(t *testing.T) {
	a := startApp(t, usageBody(48, "2026-10-03T13:29:59Z", 15, "2026-10-04T21:59:59Z"))
	a.endpoint.respondWith(200, usageBody(70, "2026-10-03T13:29:59Z", 20, "2026-10-04T21:59:59Z"))

	a.wait(5 * time.Second)
	a.press('r')

	assertContains(t, a.line("Session"), "70%")
	assertContains(t, a.text(), "updated 0s ago")
}

func TestPressingRAgainWhileAFetchIsInFlightDoesNotFetchTwice(t *testing.T) {
	a := startApp(t, usageBody(48, "2026-10-03T13:29:59Z", 15, "2026-10-04T21:59:59Z"))
	requestsAfterStartup := a.endpoint.requests

	first := a.pressDeferred('r')
	second := a.pressDeferred('r')
	a.run(first)
	a.run(second)

	if got := a.endpoint.requests - requestsAfterStartup; got != 1 {
		t.Errorf("pressing r twice during one fetch made %d requests, want 1", got)
	}
}

func TestPressingQQuits(t *testing.T) {
	a := startApp(t, usageBody(48, "2026-10-03T13:29:59Z", 15, "2026-10-04T21:59:59Z"))

	a.press('q')

	if !a.quit {
		t.Error("pressing q did not quit")
	}
}

func TestKeepsTheLastReadingMarkedStaleWhenTheEndpointIsUnreachable(t *testing.T) {
	a := startApp(t, usageBody(48, "2026-10-03T13:29:59Z", 15, "2026-10-04T21:59:59Z"))
	a.endpoint.server.Close()

	a.wait(3 * time.Minute)

	assertContains(t, a.line("Session"), "48%")
	assertContains(t, a.line("Weekly"), "15%")
	assertContains(t, a.text(), "stale, fetched 3m 0s ago · can't reach the usage endpoint")
}

func TestExpiredTokenTellsTheUserToRunClaude(t *testing.T) {
	t.Run("expiry time has passed", func(t *testing.T) {
		a := startApp(t, usageBody(48, "2026-10-03T13:29:59Z", 15, "2026-10-04T21:59:59Z"))
		writeCredentialsAt(t, a.credentialsPath, "test-token", "pro", startTime.Add(30*time.Second))
		requestsBefore := a.endpoint.requests

		a.wait(time.Minute)

		assertContains(t, a.text(), "stale, fetched 1m 0s ago · token expired, run `claude`")
		if a.endpoint.requests != requestsBefore {
			t.Errorf("sent %d requests with a token known to be expired, want 0", a.endpoint.requests-requestsBefore)
		}
	})
	t.Run("endpoint rejects the token", func(t *testing.T) {
		a := startApp(t, usageBody(48, "2026-10-03T13:29:59Z", 15, "2026-10-04T21:59:59Z"))
		a.endpoint.respondWith(401, `{"error": {"type": "authentication_error"}}`)

		a.wait(time.Minute)

		assertContains(t, a.text(), "stale, fetched 1m 0s ago · token expired, run `claude`")
	})
}

func TestPicksUpATokenClaudeCodeRefreshedWithoutRestarting(t *testing.T) {
	a := startApp(t, usageBody(48, "2026-10-03T13:29:59Z", 15, "2026-10-04T21:59:59Z"))
	a.endpoint.respond = func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer refreshed-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, usageBody(60, "2026-10-03T13:29:59Z", 18, "2026-10-04T21:59:59Z"))
	}
	a.wait(time.Minute)
	assertContains(t, a.text(), "token expired")

	writeCredentialsAt(t, a.credentialsPath, "refreshed-token", "pro", startTime.Add(9*time.Hour))
	a.wait(time.Minute)

	assertContains(t, a.line("Session"), "60%")
	assertContains(t, a.text(), "updated 0s ago")
}

func TestFailureBeforeAnyReadingShowsTheErrorAndNoUtilization(t *testing.T) {
	a := newApp(t, "")
	a.endpoint.respondWith(500, "internal error")
	a.run(a.model.Init())

	assertNotContains(t, a.text(), "%")
	assertNotContains(t, a.text(), "█")
	assertContains(t, a.text(), "unexpected response from the usage endpoint")
	assertNotContains(t, a.text(), "stale")
}

func TestReshapedResponseKeepsTheStaleReading(t *testing.T) {
	a := startApp(t, usageBody(48, "2026-10-03T13:29:59Z", 15, "2026-10-04T21:59:59Z"))
	a.endpoint.respondWith(200, `{"windows": [{"name": "session", "used": 0.5}]}`)

	a.wait(time.Minute)

	assertContains(t, a.line("Session"), "48%")
	assertContains(t, a.text(), "stale, fetched 1m 0s ago · unexpected response from the usage endpoint")
}

func TestStartupRejectsAMalformedCredentialsFile(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"not JSON":        `not json`,
		"no access token": `{"claudeAiOauth": {"expiresAt": 1791045446393, "subscriptionType": "pro"}}`,
		"no expiry":       `{"claudeAiOauth": {"accessToken": "t", "subscriptionType": "pro"}}`,
	}
	for name, content := range cases {
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}

		_, err := tui.New(tui.Deps{CredentialsPath: path, Clock: &fakeClock{now: startTime}})

		if err == nil || errors.Is(err, usage.ErrNoLogin) {
			t.Errorf("%s: got error %v, want a malformed-credentials error", name, err)
			continue
		}
		assertContains(t, err.Error(), "claudeAiOauth")
	}
}
