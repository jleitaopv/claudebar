# Read Utilization from the undocumented OAuth usage endpoint

claudebar gets Session Window and Weekly Window Utilization and Reset Times from `GET https://api.anthropic.com/api/oauth/usage` (header `anthropic-beta: oauth-2025-04-20`), the same undocumented endpoint Claude Code's `/usage` screen uses, authenticated with the access token Claude Code stores in `~/.claude/.credentials.json`. It is the only source that reports Utilization against the plan's limits across all devices and clients; summing tokens from local Claude Code transcripts only sees this machine and can't be converted to a percentage, because Anthropic doesn't publish limits in tokens.

## Consequences

- The endpoint can change or disappear without notice. All knowledge of its URL, headers and JSON shape lives in one adapter so a break is fixed in one place, and a failed or unparseable poll leaves the last good values on screen marked stale rather than crashing.
- claudebar never writes to `~/.claude/.credentials.json` and never uses the refresh token, even though that would let it recover from an expired access token. Refreshing concurrently with Claude Code risks one process overwriting the other's rotated token and logging the user out. On expiry, claudebar tells the user to run `claude`.
- Polling is kept to once per 60 seconds plus manual refresh, since the endpoint's rate limit is unknown.
