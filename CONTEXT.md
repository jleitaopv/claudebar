# claudebar

A terminal display of how much of a Claude subscription's usage limits have been consumed, and when they reset.

## Language

### Limits

**Usage Window**:
A rolling period over which Anthropic caps a subscription's usage; consumption within it is counted across all devices and clients.
_Avoid_: Quota, period, bucket

**Session Window**:
The 5-hour Usage Window, starting at the first message after the previous one reset.
_Avoid_: Session, current session, 5h limit

**Weekly Window**:
The 7-day Usage Window.
_Avoid_: Week, weekly limit

**Utilization**:
The percentage of a Usage Window's limit consumed so far.
_Avoid_: Usage, percent used, consumption

**Reset Time**:
The moment a Usage Window ends and its Utilization returns to zero. A Usage Window with no Reset Time has not started yet.
_Avoid_: Expiry, renewal

**Plan**:
The Claude subscription tier (e.g. pro, max) whose limits the Usage Windows enforce.
_Avoid_: Tier, subscription

**Reading**:
The Utilization and Reset Time of every Usage Window, as fetched at one moment.
_Avoid_: Snapshot, sample, data

**Stale**:
Describes a Reading that is still displayed after a later fetch failed.
_Avoid_: Cached, old

### Clients

**Conversation**:
A single Claude Code run with its own session ID and transcript; many Conversations contribute to one Usage Window.
_Avoid_: Session, chat
