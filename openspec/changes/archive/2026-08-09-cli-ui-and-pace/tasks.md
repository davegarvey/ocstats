## 1. Cache v2: per-workspace history

- [x] 1.1 Extend `internal/cli/cache.go` to a versioned format v2: per-workspace key and bounded chronological history of snapshots; keep the atomic tmp+rename write and `OCSTATS_CACHE_FILE` override
- [x] 1.2 Treat unreadable, v1, or unknown-version cache files as empty and rebuild from the next successful fetch
- [x] 1.3 Enforce the retention bound (drop oldest snapshots beyond 31 days / 2000 entries per workspace)
- [x] 1.4 Exclude no-subscription snapshots from history (still cached for stale fallback)
- [x] 1.5 Cache tests: write/read roundtrip, history ordering, retention eviction, version-mismatch fallback, offline stale read, no-sub exclusion

## 2. Pace engine (usage-pace)

- [x] 2.1 Introduce a deterministic clock (now) injection covering snapshot stamping, rate windows, and stale-age computation
- [x] 2.2 Implement rate estimation: window endpoints (first→latest snapshot of the current period), require ≥ 2 snapshots and a positive span, clamp rate ≥ 0, report the observation span alongside
- [x] 2.3 Implement reset detection: usage drop between consecutive snapshots (with a resetInSec uptick as a secondary signal) excludes all pre-drop history from the current period
- [x] 2.4 Implement the projection: usage + rate × (resetInSec/3600), rounded to integer percent, weekly and monthly only, reported as a neutral value with no classification
- [x] 2.5 Pace engine tests: window-endpoint rate, insufficient history, zero-duration window (rate unavailable, never NaN), negative rate clamp, reset exclusion, projection units (54% + 2%/h × 11.9h → 78%), rolling excluded

## 3. Human output: bars, colors, projection

- [x] 3.1 Render fixed 20-column usage bars (`█`/`░`, rounded) per bucket in `internal/cli/output.go`
- [x] 3.2 Add severity color coding (green <60, yellow 60–89, red ≥90) gated on stdout being a char device and `NO_COLOR` unset; never emit ANSI when piped
- [x] 3.3 Render the neutral projected usage (`→ X% by reset`) on weekly/monthly bucket lines when computable; nothing when not
- [x] 3.4 Preserve the existing stale/too-stale and no-subscription rendering paths
- [x] 3.5 Output tests: bar proportions, no ANSI when piped, projection present/absent, stale rendering unchanged

## 4. JSON contract

- [x] 4.1 Add additive `ratePctPerHour` and `rateSpanSec` (both number|null, always a pair) per bucket to `--json` output; no projected values or classifications in JSON
- [x] 4.2 JSON tests: rate pair present, rate pair null, existing fields and error shape unchanged

## 5. CLI wiring

- [x] 5.1 Wire `cmdQuota` to load cache history for the resolved workspace, append fresh snapshots (never on stale runs, never no-sub snapshots), compute pace, persist, and pass results to both renderers
- [x] 5.2 Stale-fallback path: workspace-keyed read, projection computed with age-adjusted remaining time (resetInSec − elapsed since fetchedAt), no history append
- [x] 5.3 Verify exit codes 0/2/3/4/64 and existing error paths are unchanged; run `gofmt`, `go vet ./...`, `go test ./...`

## 6. SwiftBar plugin

- [x] 6.1 Derive and display the projected usage in `scripts/ocstats.swiftbar.sh` from `ratePctPerHour` with correct units (rate × resetInSec/3600), weekly and monthly only, hidden when the rate is null
- [x] 6.2 Verify indicator, dropdown, stale, and error menus still render

## 7. End-to-end manual verification

- [x] 7.1 Build and install; run `login` and `quota` fresh: bars + colors in a terminal, no ANSI when piped, JSON shape correct
- [x] 7.2 Confirm projection appears only after multiple fetches and only on weekly/monthly (covered by automated pace-engine tests; no manual timestamp editing)
- [x] 7.3 Simulate a period reset (usage drop) and confirm the rate window restarts
- [x] 7.4 Confirm a leftover v1 cache file is ignored gracefully and rebuilt
- [x] 7.5 Regression: exit codes 0/2/3/4/64, cross-workspace stale read, `--json` error shape
