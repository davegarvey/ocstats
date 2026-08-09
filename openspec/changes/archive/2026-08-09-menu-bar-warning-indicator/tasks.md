## 1. Plugin Implementation

- [x] 1.1 Refactor the projection computation in `scripts/ocstats.swiftbar.sh` into a reusable helper that returns a numeric projection (or none)
- [x] 1.2 Select the indicator icon: warning (`exclamationmark.triangle`) when any bucket's projection is 100 or greater, else `gauge`
- [x] 1.3 Keep dropdown projection lines rendering from the same helper

## 2. Docs & Verification

- [x] 2.1 Update README menu-bar paragraph to describe the projection-based warning
- [x] 2.2 Verify icon selection with both an overshoot case (projection >= 100) and a calm case (projection < 100), and confirm no threshold-based changes remain
- [x] 2.3 `make plugin` installs the updated script cleanly
