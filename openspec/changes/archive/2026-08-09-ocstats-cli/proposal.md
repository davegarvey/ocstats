## Why

Checking the opencode Go (lite) subscription quota requires opening the opencode.ai website and clicking through to the workspace Go page. The quota data is only available server-side behind the website's session-authenticated server actions — there is no public API and the zen gateway only reports quota through 429 errors once a limit is hit. A small local CLI plus a menu bar integration gives instant, glanceable quota without touching a browser.

## What Changes

- New `ocstats` CLI (Go, single binary) that:
  - Logs in by reading the existing opencode.ai session cookie (`auth`) from the user's local browser profile (Firefox-first, plaintext cookie store) or a manual `--cookie` fallback, and stores it user-only (Keychain or 0600 file)
  - Fetches the Go subscription quota by calling the website's server action endpoint (`/_server/...`) and decodes rolling/weekly/monthly usage percentages and reset times
  - Renders a human-readable table by default, machine-readable `--json` for scripting, and a cached "last known values" fallback with a maximum staleness policy when the network is unavailable
- New SwiftBar plugin (shell script) that calls `ocstats --json` on a schedule and shows quota in the macOS menu bar (percentage in the bar, details in the dropdown)
- No hosted services: the CLI runs entirely on the user's machine, and the only network traffic it generates is to opencode.ai

## Capabilities

### New Capabilities
- `quota-fetch`: authenticating with opencode.ai (session capture from the local browser profile, user-only session storage), fetching the Go subscription quota from the server action, and parsing it into rolling/weekly/monthly usage buckets
- `quota-display`: the `ocstats` command line interface — default human output, `--json` output, distinct error exit codes, and stale-cache fallback behavior
- `menu-bar`: the SwiftBar plugin that surfaces quota in the macOS menu bar

### Modified Capabilities
- None (empty repository, no existing specs)

## Impact

- New Go module in this repository; no changes to the opencode codebase
- Reverse-engineered dependency on the opencode.ai Solid Start server-action protocol (`/_server/...` POST with session cookie) — the wire format is not officially documented and may change; the spec pins the behavior contract, not the wire format, and the fetch layer is isolated so protocol changes only touch one module
- Session capture from the local browser profile: Firefox `cookies.sqlite` is read directly (plaintext, via the system `sqlite3`); other browsers fall back to `--cookie` until supported
- macOS-only features: user-only session storage (Keychain or 0600 file), SwiftBar menu bar integration; the CLI itself is portable Go and could run elsewhere with a different session store
