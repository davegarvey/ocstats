## ADDED Requirements

### Requirement: Menu bar quota display
The menu bar plugin SHALL display a compact current usage indicator in the menu bar (e.g., the rolling-bucket percentage), and SHALL expose a dropdown with the full rolling/weekly/monthly breakdown and reset times, refreshed on a schedule. The plugin SHALL derive any pace display itself from the JSON data fields: when a bucket's `periodLengthSec` is available, the plugin SHALL display that bucket's projected usage at period end (usage percentage multiplied by the period length divided by the time elapsed within the period); when `periodLengthSec` is null or the elapsed time within the period is not positive, the plugin SHALL display no projection for that bucket.

#### Scenario: Indicator rendered
- **WHEN** SwiftBar runs the plugin with a successful quota fetch
- **THEN** the menu bar shows the compact usage indicator and the dropdown lists all three buckets with percentages and reset times

#### Scenario: Projection derived from data
- **WHEN** the JSON includes a `periodLengthSec` for a bucket with positive elapsed time within the period
- **THEN** the plugin displays the projected usage at period end computed from the usage percentage, the period length, and the reset seconds

#### Scenario: No period length, no projection
- **WHEN** the JSON reports `periodLengthSec` as null for a bucket
- **THEN** the plugin displays no projection for that bucket

#### Scenario: Stale or failing data
- **WHEN** the underlying CLI reports stale data or an error
- **THEN** the plugin displays the last-known values with a stale marker, or an explicit error menu when no data exists, and never exits with an empty menu

#### Scenario: Open website from dropdown
- **WHEN** the user clicks the dropdown item that opens the website
- **THEN** the default browser opens the opencode.ai Go quota page for the workspace ID reported by the CLI

## REMOVED Requirements

### Requirement: Menu bar quota indicator
**Reason**: The projection derivation changes from the observation-window rate (`ratePctPerHour`) to the period-anchored formula, so the requirement's scenarios are replaced.
**Migration**: The replacement "Menu bar quota indicator" requirement (in ADDED above) derives projections from `periodLengthSec` and shows none when it is null.
