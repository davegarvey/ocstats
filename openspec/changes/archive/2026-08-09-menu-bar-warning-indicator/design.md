## Context

The SwiftBar plugin already derives projections from the JSON fields (`usagePercent`, `periodLengthSec`, `resetInSec`) and renders them in the dropdown. The indicator icon is currently a static `gauge` after threshold-based colouring was removed. See proposal.md - Why for motivation.

## Goals / Non-Goals

**Goals:**
- Warn only when a projection indicates exhaustion before the reset
- Compute the condition in the plugin, reusing the existing projection derivation, with no CLI or JSON changes

**Non-Goals:**
- No changes to the CLI, the JSON schema, or the session/fetch layers
- No thresholds, colours, or severity levels on the icon

## Decisions

**Compute the warning from the same math as the dropdown projections.** The plugin already has a `projected()` helper; reusing it guarantees the icon and the menu numbers can never disagree. Alternative: have the CLI emit a `warn` flag in JSON — rejected, since the plugin is required (by the existing menu-bar spec) to derive projections itself, and the CLI output is shared with other consumers.

**Aggregate with OR across the three buckets.** If any bucket is on pace to exceed its limit, the account is at risk; a single rolling-window overshoot (the 5-hour bucket) is enough to warn. The worst bucket drives the decision.

**Edge cases fall back to calm.** Buckets without `periodLengthSec`, or with non-positive elapsed time, yield no projection; the condition treats "no projection" as under the limit. The icon therefore cannot warn on unknown data, only on computed overshoot.

## Risks / Trade-offs

- [Projection noise early in a period] → The linear projection is already displayed in the dropdown; the icon inherits its behaviour, which is a single function's worth of coupling. Acceptable: the projection is the stated product.
- [`gauge`/`exclamationmark.triangle` are SF Symbols] → macOS-only by design; the plugin is macOS-only already (SwiftBar).

## Migration Plan

Replace the plugin file in place (`make plugin`); the next 15-minute refresh picks it up. Rollback is a one-line revert of the icon expression.
