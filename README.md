# ocstats

### Your opencode Go quota, without the browser

opencode.ai publishes its Go-quota figures only inside a browser session. There is no public API; the numbers sit behind session-authenticated server actions, and the gateway reports a limit only once you have hit it. ocstats reads the figures directly, and keeps reading them, so the answer to "will I run out?" stops requiring a trip to the website.

It reports three buckets — rolling five hours, weekly, monthly — with usage, time to reset, position in the period, and a projection of where the current burn rate ends up by the reset. The projection is a straight-line extrapolation: usage so far, stretched to the end of the period. Crude, but honest.

## Install

Requires Go 1.26.

```sh
make install
```

This builds the binary into `~/.local/bin` and installs the menu-bar plugin (see below). `PREFIX` overrides the install prefix.

## Login

```sh
ocstats login
```

The tool reads your existing opencode.ai session cookie from Firefox — `profiles.ini` and `cookies.sqlite`, copied to a temp file and queried with the system `sqlite3`. No passwords, no OAuth, no pasting. On other browsers (Chrome, Safari), hand it the cookie once:

```sh
ocstats login --cookie <value>
```

Find it in the developer tools of a logged-in browser: Application → Cookies → opencode.ai → `auth`.

The session is validated before it is stored. Storage is a `0600` file in your config directory. If the site rotates the session on any fetch, the tool captures the new cookie and re-saves it; a login typically lasts until the year-long cookie expires or the session is revoked server-side. There is no `logout`; delete the `session` file in the config directory to forget it.

## Usage

```sh
ocstats quota                 # human-readable
ocstats quota --json          # for scripts and menu bars
ocstats quota --workspace <id>
```

The workspace is resolved automatically from the account's last-seen state; `--workspace` overrides it.

Exit codes are for scripting:

| Code | Meaning |
|------|---------|
| 0    | Data shown (fresh, or cached but less than 24 hours old) |
| 2    | Authentication required; run `ocstats login` |
| 3    | Fetch failed and no cache exists |
| 4    | Fetch failed and the cache is older than 24 hours |
| 64   | Usage error |

## The menu bar

`make install` also drops a plugin into SwiftBar (a free menu-bar host), which renders the quota in the menu bar and refreshes every 15 minutes. It is xbar-compatible. The menu lists all three buckets, the projection to reset, and a link straight to the quota page. The gauge stays calm until a projection overshoots the limit; then it sprouts an exclamation mark, and not a moment sooner.

## How it works

The tool calls the same server actions the website calls, with your session cookie, and decodes the responses. There is no HTML scraping, and requests are shaped like the site's own client, so there is nothing crawler-like about them. When opencode.ai changes its front end, the action IDs in `internal/fetch/fetch.go` are the first thing to check.

## Offline behaviour

Every successful fetch is cached. If the network fails, the last snapshot is shown with a `STALE` warning rather than nothing; beyond 24 hours it gives up. History (31 days, 2,000 snapshots) is retained and exposed as a per-bucket consumption rate in the JSON output; the displayed projection does not use it.

## Security

The tool holds a credential: your session cookie. It is stored at `0600`, is never logged, and is sent only to opencode.ai. Firefox's cookie store is copied before being read, never modified. The menu-bar plugin contains no secrets; it only calls `ocstats quota --json`.

## Development

```sh
make test
make vet
make fmt
```

Tests cover session capture from a real SQLite store, seroval decoding, and session rotation, among other things.
