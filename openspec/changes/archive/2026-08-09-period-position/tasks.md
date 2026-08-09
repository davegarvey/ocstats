## 1. Period inference (usage-pace)

- [x] 1.1 Add period constants (rolling 5h, weekly 7d) and boundary inference to `internal/cli/pace.go`: `boundaryNext = fetchedAt + resetInSec`; `boundaryPrev = boundaryNext − 5h` (rolling) / `− 7d` (weekly); window length = boundaryNext − boundaryPrev; all UTC arithmetic
- [x] 1.2 Implement the monthly rule: previous boundary = same day-of-month + time-of-day in the previous calendar month, day clamped to `daysInMonth` (leap-year aware); boundary derived from the observed day-of-month, never hardcoded
- [x] 1.3 Implement the sanity check: window unavailable when `|elapsed + resetInSec − window|` exceeds the 24h tolerance or the current time is past the next boundary
- [x] 1.4 Implement position computation (elapsed/window) with rendering units: hours for rolling, whole days for weekly/monthly; nil when the window is unknown
- [x] 1.5 Pace engine tests: rolling/weekly boundaries, monthly same-day (Mar 28 → Feb 28), clamps (May 31 → Apr 30, Mar 31 → Feb 28, leap-year Feb 29 → Jan 29), inconsistent window unavailable, position math, rolling included, stale snapshot boundary derived from fetchedAt

## 2. Period-anchored projection (usage-pace rework)

- [x] 2.1 Replace the rate-based projection with `usage × window / elapsed_at_fetch`: nil when the window is unknown or elapsed ≤ 0; computed for rolling, weekly, and monthly alike (rolling exclusion removed)
- [x] 2.2 Remove the projection from the rate-based pace computation (history no longer drives the projection); the observation-window rate stays in JSON only
- [x] 2.3 Pace engine tests: projection formula (36% × 31d/11.5d → 97%), nil at period start, nil without a window, rolling included, stale projection uses fetch-time elapsed while position uses the current clock

## 3. Output: position, marker, projection

- [x] 3.1 Render the neutral position on bucket lines in `internal/cli/output.go` (`1h 43m of 5h`, `day 6 of 7`, `day 11 of 31`); nothing when unknown
- [x] 3.2 Render the position marker (`│`) on the usage bar at `round(position × 20)` when the window is known; no marker otherwise; marker inherits the bar color
- [x] 3.3 Render the period-anchored projection (`→ X% by reset`) on all three bucket lines when computable
- [x] 3.4 Add additive nullable `periodLengthSec` per bucket to `--json` output; no positions or projections in JSON
- [x] 3.5 Output tests: position shown/hidden, marker present at the right column/absent, projection shown/hidden, JSON field number/null, existing bar/color/stale assertions unchanged

## 4. CLI wiring

- [x] 4.1 Wire period computation into `cmdQuota` for both the fresh and stale paths (stale: boundary from the cached snapshot's fetchedAt, position elapsed from the current clock, projection from fetch-time elapsed)
- [x] 4.2 Run `gofmt`, `go vet ./...`, `go test ./...`

## 5. End-to-end verification

- [x] 5.1 Build and run `quota` fresh: marker + position + period-anchored projection on all three buckets (monthly ≈ 36% × 31d/11.5d ≈ 97%, not the old rate-based 562%); JSON shape correct
- [x] 5.2 Stale path: position marker advances with the clock, projection stays fetch-time; positions hidden when the cached periods have ended
- [x] 5.3 Regression: exit codes 0/2/3/4/64 unchanged; SwiftBar plugin still renders correctly against the new JSON (extra field ignored)

## 6. SwiftBar plugin (period-anchored projection)

- [x] 6.1 Update `scripts/ocstats.swiftbar.sh`: derive the projected usage from `periodLengthSec` (usage × period / (period − resetInSec)) for all buckets; no projection when `periodLengthSec` is null or the elapsed time within the period is not positive
- [x] 6.2 Verify the plugin against real JSON (sensible projections, no absurd rate-based numbers), null-period JSON (no projection), and error JSON; indicator, dropdown, stale, and error menus intact
