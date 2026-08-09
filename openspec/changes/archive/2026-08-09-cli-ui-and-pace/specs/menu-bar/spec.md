## MODIFIED Requirements

### Requirement: Menu bar quota indicator
The menu bar plugin SHALL display a compact current usage indicator in the menu bar (e.g., the rolling-bucket percentage), and SHALL expose a dropdown with the full rolling/weekly/monthly breakdown and reset times, refreshed on a schedule. The plugin SHALL derive any pace display itself from the JSON data fields: when a weekly or monthly bucket's `ratePctPerHour` is available, the plugin SHALL display that bucket's projected usage at period end (usage percentage plus rate multiplied by reset seconds converted to hours); when the rate is null, the plugin SHALL display no projection for that bucket.

#### Scenario: Indicator rendered
- **WHEN** SwiftBar runs the plugin with a successful quota fetch
- **THEN** the menu bar shows the compact usage indicator and the dropdown lists all three buckets with percentages and reset times

#### Scenario: Projection derived from data
- **WHEN** the JSON includes a `ratePctPerHour` for a weekly or monthly bucket
- **THEN** the plugin displays the projected usage at period end computed from the rate and reset seconds

#### Scenario: No rate, no projection
- **WHEN** the JSON reports `ratePctPerHour` as null for a bucket
- **THEN** the plugin displays no projection for that bucket

#### Scenario: Stale or failing data
- **WHEN** the underlying CLI reports stale data or an error
- **THEN** the plugin displays the last-known values with a stale marker, or an explicit error menu when no data exists, and never exits with an empty menu

#### Scenario: Open website from dropdown
- **WHEN** the user clicks the dropdown item that opens the website
- **THEN** the default browser opens the opencode.ai Go quota page for the workspace ID reported by the CLI
