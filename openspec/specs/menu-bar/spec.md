# menu-bar Specification

## Purpose

Surfaces the Go subscription quota in the macOS menu bar via a SwiftBar plugin wrapping the CLI.

## Requirements

### Requirement: Menu bar quota display
The menu bar plugin SHALL display a compact current usage indicator in the menu bar (e.g., the rolling-bucket percentage), and SHALL expose a dropdown with the full rolling/weekly/monthly breakdown and reset times, refreshed on a schedule. The plugin SHALL derive any pace display itself from the JSON data fields: when a bucket's `periodLengthSec` is available, the plugin SHALL display that bucket's projected usage at period end (usage percentage multiplied by the period length divided by the time elapsed within the period); when `periodLengthSec` is null or the elapsed time within the period is not positive, the plugin SHALL display no projection for that bucket. The indicator SHALL display a warning when any bucket's projected usage at period end is 100% or greater; otherwise it SHALL display the default indicator. Raw usage percentages SHALL NOT by themselves change the indicator.

#### Scenario: Indicator rendered
- **WHEN** SwiftBar runs the plugin with a successful quota fetch
- **THEN** the menu bar shows the compact usage indicator and the dropdown lists all three buckets with percentages and reset times

#### Scenario: Projection derived from data
- **WHEN** the JSON includes a `periodLengthSec` for a bucket with positive elapsed time within the period
- **THEN** the plugin displays the projected usage at period end computed from the usage percentage, the period length, and the reset seconds

#### Scenario: No period length, no projection
- **WHEN** the JSON reports `periodLengthSec` as null for a bucket
- **THEN** the plugin displays no projection for that bucket

#### Scenario: Warning indicator for overshoot
- **WHEN** any bucket's projected usage at period end is 100% or greater
- **THEN** the indicator shows a warning distinct from the default indicator

#### Scenario: Default indicator for projections under the limit
- **WHEN** all bucket projections at period end are below 100%
- **THEN** the indicator shows the default indicator, regardless of raw usage percentages

#### Scenario: Stale or failing data
- **WHEN** the underlying CLI reports stale data or an error
- **THEN** the plugin displays the last-known values with a stale marker, or an explicit error menu when no data exists, and never exits with an empty menu

#### Scenario: Open website from dropdown
- **WHEN** the user clicks the dropdown item that opens the website
- **THEN** the default browser opens the opencode.ai Go quota page for the workspace ID reported by the CLI

### Requirement: No shared secrets in plugin
The plugin SHALL contain no credentials or secrets; it MUST only invoke the CLI, which owns the session, so replacing the plugin never exposes or duplicates the session.

#### Scenario: Plugin contains no secrets
- **WHEN** the plugin file is inspected
- **THEN** no session, cookie, or key material is present, and all data flows through the CLI's JSON output
