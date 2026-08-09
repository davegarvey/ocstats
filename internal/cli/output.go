package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"ocstats/internal/fetch"
)

type jsonBucket struct {
	Status          string   `json:"status"`
	UsagePercent    int      `json:"usagePercent"`
	ResetInSec      int      `json:"resetInSec"`
	RatePctPerHour  *float64 `json:"ratePctPerHour"`
	RateSpanSec     *int64   `json:"rateSpanSec"`
	PeriodLengthSec *int64   `json:"periodLengthSec"`
}

type jsonOutput struct {
	WorkspaceID  string      `json:"workspaceId"`
	Subscription *jsonBucket `json:"subscription"`
	Rolling      jsonBucket  `json:"rolling"`
	Weekly       jsonBucket  `json:"weekly"`
	Monthly      jsonBucket  `json:"monthly"`
	FetchedAt    string      `json:"fetchedAt"`
	Stale        bool        `json:"stale"`
}

func renderSnapshot(jsonOut bool, s fetch.Snapshot, p paces, periods periodInfos, useColor bool, stale bool, staleNote string, exit int) int {
	if jsonOut {
		out := jsonOutput{
			WorkspaceID: s.WorkspaceID,
			Rolling:     toJSONBucket(s.Rolling, p.Rolling, periods.Rolling),
			Weekly:      toJSONBucket(s.Weekly, p.Weekly, periods.Weekly),
			Monthly:     toJSONBucket(s.Monthly, p.Monthly, periods.Monthly),
			FetchedAt:   s.FetchedAt.Format(time.RFC3339),
			Stale:       stale,
		}
		if !s.Subscription {
			out.Subscription = nil
		} else {
			out.Subscription = &jsonBucket{Status: "subscribed"}
		}
		b, err := json.Marshal(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ocstats: encode output:", err)
			return ExitGeneric
		}
		fmt.Println(string(b))
		if staleNote != "" {
			fmt.Fprintln(os.Stderr, "ocstats:", staleNote)
		}
		return exit
	}

	if !s.Subscription {
		fmt.Printf("No Go subscription for workspace %s (fetched %s).\n", s.WorkspaceID, formatTime(s.FetchedAt))
		if stale {
			fmt.Printf("Stale data (fetch failed, shown from cache): %s old.\n", formatAge(time.Since(s.FetchedAt)))
			if staleNote != "" {
				fmt.Println(staleNote)
			}
		}
		return exit
	}

	fmt.Printf("Go quota for workspace %s (fetched %s)\n", s.WorkspaceID, formatTime(s.FetchedAt))
	renderBucket("Rolling", s.Rolling, p.Rolling, periods.Rolling, bucketRolling, useColor)
	renderBucket("Weekly", s.Weekly, p.Weekly, periods.Weekly, bucketWeekly, useColor)
	renderBucket("Monthly", s.Monthly, p.Monthly, periods.Monthly, bucketMonthly, useColor)
	if stale {
		fmt.Printf("STALE: fetch failed, showing cached data %s old.\n", formatAge(time.Since(s.FetchedAt)))
		if staleNote != "" {
			fmt.Println("ERROR:", staleNote)
		}
	}
	return exit
}

func renderBucket(name string, b fetch.Bucket, bp bucketPace, per periodInfo, k bucketKind, useColor bool) {
	var marker *int
	if per.Known && per.Elapsed >= 0 {
		col := int(per.Position*20 + 0.5)
		if col > 19 {
			col = 19
		}
		marker = &col
	}
	line := fmt.Sprintf("  %-7s %s %3d%%  resets in %s", name, usageBar(b.UsagePercent, marker, useColor), b.UsagePercent, formatAge(time.Duration(b.ResetInSec)*time.Second))
	if per.Projected != nil {
		line += fmt.Sprintf("   → %d%% by reset", *per.Projected)
	}
	if per.Known {
		line += "  " + formatPosition(k, per)
	}
	fmt.Println(line)
}

// formatPosition renders the neutral position within the current period:
// hour-based for the 5-hour rolling window, whole days for weekly and
// monthly windows.
func formatPosition(k bucketKind, per periodInfo) string {
	if k == bucketRolling {
		return fmt.Sprintf("%s of 5h", formatAge(per.Elapsed))
	}
	days := int(per.Elapsed.Hours() / 24)
	if days < 1 {
		days = 1
	}
	total := int(per.Window.Hours()/24 + 0.5)
	if days > total {
		days = total
	}
	return fmt.Sprintf("day %d of %d", days, total)
}

// usageBar renders a fixed 20-column bar filled in proportion to the usage
// percentage, rounded to the nearest percent. Empty bar = 0%, full = 100%.
// When marker is non-nil it replaces the cell at that column (0-19) with the
// period-position marker.
func usageBar(pct int, marker *int, color bool) string {
	filled := (2*pct + 5) / 10
	if filled < 0 {
		filled = 0
	}
	if filled > 20 {
		filled = 20
	}
	cells := make([]rune, 20)
	for i := range cells {
		cells[i] = '░'
	}
	for i := 0; i < filled; i++ {
		cells[i] = '█'
	}
	if marker != nil && *marker >= 0 && *marker < 20 {
		cells[*marker] = '│'
	}
	bar := string(cells)
	if !color {
		return bar
	}
	switch {
	case pct >= 90:
		return "\x1b[31m" + bar + "\x1b[0m"
	case pct >= 60:
		return "\x1b[33m" + bar + "\x1b[0m"
	default:
		return "\x1b[32m" + bar + "\x1b[0m"
	}
}

func toJSONBucket(b fetch.Bucket, bp bucketPace, per periodInfo) jsonBucket {
	jb := jsonBucket{
		Status:       b.Status,
		UsagePercent: b.UsagePercent,
		ResetInSec:   b.ResetInSec,
	}
	if bp.Info.HasRate {
		rate := bp.Info.RatePctPerHour
		span := bp.Info.RateSpanSec
		jb.RatePctPerHour = &rate
		jb.RateSpanSec = &span
	}
	if per.Known {
		length := int64(per.Window.Seconds())
		jb.PeriodLengthSec = &length
	}
	return jb
}

func formatTime(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04")
}

func formatAge(d time.Duration) string {
	secs := int(d.Seconds())
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	mins := secs / 60
	if mins < 60 {
		return fmt.Sprintf("%dm", mins)
	}
	hours := mins / 60
	if hours < 24 {
		return fmt.Sprintf("%dh %dm", hours, mins%60)
	}
	days := hours / 24
	return fmt.Sprintf("%dd %dh", days, hours%24)
}
