## Context

The CLI is a one-shot, stdlib-only Go binary: `quota` fetches three buckets (rolling/weekly/monthly) and renders plain text or JSON; `--json` is consumed by a SwiftBar menu bar plugin on a 15-minute schedule. The cache (`internal/cli/cache.go`) stores exactly one `fetch.Snapshot` (single-object JSON, atomic tmp+rename write). Exit codes and the JSON field set are stable contracts (specs: `quota-display`, `quota-fetch`, `menu-bar`). Rate estimation is impossible today: velocity needs two points, and the cache deliberately keeps only the last one. See proposal.md — Why for motivation.

**Product principle: the CLI is a mirror, not a judge.** It shows the facts and computes them honestly; it never classifies, warns, gates, or hides data. The user brings the context — they know they had a heavy session, and it was their decision.

## Goals / Non-Goals

**Goals:**
- Scannable human output: usage bars, severity colors, neutral projected usage at period end
- Pace data computed honestly from cache history (no new data sources, no API changes)
- `--json` stays data-only: additive `ratePctPerHour` + `rateSpanSec`, no classifications
- Zero new dependencies (stdlib `os.Stat` TTY detection; `NO_COLOR` honored)
- Backwards compatibility: exit codes and existing JSON field names unchanged

**Non-Goals:**
- Verdicts, warnings, thresholds, or any classification of the projection
- Warming-up gates or hysteresis state (projections show whenever computable)
- Watch/interactive mode, live refresh
- Server-side or `internal/fetch` changes
- Projections for the rolling bucket (sliding window: no fixed period end)
- Terminal-width-scaled bars (fixed width is deterministic and testable)

## Decisions

### 1. Cache format v2: per-workspace history

```
{
  "version": 2,
  "workspaces": {
    "wrk_01KN…": {
      "history": [ { "snapshot": …, "fetchedAt": … }, … ]   // chronological, bounded
    }
  }
}
```

- **Per-workspace keying**: mixing periods across `--workspace` switches would poison rate estimates. Keying is cheap and correct.
- **No verdict state**: with no classification, nothing to persist beyond history — the cache stays a pure data store.
- **Versioned**: old v1 file fails to unmarshal into v2 → treated as empty, rebuilt on next fetch. Rolling back the binary to v1 behaves symmetrically: v2 JSON yields a zero snapshot → `IsZero` guard → treated as no cache. Both directions degrade gracefully to a cold cache, never to wrong data.
- **Bounded**: retain at most 31 days / 2000 snapshots per workspace (whichever comes first), drop oldest.
- Preserve the existing atomic tmp+rename write and `OCSTATS_CACHE_FILE` override.
- **No-subscription snapshots are cached but never appended to history** — zeroed buckets would drag the rate toward 0 and misrepresent the period (bad data, not honesty).

### 2. Rate: average since first observation, unvarnished

`v = (u_latest − u_first) / (t_latest − t_first)` across the current period's snapshots, clamped ≥ 0, unavailable when < 2 snapshots or span ≤ 0.

- **No rebasing, no gating.** An early heavy session inflates the average — and that is the data. The user knows why the number is high and can interpret it; the projection decays naturally as light usage accumulates (it converges to the true period end as the window fills the period).
- Alternatives considered: last consecutive pair (noisier and *more* misleading — it amplifies a single burst), least-squares regression (a model choice that hides the plain reading of the data), EWMA (stateful smoothing that hides the actual average). All rejected: they replace the honest datum with an opinion.
- **Reset detection**: a usage drop between consecutive snapshots excludes pre-drop history (a reset mixes two periods in one window — that would be a wrong number). A jump *up* in `resetInSec` is used as a secondary reset signal for cases where a clamp at 100% hides the drop.
- **NaN guard**: span ≤ 0 → rate null. A NaN rate would make `json.Marshal` fail and violate the `--json` error contract.

### 3. Projection: one formula, correct units

`projected = usage + rate × (resetInSec / 3600)` — rate is %/hour, resetInSec is seconds. The unit conversion is the single most important detail (a factor-3600 error makes every projection nonsense). Rounded to the nearest integer percent for display. Rolling bucket: no projection (no fixed period end). Neutral by construction: no ⚠, no threshold, no color on the projected value — it is displayed exactly like any other datum.

### 4. Human output

```
Go quota · wrk_01KNCTQCZ0CJQR2ZE0F4PQA27W   fetched 14:06
  Rolling  ████░░░░░░░░░░░░░░░░   18%   resets in 3h 16m
  Weekly   ███████████░░░░░░░░░   54%   resets in 11h 53m   → 78% by reset
  Monthly  ███████░░░░░░░░░░░░░   35%   resets in 19d 8h    → 118% by reset
```

- **Bars**: fixed 20 columns, `█` × round(pct/5), `░` remainder. Deterministic and testable.
- **Colors**: green < 60%, yellow 60–89%, red ≥ 90% — identical to `color()` in `scripts/ocstats.swiftbar.sh`, so the menu bar and terminal tell the same story. TTY detection via `os.Stdout.Stat()` `ModeCharDevice` (tests pipe stdout, so they naturally exercise the no-color path); `NO_COLOR` env override; JSON never colored.
- **Projection**: shown whenever computable, silent when not — silence here means "no datum exists", not "data withheld".
- Stale/too-stale and no-subscription rendering unchanged.

### 5. JSON: additive, data-only

Per bucket: `ratePctPerHour` and `rateSpanSec` (both number|null, always as a pair). The span answers "how much evidence does this rate rest on?" — it is context data, not a verdict, and it lets consumers judge the rate for themselves. No projected value, no classification: projection is one line of consumer arithmetic from `ratePctPerHour` + `resetInSec`, and consumers should choose their own horizons.

### 6. SwiftBar plugin

Computes the projection itself with the corrected units (`rate × (resetInSec/3600)`), weekly and monthly only, nothing shown when the rate is null. No marker or judgment — the dropdown simply gains the projected value as a line of data.

## Risks / Trade-offs

- [Bursty usage makes projections noisy/high right after a burst] → accepted by design: the number is the honest average-so-far; the user interprets it. The projection converges toward the true period end as the window grows.
- [A clamp at 100% can hide a reset, mixing periods in the window] → secondary reset signal (resetInSec uptick) plus rate clamped ≥ 0 limits the damage; a wrong number from a missed reset decays as new history accumulates.
- [Users who run rarely get a rate covering a long, ragged window] → still honest data; the JSON span field lets consumers weight it, and the human output shows the number without pretending precision.
- [Cache v2 format change momentarily loses stale fallback] → version guard makes both forward and backward reads fail safe to a cold cache, never wrong data.
- [Rolling bucket gets no projection] → intentional: there is no period end to project to.

## Migration Plan

1. Ship the new binary; on first run the v1 cache is ignored (version mismatch → empty cache) and rebuilt.
2. Update `scripts/ocstats.swiftbar.sh` in the same change (repo ships both). The old plugin continues to work against the new JSON — it simply ignores the extra fields.
3. Rollback: revert the binary and/or plugin; the v2 cache is ignored by the old binary (fail-safe cold cache), no manual cleanup needed.
