package cli

import (
	"testing"
	"time"

	"ocstats/internal/fetch"
)

func hist(snaps ...fetch.Snapshot) []fetch.Snapshot { return snaps }

func TestRateOverObservedWindow(t *testing.T) {
	base := time.Now()
	h := hist(
		snap("wrk_x", true, 30, base),
		snap("wrk_x", true, 51, base.Add(2*time.Hour)),
		snap("wrk_x", true, 54, base.Add(4*time.Hour)),
	)
	p := estimateRate(h, bucketWeekly)
	if !p.HasRate {
		t.Fatal("expected a rate")
	}
	if p.RatePctPerHour != 6.0 {
		t.Fatalf("rate = %v, want 6 (window endpoints, not final segment)", p.RatePctPerHour)
	}
	if p.RateSpanSec != 4*3600 {
		t.Fatalf("span = %d, want %d", p.RateSpanSec, 4*3600)
	}
}

func TestRateInsufficientHistory(t *testing.T) {
	base := time.Now()
	if p := estimateRate(hist(snap("wrk_x", true, 30, base)), bucketWeekly); p.HasRate {
		t.Fatal("single snapshot must not yield a rate")
	}
	if p := estimateRate(nil, bucketWeekly); p.HasRate {
		t.Fatal("empty history must not yield a rate")
	}
}

func TestRateZeroDurationWindow(t *testing.T) {
	base := time.Now()
	h := hist(
		snap("wrk_x", true, 30, base),
		snap("wrk_x", true, 54, base),
	)
	p := estimateRate(h, bucketWeekly)
	if p.HasRate {
		t.Fatal("zero-duration window must yield no rate, never NaN")
	}
}

func TestRateClamp(t *testing.T) {
	if got := clampRate(-1.5); got != 0 {
		t.Fatalf("clampRate(-1.5) = %v, want 0", got)
	}
	if got := clampRate(2.5); got != 2.5 {
		t.Fatalf("clampRate(2.5) = %v, want 2.5", got)
	}
	if got := clampRate(0); got != 0 {
		t.Fatalf("clampRate(0) = %v, want 0", got)
	}
}

func TestRateResetExclusion(t *testing.T) {
	base := time.Now()
	h := hist(
		snap("wrk_x", true, 90, base),                  // period A, high usage
		snap("wrk_x", true, 95, base.Add(time.Hour)),   // period A
		snap("wrk_x", true, 10, base.Add(2*time.Hour)), // period B starts (drop)
		snap("wrk_x", true, 30, base.Add(4*time.Hour)), // period B
		snap("wrk_x", true, 54, base.Add(6*time.Hour)), // period B
	)
	p := estimateRate(h, bucketWeekly)
	if !p.HasRate {
		t.Fatal("expected a rate from period B")
	}
	want := (54.0 - 10.0) / 4.0 // 11 %/h over the 4h of period B
	if p.RatePctPerHour != want {
		t.Fatalf("rate = %v, want %v (pre-reset history excluded)", p.RatePctPerHour, want)
	}
	if p.RateSpanSec != 4*3600 {
		t.Fatalf("span = %d, want %d", p.RateSpanSec, 4*3600)
	}
}

func TestRateResetInSecUptickDetection(t *testing.T) {
	base := time.Now()
	a := snap("wrk_x", true, 100, base)
	a.Weekly.ResetInSec = 100
	b := snap("wrk_x", true, 100, base.Add(time.Hour))
	b.Weekly.ResetInSec = 50 // within-period: ticks down
	c := snap("wrk_x", true, 100, base.Add(2*time.Hour))
	c.Weekly.ResetInSec = 400 // uptick without usage drop: a reset
	d := snap("wrk_x", true, 100, base.Add(3*time.Hour))
	d.Weekly.ResetInSec = 350 // post-reset: ticks down again
	h := hist(a, b, c, d)
	p := estimateRate(h, bucketWeekly)
	if !p.HasRate {
		t.Fatal("expected a rate")
	}
	if p.RatePctPerHour != 0 {
		t.Fatalf("rate = %v, want 0 (post-reset window has no consumption)", p.RatePctPerHour)
	}
	if p.RateSpanSec != 3600 {
		t.Fatalf("span = %d, want %d (window must start after the uptick)", p.RateSpanSec, 3600)
	}
}

func TestProjectionPeriodAnchored(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC)
	s := snap("wrk_x", true, 36, now)
	s.Monthly.ResetInSec = 1672561 // next boundary Aug 28 21:46 UTC, window 31d, elapsed 11d 15h 24m
	per := computePeriod(s, bucketMonthly, now)
	if per.Projected == nil {
		t.Fatal("expected a projection")
	}
	if *per.Projected != 96 {
		t.Fatalf("projected = %d, want 96 (36%% x 31d / 11.64d)", *per.Projected)
	}
}

func TestProjectionAtPeriodStart(t *testing.T) {
	now := time.Date(2026, 7, 28, 21, 46, 1, 0, time.UTC) // exactly at the previous boundary
	s := snap("wrk_x", true, 1, now)
	s.Monthly.ResetInSec = 31 * 24 * 3600 // next boundary Aug 28 21:46 UTC
	per := computePeriod(s, bucketMonthly, now)
	if per.Known != true {
		t.Fatal("expected a known window")
	}
	if per.Projected != nil {
		t.Fatalf("expected no projection at period start, got %d", *per.Projected)
	}
}

func TestProjectionNoWindow(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC)
	s := snap("wrk_x", true, 36, now.Add(-30*time.Hour))
	s.Monthly.ResetInSec = 600 // period ended long ago
	per := computePeriod(s, bucketMonthly, now)
	if per.Known || per.Projected != nil {
		t.Fatalf("expected no window and no projection, got %+v", per)
	}
}

func TestProjectionRollingIncluded(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC)
	s := snap("wrk_x", true, 6, now)
	s.Rolling.ResetInSec = 11807
	per := computePeriod(s, bucketRolling, now)
	if per.Projected == nil {
		t.Fatal("rolling bucket must get a projection")
	}
	if *per.Projected != 17 {
		t.Fatalf("rolling projected = %d, want 17 (6%% x 5h / 1h43m)", *per.Projected)
	}
}

func TestProjectionStaleUsesFetchElapsed(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC)
	fetched := now.Add(-5 * time.Hour)
	s := snap("wrk_x", true, 36, fetched)
	s.Weekly.ResetInSec = 2 * 24 * 3600
	fresh := computePeriod(s, bucketWeekly, fetched)
	stale := computePeriod(s, bucketWeekly, now)
	if !fresh.Known || !stale.Known {
		t.Fatal("expected known windows")
	}
	if *fresh.Projected != *stale.Projected {
		t.Fatalf("stale projection must stay fetch-time: fresh %d, stale %d", *fresh.Projected, *stale.Projected)
	}
	if stale.Position <= fresh.Position {
		t.Fatalf("stale position must advance with the clock: fresh %v, stale %v", fresh.Position, stale.Position)
	}
}

func TestPeriodRollingWindow(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC)
	s := snap("wrk_x", true, 4, now)
	s.Rolling.ResetInSec = 11807
	p := computePeriod(s, bucketRolling, now)
	if !p.Known {
		t.Fatal("expected rolling window")
	}
	if p.Window != 5*time.Hour {
		t.Fatalf("rolling window = %v, want 5h", p.Window)
	}
	wantElapsed := p.Window - time.Duration(s.Rolling.ResetInSec)*time.Second
	if p.Elapsed != wantElapsed {
		t.Fatalf("elapsed = %v, want %v", p.Elapsed, wantElapsed)
	}
}

func TestPeriodWeeklyMondayBoundary(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC) // Sunday
	s := snap("wrk_x", true, 54, now)
	s.Weekly.ResetInSec = int((time.Duration(10)*time.Hour + 50*time.Minute) / time.Second)
	p := computePeriod(s, bucketWeekly, now)
	if !p.Known {
		t.Fatal("expected weekly window")
	}
	if p.Window != 7*24*time.Hour {
		t.Fatalf("weekly window = %v, want 7d", p.Window)
	}
	next := boundaryNext(s, bucketWeekly)
	if next.Weekday() != time.Monday || next.Hour() != 0 || next.Minute() != 0 {
		t.Fatalf("next weekly boundary = %v, want Monday 00:00 UTC", next)
	}
}

func TestPeriodMonthlySameDay(t *testing.T) {
	now := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	s := snap("wrk_x", true, 35, now)
	s.Monthly.ResetInSec = int((time.Duration(18)*24*time.Hour + 20*time.Hour + 42*time.Minute) / time.Second)
	next := boundaryNext(s, bucketMonthly)
	if next.Format("2006-01-02 15:04") != "2026-03-28 20:42" {
		t.Fatalf("next boundary = %v, want 2026-03-28 20:42", next)
	}
	prev := previousBoundary(next, bucketMonthly)
	if prev.Format("2006-01-02 15:04") != "2026-02-28 20:42" {
		t.Fatalf("prev boundary = %v, want 2026-02-28 20:42 (non-leap)", prev)
	}
	p := computePeriod(s, bucketMonthly, now)
	if !p.Known {
		t.Fatal("expected monthly window")
	}
	if p.Window != 28*24*time.Hour {
		t.Fatalf("window = %v, want 28d (Feb 28 -> Mar 28)", p.Window)
	}
}

func TestPeriodMonthlyClampShortMonth(t *testing.T) {
	next := time.Date(2026, 5, 31, 20, 42, 0, 0, time.UTC)
	prev := previousBoundary(next, bucketMonthly)
	if prev.Format("2006-01-02 15:04") != "2026-04-30 20:42" {
		t.Fatalf("prev = %v, want 2026-04-30 20:42", prev)
	}
}

func TestPeriodMonthlyClampFebruary(t *testing.T) {
	next := time.Date(2026, 3, 31, 20, 42, 0, 0, time.UTC)
	prev := previousBoundary(next, bucketMonthly)
	if prev.Format("2006-01-02 15:04") != "2026-02-28 20:42" {
		t.Fatalf("prev = %v, want 2026-02-28 20:42", prev)
	}
}

func TestPeriodMonthlyLeapYear(t *testing.T) {
	next := time.Date(2028, 2, 29, 20, 42, 0, 0, time.UTC)
	prev := previousBoundary(next, bucketMonthly)
	if prev.Format("2006-01-02 15:04") != "2028-01-29 20:42" {
		t.Fatalf("prev = %v, want 2028-01-29 20:42", prev)
	}
	next = time.Date(2028, 3, 31, 20, 42, 0, 0, time.UTC)
	prev = previousBoundary(next, bucketMonthly)
	if prev.Format("2006-01-02 15:04") != "2028-02-29 20:42" {
		t.Fatalf("prev = %v, want 2028-02-29 20:42 (leap)", prev)
	}
}

func TestPeriodInconsistentUnavailable(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC)
	fetched := now.Add(-20 * time.Hour)
	s := snap("wrk_x", true, 54, fetched)
	s.Weekly.ResetInSec = 10 * 3600 // period ended 10h ago (10h remaining at fetch, 20h ago)
	p := computePeriod(s, bucketWeekly, now)
	if p.Known {
		t.Fatal("window must be unavailable when the period has already ended")
	}
}

func TestPeriodStaleWithinWindow(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC)
	fetched := now.Add(-5 * time.Hour)
	s := snap("wrk_x", true, 54, fetched)
	s.Weekly.ResetInSec = 2 * 24 * 3600
	p := computePeriod(s, bucketWeekly, now)
	if !p.Known {
		t.Fatal("expected window for stale data still inside the period")
	}
	if p.Elapsed != now.Sub(previousBoundary(boundaryNext(s, bucketWeekly), bucketWeekly)) {
		t.Fatal("elapsed must be measured from the current clock")
	}
}

func TestPositionMath(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC) // Sunday: next boundary Monday 00:00
	s := snap("wrk_x", true, 54, now)
	s.Weekly.ResetInSec = int((time.Duration(10)*time.Hour + 50*time.Minute) / time.Second)
	p := computePeriod(s, bucketWeekly, now)
	if !p.Known {
		t.Fatal("expected weekly window")
	}
	wantElapsed := 7*24*time.Hour - time.Duration(s.Weekly.ResetInSec)*time.Second // 6d 13h 10m
	if p.Elapsed != wantElapsed {
		t.Fatalf("elapsed = %v, want %v", p.Elapsed, wantElapsed)
	}
	frac := float64(p.Elapsed) / float64(p.Window)
	if frac < 0.93 || frac > 0.94 {
		t.Fatalf("elapsed fraction = %v, want ~0.936 (6d 13h of 7d)", frac)
	}
}

func TestComputePeriodsRollingIncluded(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 10, 0, 0, time.UTC)
	s := snap("wrk_x", true, 4, now)
	s.Rolling.ResetInSec = 60 * 60
	ps := computePeriods(s, now)
	if !ps.Rolling.Known {
		t.Fatal("rolling must have a position window")
	}
	if !ps.Weekly.Known || !ps.Monthly.Known {
		t.Fatal("weekly and monthly must have position windows")
	}
}
