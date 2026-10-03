// Command claudebar shows how much of a Claude subscription's Session and
// Weekly Windows has been used, and when each resets.
package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jleitaopv/claudebar/internal/tui"
	"github.com/jleitaopv/claudebar/internal/usage"
)

// fetchTimeout bounds a single poll so a hung request can't block refreshes.
const fetchTimeout = 10 * time.Second

// systemClock is the real wall clock, ticking once a second.
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) Tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tui.TickMsg(t) })
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "claudebar:", err)
		os.Exit(1)
	}
}

func run() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("finding home directory: %w", err)
	}
	credentialsPath := filepath.Join(home, ".claude", ".credentials.json")
	clock := systemClock{}
	model, err := tui.New(tui.Deps{
		Usage: usage.NewClient(usage.ClientConfig{
			BaseURL:         usage.DefaultBaseURL,
			CredentialsPath: credentialsPath,
			HTTPClient:      &http.Client{Timeout: fetchTimeout},
			Now:             clock.Now,
		}),
		CredentialsPath: credentialsPath,
		Clock:           clock,
	})
	if errors.Is(err, usage.ErrNoLogin) {
		return err
	}
	if err != nil {
		return fmt.Errorf("starting: %w", err)
	}
	_, err = tea.NewProgram(model).Run()
	return err
}
