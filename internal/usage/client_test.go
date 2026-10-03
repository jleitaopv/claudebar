package usage_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jleitaopv/claudebar/internal/usage"
)

var now = time.Date(2026, 10, 3, 11, 15, 0, 0, time.UTC)

// fetchFrom runs one Fetch against a fake endpoint that answers every
// request with status and body.
func fetchFrom(t *testing.T, status int, body string) (usage.Reading, error) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)
	return newClient(t, server).Fetch(context.Background())
}

func newClient(t *testing.T, server *httptest.Server) *usage.Client {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".credentials.json")
	creds := fmt.Sprintf(`{"claudeAiOauth": {"accessToken": "test-token", "expiresAt": %d, "subscriptionType": "pro"}}`,
		now.Add(time.Hour).UnixMilli())
	if err := os.WriteFile(path, []byte(creds), 0o600); err != nil {
		t.Fatalf("writing credentials file %s: %v", path, err)
	}
	return usage.NewClient(usage.ClientConfig{
		BaseURL:         server.URL,
		CredentialsPath: path,
		HTTPClient:      server.Client(),
		Now:             func() time.Time { return now },
	})
}

func TestFetchReadsSessionAndWeeklyWindows(t *testing.T) {
	reading, err := fetchFrom(t, 200, `{
  "five_hour": {"utilization": 42.5, "resets_at": "2026-10-03T13:29:59.986824+00:00"},
  "seven_day": {"utilization": 104, "resets_at": "2026-10-04T21:59:59Z"}
}`)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	want := usage.Reading{
		SessionWindow: usage.UsageWindow{Utilization: 42.5, ResetTime: time.Date(2026, 10, 3, 13, 29, 59, 986824000, time.UTC)},
		WeeklyWindow:  usage.UsageWindow{Utilization: 104, ResetTime: time.Date(2026, 10, 4, 21, 59, 59, 0, time.UTC)},
		FetchedAt:     now,
	}
	if !reading.SessionWindow.ResetTime.Equal(want.SessionWindow.ResetTime) || reading.SessionWindow.Utilization != want.SessionWindow.Utilization ||
		!reading.WeeklyWindow.ResetTime.Equal(want.WeeklyWindow.ResetTime) || reading.WeeklyWindow.Utilization != want.WeeklyWindow.Utilization ||
		!reading.FetchedAt.Equal(want.FetchedAt) {
		t.Errorf("got %+v, want %+v", reading, want)
	}
}

func TestFetchTreatsNullAsAWindowThatHasNotStarted(t *testing.T) {
	cases := map[string]string{
		"null resets_at":     `{"five_hour": {"utilization": 0, "resets_at": null}, "seven_day": {"utilization": 3, "resets_at": "2026-10-04T21:59:59Z"}}`,
		"null window object": `{"five_hour": null, "seven_day": {"utilization": 3, "resets_at": "2026-10-04T21:59:59Z"}}`,
	}
	for name, body := range cases {
		reading, err := fetchFrom(t, 200, body)
		if err != nil {
			t.Errorf("%s: Fetch: %v", name, err)
			continue
		}
		if reading.SessionWindow.Started() || reading.SessionWindow.Utilization != 0 {
			t.Errorf("%s: Session = %+v, want a not-started window at 0%%", name, reading.SessionWindow)
		}
		if !reading.WeeklyWindow.Started() {
			t.Errorf("%s: Weekly = %+v, want a started window", name, reading.WeeklyWindow)
		}
	}
}

func TestFetchIgnoresFieldsItDoesNotUse(t *testing.T) {
	_, err := fetchFrom(t, 200, `{
  "five_hour": {"utilization": 1, "resets_at": "2026-10-03T13:29:59Z", "limit_dollars": null, "locked_reason": null},
  "seven_day": {"utilization": 1, "resets_at": "2026-10-04T21:59:59Z"},
  "seven_day_opus": {"utilization": 9, "resets_at": "2026-10-04T21:59:59Z"},
  "iguana_necktie": null,
  "brand_new_field": [1, 2, 3],
  "limits": [{"kind": "session", "severity": "normal"}]
}`)
	if err != nil {
		t.Errorf("Fetch with extra fields: %v", err)
	}
}

func TestFetchRejectsResponsesInAnUnexpectedShape(t *testing.T) {
	cases := map[string]string{
		"not JSON":              `<html>maintenance</html>`,
		"JSON array":            `[]`,
		"session window absent": `{"seven_day": {"utilization": 1, "resets_at": null}}`,
		"weekly window absent":  `{"five_hour": {"utilization": 1, "resets_at": null}}`,
		"utilization missing":   `{"five_hour": {"resets_at": null}, "seven_day": {"utilization": 1, "resets_at": null}}`,
		"utilization a string":  `{"five_hour": {"utilization": "42%", "resets_at": null}, "seven_day": {"utilization": 1, "resets_at": null}}`,
		"resets_at not a time":  `{"five_hour": {"utilization": 1, "resets_at": "soon"}, "seven_day": {"utilization": 1, "resets_at": null}}`,
		"window is a number":    `{"five_hour": 5, "seven_day": {"utilization": 1, "resets_at": null}}`,
	}
	for name, body := range cases {
		_, err := fetchFrom(t, 200, body)

		if !errors.Is(err, usage.ErrUnexpectedResponse) {
			t.Errorf("%s: got error %v, want usage.ErrUnexpectedResponse", name, err)
		}
	}
}

func TestFetchReportsNonOKStatusesAsUnexpected(t *testing.T) {
	for _, status := range []int{403, 429, 500, 503} {
		_, err := fetchFrom(t, status, `{"error": "nope"}`)

		if !errors.Is(err, usage.ErrUnexpectedResponse) {
			t.Errorf("HTTP %d: got error %v, want usage.ErrUnexpectedResponse", status, err)
		}
	}
}

func TestFetchSendsTheOAuthTokenAndBetaHeader(t *testing.T) {
	var gotAuth, gotBeta, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotBeta, gotPath = r.Header.Get("Authorization"), r.Header.Get("anthropic-beta"), r.URL.Path
		fmt.Fprint(w, `{"five_hour": null, "seven_day": null}`)
	}))
	t.Cleanup(server.Close)

	if _, err := newClient(t, server).Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-token")
	}
	if gotBeta != "oauth-2025-04-20" {
		t.Errorf("anthropic-beta = %q, want %q", gotBeta, "oauth-2025-04-20")
	}
	if gotPath != "/api/oauth/usage" {
		t.Errorf("path = %q, want %q", gotPath, "/api/oauth/usage")
	}
}
