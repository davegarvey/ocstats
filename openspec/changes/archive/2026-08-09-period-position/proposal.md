## Why

The CLI reports *how much time is left* in each quota period ("resets in 2h 12m") but not *where in the period* that leaves the user — "2h left" reads completely differently in a 5-hour window than in a 7-day one. The period lengths are not in the API response, but they are knowable: documented on opencode.ai (5-hour / weekly / monthly buckets) and directly observable from `resetInSec` (the boundary date = now + resetInSec).

## What Changes

- Human `quota` output shows the position within each period (e.g. `1h 44m of 5h`, `day 6 of 7`, `day 11 of 30`) alongside the existing usage bar, reset time, and projection.
- Period boundaries are inferred per workspace from the observed expiry (`now + resetInSec`):
  - rolling: fixed 5-hour window, previous boundary = −5h
  - weekly: 7-day calendar window (verified: boundary at Monday 00:00 UTC), previous boundary = −7d
  - monthly: calendar month anchored at the observed day-of-month + time-of-day (verified via billing history: anchored to the subscription/billing day, e.g. the 28th), previous boundary = same day-of-month + time-of-day one month earlier, clamped when the day exceeds the previous month's length (29/30/31 anchors in Feb and short months)
- Position is computed as `elapsed / window length` where `elapsed = boundary_next − now − ...` (remaining) — reported as a neutral fact like the projection; no classification.
- A sanity check (elapsed + remaining ≈ window length) gates rendering: if the inferred window is inconsistent (e.g. a clamped month for a 31st-anchor), position is not shown rather than shown wrong.
- `--json` stays data-only: each bucket gains an additive nullable `periodLengthSec` (number when the window length is known, null otherwise); consumers derive position themselves. Existing fields, exit codes, and the rolling projection exclusion are unchanged.
- The `usage-pace` spec's rationale for excluding the rolling bucket from projections ("sliding window without a fixed period end") is corrected — the rolling bucket is a fixed 5-hour window — while the exclusion itself stays (out of scope to reopen).

## Capabilities

### New Capabilities
- *(none — this extends existing capabilities)*

### Modified Capabilities
- `usage-pace`: new requirements "Period boundary inference" (per-bucket period definitions, monthly clamping, sanity check) and "Position in period" (neutral elapsed/window computation for all three buckets); the "Usage projection" requirement becomes period-anchored (usage × window / elapsed, history-free, rolling included).
- `quota-display`: human output requirement gains position display and the period-position marker on usage bars; `--json` requirement gains the additive `periodLengthSec` field per bucket.
- `menu-bar`: the plugin requirement changes — the projected usage is derived from `periodLengthSec` (period-anchored) instead of the observation-window rate.

## Impact

- `internal/cli/pace.go` — period inference and position computation (no new dependencies; clock injection already exists).
- `internal/cli/output.go` — position rendering in human output and `periodLengthSec` in JSON.
- Tests: period inference (weekly Monday-UTC boundary, monthly same-day-of-month, clamp cases 29/30/31 + Feb, leap year), position math, sanity-check fallback, JSON shape, plugin projection.
- No changes to `internal/fetch`, the cache format, or exit codes. `scripts/ocstats.swiftbar.sh` derives its projection from `periodLengthSec` (period-anchored) instead of `ratePctPerHour`; existing JSON fields remain additive.
