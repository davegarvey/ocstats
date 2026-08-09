## Context

The opencode.ai console (Solid Start app at github.com/anomalyco/opencode, `packages/console`) has no public quota API. Investigation of the source confirmed:

- Go ("lite") quota is computed server-side; the web page `workspace/[id]/go` surfaces `{status, usagePercent, resetInSec}` per bucket via a server action, and limits are never exposed to clients.
- The website exposes data through Solid Start server actions (`/_server/...` POSTs) authenticated by a session cookie named `auth` (signed server-side, `httpOnly`, `secure: false`, 365-day maxAge — `context/auth.ts:28-38`). The zen gateway (`/zen/go/v1/*`) authenticates with the `opencode-go` API key but only reports quota via 429 + `retry-after` once a limit is hit — never usage percentages.
- **The site's GitHub OAuth flow cannot deliver the session cookie to a CLI**: `auth/authorize.ts` builds the OAuth redirect URI with `new URL('./callback' + cont, input.request.url)` — always on opencode.ai's own origin — and `auth/[...callback].ts` exchanges the code server-side, sets the cookie on opencode.ai's response, and redirects the browser to an internal page. A localhost listener never receives the code or the cookie (httpOnly). Red-team review verified this; the "gh-style" login flow is infeasible for this site.
- Firefox stores cookies in plaintext SQLite (`cookies.sqlite`, `moz_cookies` table), readable with `/usr/bin/sqlite3` (ships with macOS) — no Keychain decryption needed, unlike Chrome.
- The repo is a fresh empty repository (only OpenSpec scaffold), so this is a greenfield Go CLI + plugin.

## Goals / Non-Goals

**Goals:**
- Exact rolling/weekly/monthly percentages + reset times, same numbers the website shows
- All-local operation: CLI on the user's machine, session stored user-only, no hosted service
- Zero-re-login for the user: the tool consumes their existing opencode.ai login from their browser
- Isolate the reverse-engineered wire protocol behind one module so the rest of the tool never depends on it

**Non-Goals:**
- A native SwiftUI menu bar app (SwiftBar plugin covers the need)
- Quota for other plans (black/subscription/balance) — Go sub only, though the JSON shape leaves room
- Usage history/trending — snapshots only
- Browser support beyond Firefox at launch (Chrome via `kooky`-style decryption is future work; manual `--cookie` covers the rest)

## Decisions

### 1. Login reads the session cookie from the local browser profile — not OAuth
The OAuth flow is structurally incapable of handing the cookie to the CLI (see Context). Instead, `ocstats login`:

1. Locates the Firefox profile via `profiles.ini` (`~/Library/Application Support/Firefox/Profiles/`, default profile first), opens `cookies.sqlite` (copying it first — Firefox locks the live file — and replaying the WAL tail) with `/usr/bin/sqlite3` (zero Go deps, no CGO), and selects the host-only `auth` cookie for `opencode.ai` (Firefox's schema uses a `host` column, not `baseDomain`).
2. Validates the cookie with one authenticated request to `GET /auth/status` (a stable API route returning the session data; see Spike Findings) — if it fails, the login fails with an actionable message and the previous stored session is left untouched.
3. Stores the cookie in a 0600 file under the user config dir (`os.UserConfigDir()`, i.e. `~/Library/Application Support/ocstats/session` on macOS), written atomically with a 0700 parent directory. The Keychain route was dropped: the `security` CLI takes the password as a process argument, leaking the cookie into `argv` (visible to other local processes), while the 0600 file provides the same user boundary with no such exposure. The cookie is a 365-day full-account session — treat it as a password-grade secret.
4. `--cookie <value>` provides the manual fallback for unsupported browsers.

*Alternatives considered:* (a) GitHub OAuth via localhost callback — infeasible, redirect URI is site-bound and the exchange is server-side (verified in source); (b) CDP-launched throwaway browser — forces GitHub re-login and pulls a heavyweight dep; (c) Chrome profile decryption (`kooky`) — Chrome's cookie store needs Keychain-decrypted keys and schema-version handling; deferred until Firefox path proves out.

### 2. Isolate the wire protocol in a `fetch` module with a stable internal contract
The `_server` endpoint URL/body scheme is not officially documented and may change with Solid Start versions. The fetch module is the ONLY place that knows about `/_server/...`; everything else consumes a typed `QuotaSnapshot`:

```
bucket { usagePercent: int (0-100), resetInSec: int, status: ok | rate-limited }
snapshot { workspaceId, rolling: bucket, weekly: bucket, monthly: bucket,
           subscription: bool, fetchedAt }
```

Note `limit` was dropped from the contract: the server action only returns percentages + reset times (limits live in a private env var). Protocol details to pin down in a spike before implementation:

- Exact `_server` URL for the quota server action (Solid Start compiles `use server` fns to hashed routes) — resolved in the spike; see Spike Findings
- Request body shape (JSON-serialized args) and response envelope
- Whether the `auth` cookie alone suffices or extra headers (`x-solidstart-*`) are required
- Workspace resolution endpoint (`getLastSeenWorkspaceID`, session-based — verified in `routes/workspace/common.tsx`)

### 3. Cache: local JSON file with a maximum staleness policy
The CLI writes the last successful `QuotaSnapshot` + timestamp to `~/.cache/ocstats/quota.json`. On fetch failure the CLI returns the cached snapshot with `stale: true`; cached data older than 24h is a hard error state (distinct exit code, prominent message) so a silently broken protocol can never masquerade as fresh data forever. No daemon, no persisted connection.

### 4. Menu bar via SwiftBar plugin, not a native app
A SwiftBar plugin is a single executable script in `~/Library/Application Support/SwiftBar/plugins/` that runs `ocstats --json` on SwiftBar's schedule (e.g. every 15 min) and emits SwiftBar-format lines (percentage with `color=`/`sfimage=`, dropdown with buckets + reset times, "Open website" item using the `workspaceId` the CLI reports in JSON). The plugin contains zero secrets — the CLI owns the session, satisfying the spec's no-shared-secrets requirement. Distribution: `make install` copies the plugin into the SwiftBar plugins dir.

*Alternatives considered:* native SwiftUI `MenuBarExtra` app (nicer, but needs Xcode + app bundle + distribution; SwiftBar gives the same UX for a fraction of the work).

### 5. Single Go binary with standard-library HTTP + minimal deps
Go stdlib `net/http` for fetch calls; `/usr/bin/sqlite3` for cookie reads; `/usr/bin/security` for Keychain storage; JSON via stdlib. No heavy framework; testable fetch/parse logic with `httptest` and fixture responses.

## Risks / Trade-offs

- [Server-action protocol is undocumented and may change] → Mitigation: protocol isolated in one module; the `QuotaSnapshot` contract is the seam; plus the 24h max-staleness policy surfaces breakage loudly instead of silently showing stale data.
- [Session cookie expiry (365 days) or site-side revocation] → Mitigation: auth-error exit code tells the user to re-run `ocstats login`; GitHub session revocation does NOT invalidate the console cookie (server-signed, 365d) — expiry is only detectable on the next failed fetch, which is acceptable.
- [Stolen session cookie = full opencode.ai account access for up to a year] → Mitigation: stored in a 0600 user-only file, never printed; login validates before storing; the plugin never holds it.
- [Firefox profile layout / cookies.sqlite schema changes] → Mitigation: cookie read isolated behind a small `session` module; `profiles.ini`-based discovery with clear "profile not found" errors; `--cookie` fallback covers anything exotic.
- [Web-site session write side effects] → Mitigation: workspace resolution uses the session's last-seen state (reads only); no browser-side state is mutated by the CLI.
- [ToS/ethics] → Automated use of undocumented server actions with the user's own session is fragile and unapproved surface; acceptable as a personal, low-frequency tool (~96 requests/day at 15-min polling), must not become load-generating or share the session.

## Open Questions

- The exact `_server` URL scheme and request envelope — resolved by the spike task before fetch implementation; may add protocol notes to the design when discovered.
- Whether `getLastSeenWorkspaceID`'s response shape differs from `queryLiteSubscription`'s envelope (both are server actions; spike covers both).

## Spike Findings (2026-08-09, live verification against opencode.ai)

Protocol as deployed today:

- **Invocation (validated)**: the site's own client mode (GET with a per-request instance header) works; the deployed server rejects JSON POST bodies. See `internal/fetch/fetch.go`.
- **Response**: `text/javascript`; the value is recovered by a decoder isolated in `internal/fetch/decode.go`, covered by fixture-based tests.
- **Auth failure**: HTTP 200/302 with headers `location: /auth/authorize` and `x-error: true` (the serialized result may also be a 302 Response object). Detect via response headers; treat as auth error.
- **Session cookie**: `auth`, host-only on `opencode.ai`, path `/`, 365-day expiry, httpOnly.
- **Function IDs (current deploy)**: pinned in `internal/fetch/fetch.go`; the first thing to check when the site's front end changes.
- **Session validation**: `GET /auth/status` returns `{account:{...}, current:"<id>"}` when logged in, `{}` when not — a stable API route, simpler and more robust than a server action.
- **Firefox cookie store**: schema uses `host` (not `baseDomain`); the DB is locked while Firefox runs — copy `cookies.sqlite` (and `-wal` for freshness) to a temp file before querying. No Keychain encryption.
- **Not verified live**: null-subscription wire form (the user has a Go sub; the fixture in `testdata/null-subscription.txt` is synthetic, based on the source returning `null` from the quota action).
