## Purpose

Estimates how fast Go quota is being consumed and shows what usage at period end would be if the current pace holds, so the user can interpret for themselves whether they will hit the limit.

## ADDED Requirements

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

### Requirement: Usage projection
The system SHALL compute a projected usage at period end for a bucket as the current usage plus the rate multiplied by the time remaining until reset (converted from seconds to hours), whenever a rate is available. The projection SHALL be reported as a neutral value: the system SHALL NOT classify it, warn on it, or hide it. The projection SHALL NOT be computed for the rolling bucket, which is a sliding window without a fixed period end.

#### Scenario: Projection computed
- **WHEN** a bucket has 54% usage, a rate of 2 percentage points per hour, and 42800 seconds remaining
- **THEN** the projected usage at period end is reported as 78%

#### Scenario: No rate, no projection
- **WHEN** no rate is available for a bucket
- **THEN** no projection is reported for it

#### Scenario: Rolling bucket excluded
- **WHEN** the rolling bucket has a rate available
- **THEN** no projection is reported for it
