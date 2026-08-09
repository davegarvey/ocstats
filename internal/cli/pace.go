package cli

import (
	"math"
	"time"

	"ocstats/internal/fetch"
)

// clock is the time source for stamping snapshots and computing stale ages;
// tests override it for determinism.
var clock = time.Now

type bucketKind int

const (
	bucketRolling bucketKind = iota
	bucketWeekly
	bucketMonthly
)

func bucket(s fetch.Snapshot, k bucketKind) fetch.Bucket {
	switch k {
	case bucketRolling:
		return s.Rolling
	case bucketWeekly:
		return s.Weekly
	default:
		return s.Monthly
	}
}

// paceInfo is the consumption rate for one bucket: percentage points per hour
// averaged over the observed window of the current period, plus the span that
// window covers.
type paceInfo struct {
	HasRate        bool
	RatePctPerHour float64
	RateSpanSec    int64
}

// rateWindow trims history to the longest suffix ending at the latest
// snapshot with no reset boundary inside it. A reset is detected between
// consecutive snapshots by a usage drop or by a resetInSec uptick (time until
// the period end can only go down within a period, so any uptick is a reset).
func rateWindow(h []fetch.Snapshot, k bucketKind) []fetch.Snapshot {
	for i := len(h) - 2; i >= 0; i-- {
		prev, cur := bucket(h[i], k), bucket(h[i+1], k)
		if cur.UsagePercent < prev.UsagePercent || cur.ResetInSec > prev.ResetInSec {
			return h[i+1:]
		}
	}
	return h
}

// estimateRate computes the average consumption rate over the current
// period's window. It requires at least two snapshots and a strictly positive
// window duration; the rate is clamped to a minimum of zero.
func estimateRate(h []fetch.Snapshot, k bucketKind) paceInfo {
	w := rateWindow(h, k)
	if len(w) < 2 {
		return paceInfo{}
	}
	first, last := bucket(w[0], k), bucket(w[len(w)-1], k)
	span := w[len(w)-1].FetchedAt.Sub(w[0].FetchedAt)
	if span <= 0 {
		return paceInfo{}
	}
	rate := (float64(last.UsagePercent) - float64(first.UsagePercent)) / span.Hours()
	return paceInfo{
		HasRate:        true,
		RatePctPerHour: clampRate(rate),
		RateSpanSec:    int64(span.Seconds()),
	}
}

func clampRate(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}

// bucketPace bundles a bucket's consumption rate (observation-window average),
// which is data for JSON consumers; the displayed projection is period-anchored
// and lives on periodInfo.
type bucketPace struct {
	Info paceInfo
}

// paces holds the per-bucket pace results for one rendering.
type paces struct {
	Rolling bucketPace
	Weekly  bucketPace
	Monthly bucketPace
}

// computePaces derives the observation-window consumption rate for all
// buckets from the current period's history; the rate feeds the JSON contract
// as data. adjust is the time elapsed since the snapshot was fetched and is
// reserved for rate consumers; the displayed projection is period-anchored
// (see computePeriod) and independent of history.
func computePaces(history []fetch.Snapshot, s fetch.Snapshot, adjust time.Duration) paces {
	var p paces
	p.Rolling.Info = estimateRate(history, bucketRolling)
	p.Weekly.Info = estimateRate(history, bucketWeekly)
	p.Monthly.Info = estimateRate(history, bucketMonthly)
	return p
}

const (
	periodRolling = 5 * time.Hour
	periodWeekly  = 7 * 24 * time.Hour
	// windowTolerance bounds the sanity check: the clock-based elapsed plus
	// the snapshot's remaining time must approximate the inferred window.
	windowTolerance = 24 * time.Hour
)

// periodInfo describes a bucket's current period window.
type periodInfo struct {
	Known bool
	// Window is the length of the current period.
	Window time.Duration
	// Elapsed is the clock-based time since the previous boundary (advances
	// even with stale data).
	Elapsed time.Duration
	// ElapsedAtFetch is the time since the previous boundary at the snapshot's
	// fetch time (anchors the projection to the data, not the clock).
	ElapsedAtFetch time.Duration
	// Position is Elapsed/Window, clamped to [0,1].
	Position float64
	// Projected is the period-anchored usage at period end: current usage
	// times Window/ElapsedAtFetch. Nil when not computable.
	Projected *int
}

// periodInfos holds the per-bucket period windows for one rendering.
type periodInfos struct {
	Rolling periodInfo
	Weekly  periodInfo
	Monthly periodInfo
}

// boundaryNext returns the period end: the snapshot's fetch time plus its
// reset seconds. For stale snapshots this keeps the boundary at the fixed
// instant the data describes, independent of the current clock.
func boundaryNext(s fetch.Snapshot, k bucketKind) time.Time {
	return s.FetchedAt.Add(time.Duration(bucket(s, k).ResetInSec) * time.Second)
}

// previousBoundary derives the period start from the next boundary: a fixed
// 5-hour step for rolling, a fixed 7-day step for weekly, and the same
// day-of-month and time-of-day in the previous calendar month for monthly.
func previousBoundary(next time.Time, k bucketKind) time.Time {
	switch k {
	case bucketRolling:
		return next.Add(-periodRolling)
	case bucketWeekly:
		return next.Add(-periodWeekly)
	default:
		return previousMonthBoundary(next)
	}
}

// previousMonthBoundary returns the same day-of-month and time-of-day in the
// previous calendar month, clamped to the previous month's last day when the
// day does not exist there. All arithmetic is UTC on fixed instants.
func previousMonthBoundary(next time.Time) time.Time {
	y, m, d := next.Date()
	hh, mm, ss := next.Clock()
	py, pm := y, m-1
	if pm == 0 {
		pm = 12
		py--
	}
	if d > daysInMonth(py, pm) {
		d = daysInMonth(py, pm)
	}
	return time.Date(py, pm, d, hh, mm, ss, next.Nanosecond(), time.UTC)
}

func daysInMonth(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// computePeriod infers a bucket's period window from the snapshot and the
// current time, along with the position within it and the period-anchored
// projection. The window is unavailable when it cannot be derived or when the
// inference is inconsistent: the clock-based elapsed time plus the snapshot's
// remaining time must approximate the window length within the tolerance, and
// the current time must not be past the next boundary. Both fail when the data
// no longer describes the current period (e.g. a stale snapshot whose period
// has already ended), which would otherwise render a position against an
// expired period.
func computePeriod(s fetch.Snapshot, k bucketKind, now time.Time) periodInfo {
	next := boundaryNext(s, k)
	prev := previousBoundary(next, k)
	window := next.Sub(prev)
	elapsed := now.Sub(prev)
	mismatch := window - (elapsed + time.Duration(bucket(s, k).ResetInSec)*time.Second)
	if mismatch < 0 {
		mismatch = -mismatch
	}
	if mismatch > windowTolerance || elapsed > window {
		return periodInfo{}
	}
	elapsedAtFetch := s.FetchedAt.Sub(prev)
	position := float64(elapsed) / float64(window)
	if position < 0 {
		position = 0
	}
	if position > 1 {
		position = 1
	}
	var projected *int
	if elapsedAtFetch > 0 {
		v := float64(bucket(s, k).UsagePercent) * float64(window) / float64(elapsedAtFetch)
		r := int(math.Round(v))
		projected = &r
	}
	return periodInfo{
		Known:          true,
		Window:         window,
		Elapsed:        elapsed,
		ElapsedAtFetch: elapsedAtFetch,
		Position:       position,
		Projected:      projected,
	}
}

// computePeriods derives the period windows for all three buckets. now is the
// current time: the snapshot's own fetch time for fresh data, the clock for
// stale data.
func computePeriods(s fetch.Snapshot, now time.Time) periodInfos {
	return periodInfos{
		Rolling: computePeriod(s, bucketRolling, now),
		Weekly:  computePeriod(s, bucketWeekly, now),
		Monthly: computePeriod(s, bucketMonthly, now),
	}
}
