// Package tui is claudebar's Bubble Tea front end: it polls for Readings and
// renders the Session and Weekly Windows.
package tui

import (
	"context"
	"errors"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jleitaopv/claudebar/internal/usage"
)

// Clock is the app's source of time. Tick returns a command that delivers
// the next TickMsg one second from now.
type Clock interface {
	Now() time.Time
	Tick() tea.Cmd
}

// TickMsg is the once-a-second heartbeat that advances countdowns and
// schedules polls.
type TickMsg time.Time

// Deps holds the app's dependencies.
type Deps struct {
	Usage           *usage.Client
	CredentialsPath string
	Clock           Clock
}

type fetchResultMsg struct {
	reading usage.Reading
	err     error
}

// pollInterval is how often a new Reading is fetched. The endpoint's rate
// limit is unknown, so this is deliberately conservative (ADR 0001).
const pollInterval = 60 * time.Second

// Model is the claudebar Bubble Tea model.
type Model struct {
	deps    Deps
	plan    string
	reading *usage.Reading
	// fetchErr is why the most recent fetch failed, or nil if it succeeded.
	// When set, reading (if any) is Stale.
	fetchErr error
	width    int
	// fetching is true while a fetch is in flight, so polls never overlap.
	fetching       bool
	lastFetchStart time.Time
}

// New builds the model. It reads the credentials file once up front, for the
// Plan shown in the header and to fail fast: it returns usage.ErrNoLogin when
// there is no Claude subscription login on this machine.
func New(deps Deps) (*Model, error) {
	creds, err := usage.ReadCredentials(deps.CredentialsPath)
	if err != nil {
		return nil, err
	}
	return &Model{deps: deps, plan: creds.Plan, width: defaultWidth}, nil
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.deps.Clock.Tick(), m.startFetch())
}

func (m *Model) startFetch() tea.Cmd {
	m.fetching = true
	m.lastFetchStart = m.deps.Clock.Now()
	return func() tea.Msg {
		reading, err := m.deps.Usage.Fetch(context.Background())
		return fetchResultMsg{reading: reading, err: err}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case fetchResultMsg:
		m.fetching = false
		m.fetchErr = msg.err
		if msg.err == nil {
			m.reading = &msg.reading
		}
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			if !m.fetching {
				return m, m.startFetch()
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case TickMsg:
		if m.fetching || m.deps.Clock.Now().Sub(m.lastFetchStart) < pollInterval {
			return m, m.deps.Clock.Tick()
		}
		return m, tea.Batch(m.deps.Clock.Tick(), m.startFetch())
	}
	return m, nil
}

func (m *Model) View() tea.View {
	now := m.deps.Clock.Now()
	var b strings.Builder
	b.WriteString(spreadLine("claudebar", m.plan, m.width) + "\n")
	b.WriteString(separator(m.width) + "\n")
	if m.reading != nil {
		b.WriteString(renderWindowLine("Session", m.reading.SessionWindow, m.width) + "\n")
		b.WriteString(renderResetLine(m.reading.SessionWindow, now) + "\n\n")
		b.WriteString(renderWindowLine("Weekly", m.reading.WeeklyWindow, m.width) + "\n")
		b.WriteString(renderResetLine(m.reading.WeeklyWindow, now) + "\n")
	}
	b.WriteString(separator(m.width) + "\n")
	b.WriteString(spreadLine(m.status(now), keyHints, m.width))
	view := tea.NewView(b.String())
	view.AltScreen = true
	return view
}

func (m *Model) status(now time.Time) string {
	if m.reading == nil && m.fetchErr != nil {
		return "no Reading yet · " + failureReason(m.fetchErr)
	}
	if m.reading == nil {
		return "fetching…"
	}
	age := formatDuration(now.Sub(m.reading.FetchedAt))
	if m.fetchErr != nil {
		return "stale, fetched " + age + " ago · " + failureReason(m.fetchErr)
	}
	return "updated " + age + " ago"
}

// failureReason is the short, user-facing explanation of a failed fetch.
func failureReason(err error) string {
	switch {
	case errors.Is(err, usage.ErrUnreachable):
		return "can't reach the usage endpoint"
	case errors.Is(err, usage.ErrTokenExpired):
		return "token expired, run `claude`"
	case errors.Is(err, usage.ErrUnexpectedResponse):
		return "unexpected response from the usage endpoint"
	default:
		return err.Error()
	}
}
