# usage-pace Specification

## Purpose

Estimates how fast Go quota is being consumed and shows what usage at period end would be if the current pace holds, so the user can interpret for themselves whether they will hit the limit.

## Requirements

### Requirement: Consumption rate estimation
The system SHALL estimate each bucket's consumption rate as percentage points per hour from at least two snapshots taken within the same quota period. The rate SHALL be computed over the full observed window of the current period (from the first to the latest snapshot), not from the most recent consecutive pair. The rate SHALL be clamped to a minimum of zero, and SHALL be unavailable when fewer than two snapshots exist in the period or when the observed window has zero or negative duration. When a rate is available, the span of the observed window SHALL be reported alongside it.

#### Scenario: Rate over the observed window
- **WHEN** snapshots exist in the same period at 0h (30%), 2h (51%), and 4h (54%)
- **THEN** the rate is 6 percentage points per hour (window endpoints), not the rate of the final segment alone

#### Scenario: Rate clamped to zero
- **WHEN** the computed rate is negative
- **THEN** the rate is reported as zero

#### Scenario: Insufficient history
- **WHEN** fewer than two snapshots exist within the current period
- **THEN** the rate is unavailable

#### Scenario: Zero-duration window
- **WHEN** two snapshots exist but with equal fetched-at timestamps
- **THEN** the rate is unavailable rather than undefined

#### Scenario: Span reported with rate
- **WHEN** a rate is available
- **THEN** the observation span it was computed over is reported alongside it

### Requirement: Reset detection
The system SHALL detect quota period resets: when a snapshot shows lower usage than the previous snapshot of the same bucket, all earlier snapshots SHALL be excluded from the current period's history for rate estimation.

#### Scenario: Reset observed
- **WHEN** a snapshot shows lower usage than the previous snapshot of the same bucket
- **THEN** snapshots before the drop are no longer used for the current period's rate

#### Scenario: Monotonic period
- **WHEN** all snapshots in the period are non-decreasing in usage
- **THEN** the full period history is used for the rate

### Requirement: Period boundary inference
The system SHALL infer each bucket's quota period boundaries from the observed expiry: the next boundary SHALL be the snapshot's fetch time plus its reset seconds. The previous boundary and window length SHALL be derived per bucket: for the rolling bucket the period SHALL be a fixed 5-hour window (previous boundary = next boundary minus 5 hours); for the weekly bucket a fixed 7-day window (previous boundary = next boundary minus 7 days); for the monthly bucket a calendar month anchored at the next boundary's day-of-month and time-of-day (previous boundary = that day-of-month and time-of-day in the previous calendar month, clamped to the previous month's last day when the day does not exist there). The window length SHALL be the difference between the next and previous boundaries. The window SHALL be reported as unavailable when it cannot be derived or when it is inconsistent (elapsed time plus remaining time differs from the window length beyond a tolerance).

#### Scenario: Rolling window
- **WHEN** the rolling bucket reports a reset time
- **THEN** the previous boundary is the next boundary minus 5 hours

#### Scenario: Weekly window
- **WHEN** the weekly bucket reports a reset time
- **THEN** the previous boundary is the next boundary minus 7 days

#### Scenario: Monthly same day-of-month
- **WHEN** the next monthly boundary is March 28 at 20:42 and the previous month is February of a non-leap year
- **THEN** the previous boundary is February 28 at 20:42

#### Scenario: Monthly clamp to short month
- **WHEN** the next monthly boundary is May 31 and the previous month is April
- **THEN** the previous boundary is April 30 at the same time-of-day

#### Scenario: Monthly clamp to February
- **WHEN** the next monthly boundary is March 31 in a non-leap year
- **THEN** the previous boundary is February 28 at the same time-of-day

#### Scenario: Leap year preserved
- **WHEN** the next monthly boundary is February 29 in a leap year
- **THEN** the previous boundary is January 29 at the same time-of-day

#### Scenario: Inconsistent window unavailable
- **WHEN** elapsed plus remaining differs from the inferred window length beyond the tolerance
- **THEN** the window is reported as unavailable

### Requirement: Position in period
The system SHALL compute each bucket's position within its current period as the elapsed time since the previous boundary divided by the window length, for the rolling, weekly, and monthly buckets alike, whenever the window is known. The position SHALL be reported as a neutral fact: the system SHALL NOT classify, warn on, or hide it. When the window is unknown, no position SHALL be reported.

#### Scenario: Position computed
- **WHEN** the weekly window is 7 days and the elapsed time since the previous boundary is 6 days 12 hours
- **THEN** the position is reported as 6 days of 7

#### Scenario: Position unavailable
- **WHEN** the window is unknown
- **THEN** no position is reported

#### Scenario: Rolling bucket included
- **WHEN** the rolling bucket has a known 5-hour window
- **THEN** a position within that window is reported for it

### Requirement: Projected usage at period end
The system SHALL compute a projected usage at period end for a bucket as the current usage multiplied by the ratio of the period window length to the time elapsed since the previous boundary at the snapshot's fetch time, whenever the period window is known and that elapsed time is positive. The projection SHALL be reported as a neutral value: the system SHALL NOT classify it, warn on it, or hide it. The projection SHALL NOT require consumption history; it SHALL be computed for the rolling, weekly, and monthly buckets alike.

#### Scenario: Projection computed
- **WHEN** a bucket has 36% usage, a 31-day window, and 11.5 days elapsed since the previous boundary
- **THEN** the projected usage at period end is reported as 97%

#### Scenario: No window, no projection
- **WHEN** the period window is unknown for a bucket
- **THEN** no projection is reported for it

#### Scenario: No projection at period start
- **WHEN** the elapsed time since the previous boundary is zero or negative
- **THEN** no projection is reported for the bucket

#### Scenario: Rolling bucket included
- **WHEN** the rolling bucket has a known window and positive elapsed time
- **THEN** a projection is reported for it
