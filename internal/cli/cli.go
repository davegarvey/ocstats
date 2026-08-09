package cli

import (
	"fmt"
	"os"
	"time"

	"ocstats/internal/fetch"
	"ocstats/internal/session"
)

func baseURL() string {
	if v := os.Getenv("OCSTATS_BASE_URL"); v != "" {
		return v
	}
	return fetch.BaseURL
}

const (
	ExitOK      = 0
	ExitGeneric = 1
	ExitAuth    = 2
	ExitFetch   = 3
	ExitStale   = 4
	ExitUsage   = 64
	MaxStaleAge = 24 * time.Hour
)

func Run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return ExitUsage
	}
	switch args[0] {
	case "login":
		return cmdLogin(args[1:])
	case "quota":
		return cmdQuota(args[1:])
	case "help", "-h", "--help":
		usage(os.Stdout)
		return ExitOK
	default:
		fmt.Fprintf(os.Stderr, "ocstats: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		return ExitUsage
	}
}

func usage(w *os.File) {
	fmt.Fprintln(w, `ocstats - opencode Go subscription quota

Usage:
  ocstats login [--cookie <value>]   capture the opencode.ai session from Firefox (or a provided cookie)
  ocstats quota [--json] [--workspace <id>]   show Go quota; exits 0 if data (fresh or stale) is shown
  ocstats help

Exit codes:
  0   quota data shown (fresh or stale within 24h)
  2   authentication required / session rejected
  3   fetch failed and no usable cache
  4   cached data older than 24h
  64  usage error`)
}

func cmdLogin(args []string) int {
	cookie := ""
	provided := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--cookie":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "ocstats login: --cookie requires a value")
				return ExitUsage
			}
			cookie = args[i+1]
			provided = true
			i++
		default:
			fmt.Fprintf(os.Stderr, "ocstats login: unknown flag %q\n", args[i])
			return ExitUsage
		}
	}
	if !provided {
		c, err := session.ReadFirefoxCookieFromDefaultProfile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ocstats login: %v\n\nLog in to https://opencode.ai in Firefox, or use --cookie.\n", err)
			return ExitAuth
		}
		cookie = c
	}
	store := session.DefaultStore()
	client := &fetch.Client{Base: baseURL(), Cookie: cookie}
	if err := client.ValidateSession(); err != nil {
		fmt.Fprintf(os.Stderr, "ocstats login: %v\n\nLog in to https://opencode.ai in your browser and retry.\n", err)
		return ExitAuth
	}
	if client.NewCookie != "" {
		cookie = client.NewCookie
	}
	if err := store.Save(cookie); err != nil {
		fmt.Fprintf(os.Stderr, "ocstats login: %v\n", err)
		return ExitGeneric
	}
	fmt.Fprintln(os.Stderr, "ocstats: session stored")
	return ExitOK
}

func cmdQuota(args []string) int {
	jsonOut := false
	workspace := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--workspace":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "ocstats quota: --workspace requires a value")
				return ExitUsage
			}
			workspace = args[i+1]
			i++
		default:
			fmt.Fprintf(os.Stderr, "ocstats quota: unknown flag %q\n", args[i])
			return ExitUsage
		}
	}

	store := session.DefaultStore()
	cookie, err := store.Load()
	if err != nil {
		return reportError(jsonOut, "auth", err.Error(), ExitAuth)
	}

	client := &fetch.Client{Base: baseURL(), Cookie: cookie}
	if workspace == "" {
		workspace, err = client.ResolveWorkspace()
		if err != nil {
			if fetch.IsKind(err, fetch.KindAuth) {
				return reportError(jsonOut, "auth", err.Error()+"\nRun \"ocstats login\" again.", ExitAuth)
			}
			return reportError(jsonOut, "no_workspace",
				err.Error()+"\nUse --workspace <id> to select one explicitly.", ExitAuth)
		}
	}

	q, ok, err := client.QueryQuota(workspace)
	if err != nil {
		if fetch.IsKind(err, fetch.KindAuth) {
			return reportError(jsonOut, "auth", err.Error()+"\nRun \"ocstats login\" again.", ExitAuth)
		}
		cache := defaultCache()
		cached, cachedOK := cache.latest(workspace)
		if !cachedOK {
			return reportError(jsonOut, "fetch", err.Error(), ExitFetch)
		}
		age := clock().Sub(cached.FetchedAt)
		pace := computePaces(cache.history(workspace), cached, age)
		periods := computePeriods(cached, clock())
		if age > MaxStaleAge {
			return renderSnapshot(jsonOut, cached, pace, periods, useColor(), true, fmt.Sprintf("cached data is %s old (limit %s); fetch error: %v", formatAge(age), MaxStaleAge, err), ExitStale)
		}
		return renderSnapshot(jsonOut, cached, pace, periods, useColor(), true, "", ExitOK)
	}
	snapshot := fetch.Snapshot{
		WorkspaceID:  workspace,
		Rolling:      q.RollingUsage,
		Weekly:       q.WeeklyUsage,
		Monthly:      q.MonthlyUsage,
		Subscription: ok,
		FetchedAt:    clock(),
	}
	cache := defaultCache()
	history, cerr := cache.append(workspace, snapshot, snapshot.FetchedAt)
	if cerr != nil {
		fmt.Fprintf(os.Stderr, "ocstats: warning: cache write failed: %v\n", cerr)
	}
	pace := computePaces(history, snapshot, 0)
	periods := computePeriods(snapshot, snapshot.FetchedAt)
	if client.NewCookie != "" {
		if cerr := store.Save(client.NewCookie); cerr != nil {
			fmt.Fprintf(os.Stderr, "ocstats: warning: session refresh failed: %v\n", cerr)
		}
	}
	return renderSnapshot(jsonOut, snapshot, pace, periods, useColor(), false, "", ExitOK)
}

func reportError(jsonOut bool, code string, msg string, exit int) int {
	if jsonOut {
		fmt.Printf(`{"error":{"code":%q,"message":%q}}`+"\n", code, msg)
	} else {
		fmt.Fprintln(os.Stderr, "ocstats:", msg)
	}
	return exit
}

// useColor reports whether human output should be color-coded: only when
// stdout is a character device (a terminal) and NO_COLOR is not set.
func useColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
