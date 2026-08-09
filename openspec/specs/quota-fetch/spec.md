# quota-fetch Specification

## Purpose

Authenticates with opencode.ai and retrieves the Go (lite) subscription quota, so quota values are available without visiting the website.

## Requirements

### Requirement: Login captures the session from the local browser
The system SHALL support a login command that extracts the opencode.ai session cookie (`auth`, host-only, 365-day expiry) from the user's local browser profile: Firefox profiles are read from `profiles.ini` and `cookies.sqlite`; the session value is read without requiring the user to paste anything. A manual fallback (`--cookie`) SHALL exist for browsers the tool does not support. The login command SHALL validate the captured session with an authenticated request before considering the login successful.

#### Scenario: Login from a supported browser
- **WHEN** the user runs the login command and is logged in to opencode.ai in a supported browser
- **THEN** the session cookie is read from the browser profile, validated against opencode.ai, and stored for subsequent fetches

#### Scenario: Manual cookie fallback
- **WHEN** the user runs the login command with `--cookie` and a valid session value
- **THEN** the provided value is validated and stored, bypassing browser detection

#### Scenario: Invalid or expired session
- **WHEN** the captured session fails validation against opencode.ai
- **THEN** the login fails with an actionable message and the previously stored session, if any, is left untouched

### Requirement: Session storage is user-only
The session cookie SHALL be persisted in a user-only store (macOS Keychain item or a file with 0600 permissions), never in the repository, never in world-readable files, and never printed in command output.

#### Scenario: Session persistence
- **WHEN** the user logs in and later runs a quota command
- **THEN** the session is read from the store and used without re-authentication

#### Scenario: Access restricted to the user
- **WHEN** the stored session is inspected by another OS user on the machine
- **THEN** it cannot be read (0600 file permissions or Keychain access control)

#### Scenario: Secret hygiene
- **WHEN** the system reads or writes the session
- **THEN** the cookie value never appears in stdout, stderr, or logs

### Requirement: Fetch Go subscription quota
The system SHALL fetch the Go subscription quota from opencode.ai using the stored session, returning the three usage buckets the website shows — rolling (sliding window), weekly, and monthly. Each bucket SHALL include a usage percentage (integer 0-100) and a reset time (integer seconds until reset), decoded from the server-action response. The response SHALL be validated against a fixed schema; decode failures are treated as fetch errors.

#### Scenario: Successful fetch
- **WHEN** a valid session is used to fetch quota
- **THEN** the system returns rolling, weekly, and monthly usage percentages and reset seconds decoded from the server response, and rejects responses that do not match the expected schema

#### Scenario: No Go subscription
- **WHEN** the account has no Go (lite) subscription
- **THEN** the fetch result explicitly reports "no subscription" rather than an error, and no percentages are fabricated

#### Scenario: Fetch failure
- **WHEN** the quota fetch fails (network error, server error, decode failure, or changed protocol)
- **THEN** the system reports a fetch error and, if previous values are cached, surfaces the cached values as stale instead of failing silently

### Requirement: Cache last-known quota
The system SHALL cache recent successful quota snapshots locally, each with a fetched-at timestamp, so a consumer (including the menu bar plugin) can show last-known values during outages and the system can estimate consumption rates across fetches. The cache SHALL keep a bounded history of snapshots in chronological order, retaining only recent data. The cache format SHALL be versioned; an unreadable or outdated cache SHALL be treated as empty and rebuilt from the next successful fetch.

#### Scenario: Offline fallback
- **WHEN** the quota fetch fails but a previous successful fetch was cached
- **THEN** the consumer receives the cached values flagged as stale with the age of the data

#### Scenario: History retained
- **WHEN** multiple successful fetches occur for the same workspace
- **THEN** the cache holds the recent snapshots in chronological order with their fetched-at timestamps

#### Scenario: Bounded history
- **WHEN** the history exceeds the retention limit
- **THEN** the oldest snapshots are dropped from the cache

#### Scenario: Corrupt or outdated cache
- **WHEN** the cache file is unreadable or uses an unsupported format version
- **THEN** it is treated as empty and rebuilt from the next successful fetch

### Requirement: Workspace selection
The system SHALL resolve which workspace to report on: automatically using the account's last-seen workspace as reported by opencode.ai, with a `--workspace <id>` override flag. When no workspace can be resolved, the fetch SHALL fail with an actionable error.

#### Scenario: Automatic workspace
- **WHEN** the user runs a quota command without `--workspace`
- **THEN** the last-seen workspace from the account session is used

#### Scenario: Explicit workspace
- **WHEN** the user passes `--workspace <id>`
- **THEN** quota is fetched for that workspace regardless of the last-seen value

#### Scenario: No workspace resolvable
- **WHEN** the session has no workspace membership and no override is given
- **THEN** the command fails with a message telling the user to open the opencode.ai workspace page and retry, or pass `--workspace`
