## Purpose

Presents the Go subscription quota to a human or script from the terminal, with stable output formats.

## ADDED Requirements

### Requirement: Human-readable quota output
The default command output SHALL display the three quota buckets with their usage percentages and reset times in a compact table, plus the fetch time and a staleness indicator when showing cached data. When the data is stale beyond a maximum staleness threshold (24 hours by default), the output SHALL show the cached values with a prominent hard-error message and the fetch error.

#### Scenario: Fresh quota displayed
- **WHEN** the user runs the quota command with no flags and a fresh fetch succeeded
- **THEN** the output shows rolling, weekly, and monthly percentages and reset times in a readable table

#### Scenario: Stale data displayed
- **WHEN** the fetch failed but cached values younger than the maximum staleness threshold exist
- **THEN** the output shows the cached values with a clear indicator that the data is stale and how old it is

#### Scenario: Stale data beyond threshold
- **WHEN** the fetch failed and the cached values are older than the maximum staleness threshold
- **THEN** the output shows the cached values with a prominent error stating the data is too old, along with the underlying fetch error

### Requirement: Machine-readable output
The system SHALL support a `--json` flag that emits a single JSON object on stdout containing the same data the human format shows: each bucket (percentage, reset seconds), the workspace ID, a fetched-at timestamp, and a stale flag. Errors SHALL also be emitted as a single JSON object on stdout with a machine-readable error code.

#### Scenario: JSON output
- **WHEN** the user runs the quota command with `--json`
- **THEN** stdout contains one JSON object with rolling, weekly, and monthly buckets, the workspace ID, fetched-at timestamp, and stale flag

#### Scenario: No subscription in JSON mode
- **WHEN** the account has no Go subscription and `--json` is used
- **THEN** stdout contains a single JSON object reporting `subscription: null` and the process exits 0

#### Scenario: Errors in JSON mode
- **WHEN** the fetch fails, no cache exists, and `--json` is used
- **THEN** the command exits with the fetch error code and emits a JSON error object on stdout with a machine-readable error code

### Requirement: Exit codes
The system SHALL exit 0 whenever quota data is shown, whether fresh or stale within the staleness threshold. Distinct non-zero exit codes SHALL distinguish: authentication failure, fetch failure (network/server/decode), too-stale data, and usage errors.

#### Scenario: Success exit code
- **WHEN** the quota command completes with fresh data, or with stale data within the threshold
- **THEN** the process exits with status 0

#### Scenario: Authentication failure
- **WHEN** the stored session is rejected by opencode.ai
- **THEN** the process exits with the authentication exit code and tells the user to run the login command again

#### Scenario: Fetch failure with no cache
- **WHEN** the quota cannot be fetched and no cache exists
- **THEN** the process exits with the fetch exit code and an actionable error message

#### Scenario: Data too stale
- **WHEN** only cached data exists and it is beyond the maximum staleness threshold
- **THEN** the process exits with the too-stale exit code while still showing the cached values

#### Scenario: Usage error
- **WHEN** the user passes invalid flags or arguments
- **THEN** the process exits with the usage exit code and prints usage help
