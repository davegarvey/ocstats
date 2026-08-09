## Why

The CLI currently dumps three plain-text percentages with no visual hierarchy, so reading it requires parsing numbers. And it can only report *position* (usage % now) — not the pace of consumption, which needs history the tool already collects through its cache and the 15-minute SwiftBar loop.

**Principle: the CLI is a mirror, not a judge.** It shows the facts and computes them honestly; the user interprets. A heavy session early in a period *should* show up as a high projected usage — the user knows they did it, and it was their decision. No warnings, no gates, no hidden data.

## What Changes

- Human `quota` output becomes visually scannable: usage bars per bucket, color-coded by severity, with a neutral projected usage at period end (`→ X% by reset`) shown whenever a rate can be computed.
- Pace computation: the cache grows from a single snapshot to a bounded per-workspace history; the CLI estimates the consumption rate as the average over the observed window (percent/hour), clamped non-negative, with a guard against zero-duration windows.
- Honest-number guard rails: reset detection (usage drop, with a resetInSec uptick as secondary signal) restarts the window; no-subscription snapshots never enter history; stale-fallback projections adjust for elapsed time and read the right workspace.
- `--json` stays data-only and opinion-free: each bucket gains `ratePctPerHour` and `rateSpanSec` (both nullable). No classifications, verdicts, or projections — consumers compute their own. Existing fields and exit codes are unchanged (backwards compatible).
- SwiftBar plugin derives the projected usage itself from the JSON fields with correct units.

## Capabilities

### New Capabilities
- `usage-pace`: consumption rate estimation (average over the observed window, clamped non-negative), same-period history tracking, reset detection, and the neutral projected-usage-at-period-end computation.

### Modified Capabilities
- `quota-fetch`: the "Cache last-known quota" requirement changes — the cache stores a bounded per-workspace history of snapshots (excluding no-subscription snapshots) instead of only the last snapshot, enabling rate estimation.
- `quota-display`: human output requirement changes — usage bars, severity colors, and neutral projected usage in the default output; `--json` gains the additive `ratePctPerHour`/`rateSpanSec` fields per bucket.
- `menu-bar`: the plugin requirement changes — it derives and displays its own projected usage from the JSON data.

## Impact

- `internal/cli/` — `output.go` (bars, colors, projection rendering), `cache.go` (versioned per-workspace history), `cli.go` (pace wiring, stale-path fixes), new pace engine.
- `internal/fetch/` — unchanged (Snapshot carries the same fields).
- `scripts/ocstats.swiftbar.sh` — projected usage from `ratePctPerHour`.
- Tests: output tests extended for bars/projection; new tests for rate estimation, reset detection, clamping, and projection units. Existing string assertions may need updating for the new layout.
- No new dependencies (TTY detection via stdlib, `NO_COLOR` honored). No changes to exit codes or `--json` field names.
