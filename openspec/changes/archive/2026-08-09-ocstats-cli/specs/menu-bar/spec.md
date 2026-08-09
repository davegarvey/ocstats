## Purpose

Surfaces the Go subscription quota in the macOS menu bar via a SwiftBar plugin wrapping the CLI.

## ADDED Requirements

### Requirement: Menu bar quota indicator
The menu bar plugin SHALL display a compact current usage indicator in the menu bar (e.g., the rolling-bucket percentage), and SHALL expose a dropdown with the full rolling/weekly/monthly breakdown and reset times, refreshed on a schedule.

#### Scenario: Indicator rendered
- **WHEN** SwiftBar runs the plugin with a successful quota fetch
- **THEN** the menu bar shows the compact usage indicator and the dropdown lists all three buckets with percentages and reset times

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
