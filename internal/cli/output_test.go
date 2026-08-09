package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"ocstats/internal/fetch"
)

func capture(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func render(s fetch.Snapshot, p paces, stale bool) string {
	return capture(func() { renderSnapshot(false, s, p, periodInfos{}, false, stale, "", ExitOK) })
}

func TestRenderNoSubscriptionHuman(t *testing.T) {
	s := fetch.Snapshot{WorkspaceID: "wrk_x", Subscription: false, FetchedAt: time.Now()}
	out := render(s, paces{}, false)
	if !strings.Contains(out, "No Go subscription") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRenderNoSubscriptionJSON(t *testing.T) {
	s := fetch.Snapshot{WorkspaceID: "wrk_x", Subscription: false, FetchedAt: time.Now()}
	out := capture(func() { renderSnapshot(true, s, paces{}, periodInfos{}, false, false, "", ExitOK) })
	if !strings.Contains(out, `"subscription":null`) {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRenderFreshHuman(t *testing.T) {
	s := fetch.Snapshot{
		WorkspaceID:  "wrk_x",
		Subscription: true,
		Rolling:      fetch.Bucket{Status: "ok", UsagePercent: 68, ResetInSec: 3600},
		Weekly:       fetch.Bucket{Status: "ok", UsagePercent: 41, ResetInSec: 7200},
		Monthly:      fetch.Bucket{Status: "ok", UsagePercent: 12, ResetInSec: 90000},
		FetchedAt:    time.Now(),
	}
	out := render(s, paces{}, false)
	for _, want := range []string{"Rolling", "68%", "Weekly", "41%", "Monthly", "12%", "resets in"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
	if strings.Contains(out, "STALE") {
		t.Fatalf("unexpected stale marker: %q", out)
	}
}

func TestRenderStaleJSON(t *testing.T) {
	s := fetch.Snapshot{WorkspaceID: "wrk_x", Subscription: true, FetchedAt: time.Now()}
	out := capture(func() { renderSnapshot(true, s, paces{}, periodInfos{}, false, true, "", ExitOK) })
	if !strings.Contains(out, `"stale":true`) {
		t.Fatalf("missing stale flag: %q", out)
	}
}

func TestUsageBar(t *testing.T) {
	cases := []struct {
		pct     int
		filled  int
		emptied int
	}{
		{0, 0, 20},
		{3, 1, 19},
		{18, 4, 16},
		{54, 11, 9},
		{100, 20, 0},
	}
	for _, c := range cases {
		bar := usageBar(c.pct, nil, false)
		if got := strings.Count(bar, "█"); got != c.filled {
			t.Errorf("usageBar(%d) filled = %d, want %d (bar %q)", c.pct, got, c.filled, bar)
		}
		if got := strings.Count(bar, "░"); got != c.emptied {
			t.Errorf("usageBar(%d) emptied = %d, want %d (bar %q)", c.pct, got, c.emptied, bar)
		}
	}
}

func TestNoANSIWhenPiped(t *testing.T) {
	s := fetch.Snapshot{
		WorkspaceID: "wrk_x", Subscription: true,
		Rolling:   fetch.Bucket{UsagePercent: 90},
		Weekly:    fetch.Bucket{UsagePercent: 60},
		Monthly:   fetch.Bucket{UsagePercent: 5},
		FetchedAt: time.Now(),
	}
	out := capture(func() { renderSnapshot(false, s, paces{}, periodInfos{}, false, false, "", ExitOK) })
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("ANSI escapes leaked into piped output: %q", out)
	}
}

func TestColorOnTerminal(t *testing.T) {
	s := fetch.Snapshot{
		WorkspaceID: "wrk_x", Subscription: true,
		Rolling:   fetch.Bucket{UsagePercent: 90},
		Weekly:    fetch.Bucket{UsagePercent: 60},
		Monthly:   fetch.Bucket{UsagePercent: 5},
		FetchedAt: time.Now(),
	}
	out := capture(func() { renderSnapshot(false, s, paces{}, periodInfos{}, true, false, "", ExitOK) })
	if !strings.Contains(out, "\x1b[31m") {
		t.Fatalf("missing red for 90%%: %q", out)
	}
	if !strings.Contains(out, "\x1b[33m") {
		t.Fatalf("missing yellow for 60%%: %q", out)
	}
	if !strings.Contains(out, "\x1b[32m") {
		t.Fatalf("missing green for 5%%: %q", out)
	}
}

func TestProjectionShown(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC)
	s := fetch.Snapshot{
		WorkspaceID: "wrk_x", Subscription: true,
		Rolling:   fetch.Bucket{UsagePercent: 6, ResetInSec: 11807},
		Weekly:    fetch.Bucket{UsagePercent: 55, ResetInSec: 39000},
		Monthly:   fetch.Bucket{UsagePercent: 36, ResetInSec: 1672561},
		FetchedAt: now,
	}
	var periods periodInfos
	periods.Rolling = computePeriod(s, bucketRolling, now)
	periods.Weekly = computePeriod(s, bucketWeekly, now)
	periods.Monthly = computePeriod(s, bucketMonthly, now)
	out := capture(func() { renderSnapshot(false, s, paces{}, periods, false, false, "", ExitOK) })
	for _, want := range []string{"→ 17% by reset", "→ 59% by reset", "→ 96% by reset"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing projection %q in %q", want, out)
		}
	}
}

func TestProjectionHiddenWithoutWindow(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC)
	s := fetch.Snapshot{
		WorkspaceID: "wrk_x", Subscription: true,
		Rolling:   fetch.Bucket{UsagePercent: 3, ResetInSec: 3600},
		Weekly:    fetch.Bucket{UsagePercent: 54, ResetInSec: 42800},
		Monthly:   fetch.Bucket{UsagePercent: 35, ResetInSec: 90000},
		FetchedAt: now.Add(-30 * time.Hour),
	}
	out := render(s, paces{}, false)
	if strings.Contains(out, "→") {
		t.Fatalf("projection must be silent without a window: %q", out)
	}
}

func TestBarPositionMarker(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC)
	s := fetch.Snapshot{
		WorkspaceID: "wrk_x", Subscription: true,
		Rolling:   fetch.Bucket{UsagePercent: 6, ResetInSec: 11807},
		Weekly:    fetch.Bucket{UsagePercent: 55, ResetInSec: 39000},
		Monthly:   fetch.Bucket{UsagePercent: 36, ResetInSec: 1672561},
		FetchedAt: now,
	}
	var periods periodInfos
	periods.Rolling = computePeriod(s, bucketRolling, now)
	periods.Weekly = computePeriod(s, bucketWeekly, now)
	periods.Monthly = computePeriod(s, bucketMonthly, now)
	out := capture(func() { renderSnapshot(false, s, paces{}, periods, false, false, "", ExitOK) })
	lines := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		for _, name := range []string{"Rolling", "Weekly", "Monthly"} {
			if strings.HasPrefix(line, "  "+name) {
				lines[name] = line
			}
		}
	}
	for name, line := range lines {
		runes := []rune(line)
		barStart := -1
		for i, r := range runes {
			if r == '█' || r == '░' {
				barStart = i
				break
			}
		}
		if barStart < 0 {
			t.Fatalf("no bar found in %s line: %q", name, line)
		}
		bar := string(runes[barStart : barStart+20])
		if !strings.Contains(bar, "│") {
			t.Fatalf("%s bar missing position marker: %q (bar %q)", name, line, bar)
		}
	}
}

func TestBarNoMarkerWhenUnknown(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC)
	s := fetch.Snapshot{
		WorkspaceID: "wrk_x", Subscription: true,
		Rolling:   fetch.Bucket{UsagePercent: 4, ResetInSec: 100},
		Weekly:    fetch.Bucket{UsagePercent: 54, ResetInSec: 100},
		Monthly:   fetch.Bucket{UsagePercent: 35, ResetInSec: 100},
		FetchedAt: now.Add(-30 * time.Hour),
	}
	out := render(s, paces{}, false)
	if strings.Contains(out, "│") {
		t.Fatalf("bar must have no marker when windows are unknown: %q", out)
	}
}

func TestUsageBarMarkerColumn(t *testing.T) {
	mid := 10
	bar := usageBar(50, &mid, false)
	if got := runeIndex(bar, '│'); got != 10 {
		t.Fatalf("marker column = %d, want 10: %q", got, bar)
	}
	bar = usageBar(50, nil, false)
	if strings.Contains(bar, "│") {
		t.Fatalf("nil marker must render no marker: %q", bar)
	}
	edge := 19
	bar = usageBar(100, &edge, false)
	if got := runeIndex(bar, '│'); got != 19 {
		t.Fatalf("marker at last cell = %d, want 19: %q", got, bar)
	}
}

func runeIndex(s string, want rune) int {
	for i, r := range []rune(s) {
		if r == want {
			return i
		}
	}
	return -1
}

func TestJSONRatePairPresent(t *testing.T) {
	s := fetch.Snapshot{
		WorkspaceID: "wrk_x", Subscription: true,
		Rolling:   fetch.Bucket{Status: "ok", UsagePercent: 3, ResetInSec: 100},
		Weekly:    fetch.Bucket{Status: "ok", UsagePercent: 54, ResetInSec: 200},
		Monthly:   fetch.Bucket{Status: "ok", UsagePercent: 35, ResetInSec: 300},
		FetchedAt: time.Now(),
	}
	var p paces
	p.Weekly = bucketPace{Info: paceInfo{HasRate: true, RatePctPerHour: 2.5, RateSpanSec: 7200}}
	out := capture(func() { renderSnapshot(true, s, p, periodInfos{}, false, false, "", ExitOK) })
	var parsed struct {
		Rolling jsonBucket `json:"rolling"`
		Weekly  jsonBucket `json:"weekly"`
		Monthly jsonBucket `json:"monthly"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Rolling.RatePctPerHour != nil || parsed.Rolling.RateSpanSec != nil {
		t.Fatalf("rolling rate pair must be null without a rate: %+v", parsed.Rolling)
	}
	if parsed.Weekly.RatePctPerHour == nil || parsed.Weekly.RateSpanSec == nil {
		t.Fatalf("weekly rate pair must be present: %+v", parsed.Weekly)
	}
	if *parsed.Weekly.RatePctPerHour != 2.5 || *parsed.Weekly.RateSpanSec != 7200 {
		t.Fatalf("weekly rate pair wrong: %+v", parsed.Weekly)
	}
	if parsed.Monthly.RatePctPerHour != nil || parsed.Monthly.RateSpanSec != nil {
		t.Fatalf("monthly rate pair must be null without a rate: %+v", parsed.Monthly)
	}
}

func TestJSONRatePairNull(t *testing.T) {
	s := fetch.Snapshot{
		WorkspaceID: "wrk_x", Subscription: true,
		Rolling:   fetch.Bucket{Status: "ok", UsagePercent: 3, ResetInSec: 100},
		Weekly:    fetch.Bucket{Status: "ok", UsagePercent: 54, ResetInSec: 200},
		Monthly:   fetch.Bucket{Status: "ok", UsagePercent: 35, ResetInSec: 300},
		FetchedAt: time.Now(),
	}
	out := capture(func() { renderSnapshot(true, s, paces{}, periodInfos{}, false, false, "", ExitOK) })
	if !strings.Contains(out, `"ratePctPerHour":null`) {
		t.Fatalf("expected null rate fields: %q", out)
	}
	if strings.Contains(out, "by reset") || strings.Contains(out, "projected") {
		t.Fatalf("JSON must not contain projections or classifications: %q", out)
	}
}

func TestPositionShown(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC)
	s := fetch.Snapshot{
		WorkspaceID: "wrk_x", Subscription: true,
		Rolling:   fetch.Bucket{UsagePercent: 4, ResetInSec: 11807},
		Weekly:    fetch.Bucket{UsagePercent: 54, ResetInSec: 39000},
		Monthly:   fetch.Bucket{UsagePercent: 35, ResetInSec: 1672561},
		FetchedAt: now,
	}
	var periods periodInfos
	periods.Rolling = computePeriod(s, bucketRolling, now)
	periods.Weekly = computePeriod(s, bucketWeekly, now)
	periods.Monthly = computePeriod(s, bucketMonthly, now)
	out := capture(func() { renderSnapshot(false, s, paces{}, periods, false, false, "", ExitOK) })
	for _, want := range []string{"1h 43m of 5h", "day 6 of 7", "day 11 of 31"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing position marker %q in %q", want, out)
		}
	}
}

func TestPositionHiddenWhenUnknown(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC)
	s := fetch.Snapshot{
		WorkspaceID: "wrk_x", Subscription: true,
		Rolling:   fetch.Bucket{UsagePercent: 4, ResetInSec: 100},
		Weekly:    fetch.Bucket{UsagePercent: 54, ResetInSec: 100},
		Monthly:   fetch.Bucket{UsagePercent: 35, ResetInSec: 100},
		FetchedAt: now.Add(-30 * time.Hour), // every period has ended
	}
	out := render(s, paces{}, false)
	if strings.Contains(out, "of 5h") || strings.Contains(out, "day ") {
		t.Fatalf("position must be hidden when windows are unknown: %q", out)
	}
}

func TestFormatPosition(t *testing.T) {
	weekly := periodInfo{Known: true, Window: 7 * 24 * time.Hour, Elapsed: 6*24*time.Hour + 12*time.Hour}
	if got := formatPosition(bucketWeekly, weekly); got != "day 6 of 7" {
		t.Fatalf("weekly position = %q, want %q", got, "day 6 of 7")
	}
	monthly := periodInfo{Known: true, Window: 30 * 24 * time.Hour, Elapsed: 10*24*time.Hour + 5*time.Hour}
	if got := formatPosition(bucketMonthly, monthly); got != "day 10 of 30" {
		t.Fatalf("monthly position = %q, want %q", got, "day 10 of 30")
	}
	rolling := periodInfo{Known: true, Window: 5 * time.Hour, Elapsed: time.Hour + 44*time.Minute}
	if got := formatPosition(bucketRolling, rolling); got != "1h 44m of 5h" {
		t.Fatalf("rolling position = %q, want %q", got, "1h 44m of 5h")
	}
}

func TestJSONPeriodLength(t *testing.T) {
	s := fetch.Snapshot{
		WorkspaceID: "wrk_x", Subscription: true,
		Rolling:   fetch.Bucket{Status: "ok", UsagePercent: 4, ResetInSec: 100},
		Weekly:    fetch.Bucket{Status: "ok", UsagePercent: 54, ResetInSec: 200},
		Monthly:   fetch.Bucket{Status: "ok", UsagePercent: 35, ResetInSec: 300},
		FetchedAt: time.Now(),
	}
	now := s.FetchedAt
	var periods periodInfos
	periods.Rolling = computePeriod(s, bucketRolling, now)
	periods.Weekly = computePeriod(s, bucketWeekly, now)
	periods.Monthly = computePeriod(s, bucketMonthly, now)
	out := capture(func() { renderSnapshot(true, s, paces{}, periods, false, false, "", ExitOK) })
	var parsed struct {
		Rolling jsonBucket `json:"rolling"`
		Weekly  jsonBucket `json:"weekly"`
		Monthly jsonBucket `json:"monthly"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Rolling.PeriodLengthSec == nil || *parsed.Rolling.PeriodLengthSec != 5*3600 {
		t.Fatalf("rolling periodLengthSec wrong: %+v", parsed.Rolling)
	}
	if parsed.Weekly.PeriodLengthSec == nil || *parsed.Weekly.PeriodLengthSec != 7*24*3600 {
		t.Fatalf("weekly periodLengthSec wrong: %+v", parsed.Weekly)
	}
	if parsed.Monthly.PeriodLengthSec == nil {
		t.Fatalf("monthly periodLengthSec missing: %+v", parsed.Monthly)
	}
	if strings.Contains(out, "of 5h") || strings.Contains(out, "day ") {
		t.Fatalf("JSON must not contain rendered positions: %q", out)
	}
}

func TestJSONPeriodLengthNull(t *testing.T) {
	s := fetch.Snapshot{
		WorkspaceID: "wrk_x", Subscription: true,
		Rolling:   fetch.Bucket{Status: "ok", UsagePercent: 4, ResetInSec: 100},
		Weekly:    fetch.Bucket{Status: "ok", UsagePercent: 54, ResetInSec: 200},
		Monthly:   fetch.Bucket{Status: "ok", UsagePercent: 35, ResetInSec: 300},
		FetchedAt: time.Now().Add(-30 * time.Hour),
	}
	out := capture(func() { renderSnapshot(true, s, paces{}, periodInfos{}, false, false, "", ExitOK) })
	if !strings.Contains(out, `"periodLengthSec":null`) {
		t.Fatalf("expected null periodLengthSec fields: %q", out)
	}
}
