## 1. Project Setup

- [x] 1.1 Initialize the Go module (`go mod init ocstats`) with a minimal CLI skeleton: `ocstats login` and `ocstats quota` subcommands using only the standard library, plus `--json`, `--workspace`, and `--cookie` flags parsed per the quota-display and quota-fetch specs
- [x] 1.2 Add a `Makefile` with `build`, `test`, and `install` targets (binary to `~/.local/bin` or `$(GOBIN)`, SwiftBar plugin to `~/Library/Application Support/SwiftBar/plugins/`)

## 2. Protocol Spike

- [x] 2.1 Probe the live site to discover the exact `_server` URL for the quota server action and the workspace-resolution action; record the URL scheme, request body shape, and response envelope in design.md under a "Spike findings" note
- [x] 2.2 Verify that the `auth` cookie alone authenticates the `_server` request — no extra headers — and record how a null subscription and an unexpected response (redirect, 500) appear on the wire
- [x] 2.3 Capture sanitized real response fixtures (usage percentages and reset seconds only; no cookie or personal data) into `testdata/` for use by the fetch unit tests

## 3. Session Module

- [x] 3.1 Implement Firefox profile discovery (`profiles.ini` parsing, default profile first) and cookie extraction from `cookies.sqlite` via `/usr/bin/sqlite3` (host-only `auth` cookie for `opencode.ai`), with clear errors when no profile or cookie is found
- [x] 3.2 Implement session storage: macOS Keychain via `/usr/bin/security` with a 0600-file fallback under `~/.config/ocstats/`; ensure the cookie value never reaches stdout/stderr/logs; unit-test the file-storage path (temp dirs) and verify file mode is 0600
- [x] 3.3 Implement `ocstats login`: extract → validate with one authenticated request → store; on validation failure, leave the existing stored session untouched; implement the `--cookie` manual fallback

## 4. Fetch Module

- [x] 4.1 Implement the authenticated `_server` POST client (base URL, cookie injection, timeout, non-2xx and redirect handling) returning a typed `QuotaSnapshot` (`usagePercent` int 0-100, `resetInSec` int, `status`, `subscription` flag, `workspaceId`, `fetchedAt`) per the design contract — this is the only module that knows the wire protocol
- [x] 4.2 Implement workspace resolution: automatic last-seen workspace plus `--workspace <id>` override, with an actionable error when neither is available
- [x] 4.3 Add unit tests with `httptest` + the `testdata/` fixtures: successful fetch, null subscription, 500, and a malformed response (decode failure → fetch error, not a crash)
- [x] 4.4 Implement the quota cache: read/write `~/.cache/ocstats/quota.json` with `fetchedAt`, exposing cached values flagged stale; compute the 24-hour maximum staleness threshold

## 5. CLI Commands

- [x] 5.1 Implement `ocstats quota` human output: three-bucket table with percentages and reset times, fetch time, stale indicator with data age, and the too-stale error state (cached values shown with prominent error + fetch error)
- [x] 5.2 Implement `--json` output: single object with buckets, `workspaceId`, `fetchedAt`, stale flag; `subscription: null` when no Go sub; error objects on stdout with machine-readable codes
- [x] 5.3 Wire exit codes per spec: 0 when data shown (fresh or stale within threshold), distinct codes for auth failure, fetch failure, too-stale, and usage errors; auth errors print the re-login instruction

## 6. Menu Bar Plugin

- [x] 6.1 Write the SwiftBar plugin script: runs `ocstats --json`, renders the rolling percentage in the menu bar with color state, dropdown with all three buckets + reset times, "Open website" item using `workspaceId`, and explicit stale/error menus (never an empty menu); contains no secrets
- [x] 6.2 Wire plugin installation into `make install` and document the SwiftBar plugin-directory requirement in the Makefile comments

## 7. Verification

- [x] 7.1 Run `go vet`, `gofmt`, and the unit tests; verify every spec scenario is covered by a test or a manual checklist entry
- [x] 7.2 End-to-end manual pass on macOS: `login` from Firefox, `quota` fresh, `--json` shape, kill network → stale output, age the cache past 24h → too-stale exit code, remove session → auth error, render the SwiftBar plugin and confirm dropdown + open-website
