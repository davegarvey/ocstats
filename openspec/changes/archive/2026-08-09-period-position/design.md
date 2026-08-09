## Context

The CLI already renders usage bars, severity colors, a neutral projection, and a data-only JSON contract (specs: `quota-display`, `usage-pace`). The API reports `usagePercent` and `resetInSec` but never the period length, so "resets in 2h" carries no sense of scale. The periods are nevertheless knowable: documented on opencode.ai (5-hour / weekly / monthly limits), observable in the data (`now + resetInSec` = the boundary), and corroborated by the account's billing history. See proposal.md — Why for motivation.

**Verified facts from real data:**
- Weekly: boundary lands on **Monday 00:00:00 UTC to the second** (two samples) — calendar-aligned 7-day window.
- Monthly: boundary = the subscription/billing **day-of-month + time-of-day** (payment history: May/Jun/Jul 28; quota boundary Aug 28 20:42:36 UTC matches the May signup moment to the minute; the Jun/Jul payment *times* drift ~1h but the quota boundary does not follow them).
- Rolling: `resetInSec` ticks down monotonically and jumps at a boundary — a fixed 5-hour block reading, not a sliding window (a sliding window has no clean expiry).

## Goals / Non-Goals

**Goals:**
- Neutral position-in-period display for all three buckets (rolling, weekly, monthly)
- Anchors derived from the data per workspace — no hardcoded day-of-month anywhere
- JSON stays data-only: additive nullable `periodLengthSec` per bucket
- Sanity check so an inconsistent inference renders nothing rather than wrong data
- Zero new dependencies, no new fetches, no cache format change

**Non-Goals:**
- Classifying, warning on, or interpreting position or projection (mirror, not judge)
- Position display in the SwiftBar plugin (consumer's choice; JSON carries the data)
- Modeling server-side period *changes* beyond what the next observed boundary reveals (self-correcting by construction: the anchor is re-derived every fetch)

## Decisions

### 1. Boundary inference per bucket

`boundary_next = snapshot.FetchedAt + resetInSec` (a fixed instant — for stale snapshots this uses the cached `FetchedAt`, and `elapsed` is measured with the current clock).

```
bucket     boundary_prev                window length
────────   ──────────────────────────   ─────────────────
rolling    boundary_next − 5h           fixed 5h
weekly     boundary_next − 7d           fixed 7d
monthly    same day-of-month + time,    calendar month
           previous month (clamped)     (28–31 days)
```

Weekly is exact to the second (verified); rolling is fixed arithmetic under the fixed-block reading; monthly is calendar arithmetic that reads the actual month lengths — no 30-day assumption anywhere.

### 2. Monthly previous-boundary rule with clamp

`prev = same time-of-day on day min(anchorDay, daysInMonth(prevMonth)) of the previous month, UTC arithmetic`.

- The clamp reproduces a clamping server's behavior when going backward: an anchor-31 boundary on May 31 yields Apr 30; on Mar 31 yields Feb 28 (29 in leap years).
- **Residual ambiguity — the clamped month itself:** when the *observed* boundary is a clamped one (e.g. the server clamped Jan 31 → Feb 28), the observed day (28) cannot reveal the true anchor (31). Position in that window may be off by up to 3 days. It self-corrects the next month (the Mar 31 boundary, clamped backward, recovers Feb 28 exactly). The sanity check (Decision 4) is the gate for this case.
- All arithmetic in UTC: `resetInSec` is a duration and boundary timestamps are instants, so DST never enters.

### 3. Position computation and rendering

`elapsed = clock() − boundary_prev`; `position = elapsed / window`. Rendered per bucket scale, with the position marker drawn on the usage bar at column `round(position × 20)`:

```
  Rolling  █│░░░░░░░░░░░░░░░░░░   6%   resets in 1h 8m   3h 51m of 5h
  Weekly   ████████████│░░░░░░░░  55%   resets in 9h 45m  day 6 of 7
  Monthly  ████████████│░░░░░░░░  36%   resets in 19d 6h  day 11 of 31
```

Hour-based for the 5h window, day-based for weekly/monthly (days since boundary / days in window, rounded). The marker and the text are the same datum in two forms — the marker makes fill-vs-position comparison instant, the text carries the exact values. Neutral by construction, same treatment as the projection.

### 4. Projection: period-anchored, history-free

`projected = usage × window / elapsed_at_fetch` (elapsed at the snapshot's fetch time; nil when the window is unknown or elapsed ≤ 0). This is the period's average consumption extrapolated — usage resets to 0 at the boundary, so `usage / elapsed` *is* the average burn rate of the period, no history required.

- This corrects an earlier design flaw: the previous projection extrapolated the *observation-window* rate (e.g. an hour of history) over the remaining period, producing absurd numbers like 562% for a user 37% into the period at 36% usage. The period-anchored form answers the actual question: "if this period's pace holds, where do we land?" (36% × 31d/11.5d ≈ 97%).
- Applies to **all three buckets** — rolling included, its projection exclusion is removed (the fixed-5h reading holds; the sliding-window discriminator observation noted in Risks remains the way to disprove it).
- The history-based rate (`ratePctPerHour`, `rateSpanSec`) remains in the JSON contract as data, but no longer drives the displayed projection.

### 5. Sanity check gates the inference

`|elapsed + resetInSec − window| ≤ tolerance`, tolerance = 24h (covers the ≤3-day clamp ambiguity's worst case; the check is exact for consistent months). On failure: `periodLengthSec` = null, no position rendered. This is the "don't show garbage" contract — a clamped month renders nothing rather than a wrong "day N of M".

### 6. JSON: one additive field

Per bucket: `periodLengthSec` (number|null). Consumers derive position as `(periodLengthSec − resetInSec) / periodLengthSec`. Data-only, consistent with `ratePctPerHour`/`rateSpanSec`; the SwiftBar plugin ignores it until it wants to.

### 7. No cache or fetch changes

Inference needs only the snapshot being rendered plus the clock — no history, no new API calls. The stale path works because `boundary_next` is a fixed instant derived from the snapshot's own timestamp.

## Risks / Trade-offs

- [Monthly anchor misread if the server changes period semantics] → anchor re-derived from every fetch's boundary; a change is absorbed at the next boundary; the sanity check gates the transition month
- [Clamped month (29/30/31 anchors) shows no position instead of a ±3-day one] → deliberate: the inference is genuinely ambiguous there; a hidden position beats a wrong one
- [Rolling is actually a sliding window after all] → position and projection drift within a bounded 5h; one observation settles it (a usage drop without a boundary jump back near 5h), and the fixed-block reading matches the clean expiry the API reports
- [Weekly DST month boundaries] → irrelevant: UTC arithmetic on fixed instants

## Migration Plan

Additive-only: new rendering on the same snapshot data, one nullable JSON field, no format changes, no rollback concerns. Old binaries and the plugin are unaffected (unknown JSON fields are ignored).

## Open Questions

- Whether the SwiftBar plugin should display position (deferrable; `periodLengthSec` is already in the JSON contract).
- Tolerance constant (24h) — revisit if real clamp-month data shows it should be tighter.
