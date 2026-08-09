## MODIFIED Requirements

### Requirement: Human-readable quota output
The default command output SHALL display the three quota buckets in a compact table: each bucket SHALL show a usage bar, its usage percentage, and its reset time, plus the fetch time and a staleness indicator when showing cached data. Buckets with a computable projection SHALL additionally show the projected usage at period end as a neutral value. When stdout is a terminal, usage bars SHALL be color-coded by severity; color MAY be disabled via the `NO_COLOR` environment variable, and SHALL NOT be emitted when stdout is not a terminal. When the data is stale beyond a maximum staleness threshold (24 hours by default), the output SHALL show the cached values with a prominent hard-error message and the fetch error.

#### Scenario: Fresh quota displayed
- **WHEN** the user runs the quota command with no flags and a fresh fetch succeeded
- **THEN** the output shows rolling, weekly, and monthly usage bars, percentages, and reset times in a readable table

#### Scenario: Usage bar length
- **WHEN** a bucket has a usage percentage
- **THEN** its usage bar is filled in proportion to the percentage

#### Scenario: Color on a terminal
- **WHEN** stdout is a terminal and `NO_COLOR` is not set
- **THEN** usage bars are color-coded by severity

#### Scenario: No color when piped
- **WHEN** stdout is not a terminal or `NO_COLOR` is set
- **THEN** the output contains no ANSI escape sequences

#### Scenario: Projection displayed
- **WHEN** a bucket has a computable projection
- **THEN** its line shows the projected usage at period end as a neutral value

#### Scenario: No projection displayed
- **WHEN** a bucket has no computable projection
- **THEN** its line shows no projection

#### Scenario: Stale data displayed
- **WHEN** the fetch failed but cached values younger than the maximum staleness threshold exist
- **THEN** the output shows the cached values with a clear indicator that the data is stale and how old it is

#### Scenario: Stale data beyond threshold
- **WHEN** the fetch failed and the cached values are older than the maximum staleness threshold
- **THEN** the output shows the cached values with a prominent error stating the data is too old, along with the underlying fetch error

### Requirement: Machine-readable output
The system SHALL support a `--json` flag that emits a single JSON object on stdout containing the bucket data (usage percentage, reset seconds, and consumption rate per bucket), the workspace ID, a fetched-at timestamp, and a stale flag. The per-bucket consumption rate (`ratePctPerHour`) and its observation span (`rateSpanSec`) SHALL be numbers when a rate can be estimated and null otherwise. JSON output SHALL be data only: it SHALL NOT include classifications, verdicts, or warnings, which consumers derive from the data. Errors SHALL also be emitted as a single JSON object on stdout with a machine-readable error code.

#### Scenario: JSON output
- **WHEN** the user runs the quota command with `--json`
- **THEN** stdout contains one JSON object with rolling, weekly, and monthly buckets including their usage percentages, reset seconds, and rate fields, the workspace ID, fetched-at timestamp, and stale flag

#### Scenario: Rate available in JSON
- **WHEN** a consumption rate can be estimated for a bucket
- **THEN** its `ratePctPerHour` and `rateSpanSec` fields are numbers

#### Scenario: Rate unavailable in JSON
- **WHEN** a consumption rate cannot yet be estimated for a bucket
- **THEN** its `ratePctPerHour` and `rateSpanSec` fields are null

#### Scenario: No subscription in JSON mode
- **WHEN** the account has no Go subscription and `--json` is used
- **THEN** stdout contains a single JSON object reporting `subscription: null` and the process exits 0

#### Scenario: Errors in JSON mode
- **WHEN** the fetch fails, no cache exists, and `--json` is used
- **THEN** the command exits with the fetch error code and emits a JSON error object on stdout with a machine-readable error code
