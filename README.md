# claudebar

A small terminal dashboard showing how much of your Claude subscription's usage limits you've used, and when they reset.

```
 claudebar                          pro
 ─────────────────────────────────────
 Session  ██████████░░░░░░░░░░  48%
          resets in 2h 14m (14:29)

 Weekly   ███░░░░░░░░░░░░░░░░░  15%
          resets in 1d 8h (Sun 22:59)
 ─────────────────────────────────────
 updated 12s ago          r refresh  q quit
```

It shows the same account-wide numbers as Claude Code's `/usage` screen, so usage from claude.ai, the desktop app and your other machines is included. Leave it open in a tmux pane and you'll see a limit coming before you hit it.

## Requirements

- A Claude Pro or Max subscription, logged in through [Claude Code](https://claude.com/claude-code) on this machine. claudebar reuses that login, so there's nothing to configure.
- Linux. claudebar reads Claude Code's credentials from `~/.claude/.credentials.json`; on macOS Claude Code keeps them in the Keychain instead, which isn't supported yet.
- Go 1.27 or newer to build it.

API-key (pay-as-you-go) accounts have no usage limits to show, and claudebar will exit with a message saying so.

## Install

```sh
go install github.com/jleitaopv/claudebar@latest
```

Or from a clone:

```sh
go run .
```

## Usage

```sh
claudebar
```

| Key        | Action                    |
| ---------- | ------------------------- |
| `r`        | Fetch new numbers now     |
| `q`        | Quit                      |

What's on screen:

- **Session**: the rolling 5-hour window. It starts with your first message and resets 5 hours later. If you haven't sent anything recently it shows `0% · starts with your next message`.
- **Weekly**: the rolling 7-day window.
- **Bar colour**: green below 50%, yellow from 50% to 80%, red at 80% and above. With extra usage enabled you can go over 100%; the real number is shown.
- **Reset Time**: a live countdown plus the clock time in your time zone.
- **Status line**: how long ago the numbers were fetched. If a fetch fails (offline, expired login), the last numbers stay on screen marked `stale`, with the reason. For an expired login, run `claude` once and claudebar picks up the refreshed token on its next fetch.

claudebar fetches new numbers once a minute and the countdowns tick every second in between.

## How it works

claudebar calls `https://api.anthropic.com/api/oauth/usage`, the endpoint behind Claude Code's `/usage`, with the access token Claude Code stores. That endpoint isn't officially documented, so a change on Anthropic's side can break claudebar until it's updated. When that happens you'll see `unexpected response from the usage endpoint` rather than wrong numbers.

claudebar only ever reads your credentials file. It never refreshes or rewrites the token, so it can't interfere with Claude Code's login. The reasoning is in [ADR 0001](docs/adr/0001-read-utilization-from-undocumented-oauth-usage-endpoint.md).

## Development

```sh
go test ./...
```

The tests drive the whole app through its Bubble Tea model against a fake usage endpoint, a temporary credentials file and a fake clock, so they need no network or real login. The project's vocabulary (Usage Window, Utilization, Reset Time, …) is defined in [CONTEXT.md](CONTEXT.md).

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).
