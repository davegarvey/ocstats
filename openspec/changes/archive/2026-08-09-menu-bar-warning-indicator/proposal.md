## Why

The menu-bar indicator changed its icon on usage thresholds (90% of any bucket), a behaviour no spec ever promised and one that cried wolf while a projection still pointed under the limit. The signal that actually matters is the projection: running out before the reset. The indicator should warn only then.

## What Changes

- The SwiftBar plugin shows a warning icon (`exclamationmark.triangle`) when any bucket's projected usage at period end reaches or exceeds 100%.
- Otherwise the indicator shows the plain gauge icon.
- No icon change occurs on raw usage thresholds.
- The warning condition is computed by the plugin itself from the existing JSON fields (`usagePercent`, `periodLengthSec`, `resetInSec`), consistent with how projections are already derived.
- README updated to describe the warning behaviour.

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `menu-bar`: the compact indicator gains a defined warning state; previously unspecified icon behaviour is pinned to the projection, not thresholds.

## Impact

- `scripts/ocstats.swiftbar.sh`: icon selection logic now derived from projected usage; the exclamation swap previously gated on `usagePercent >= 90` is removed.
- `README.md`: menu-bar section wording.
- No changes to the CLI, its JSON output, or the session/fetch layers.
