package main

import (
	"math"
	"slices"
	"time"

	"adrianlehmann.net/cta"
	"adrianlehmann.net/cta/example/bunchinglib"
)

// Observations of one vehicle running one pattern, ordered by time
type track []Observation

// A track is split into a new run when the bus goes backwards by more than this (new trip on the same pattern)
// or when we lose sight of it for too long. Small backwards jumps are GPS noise.
const RUN_BACKTRACK_TOLERANCE_FT = 1000

// Groups observations into per-vehicle, per-pattern runs that each cover one pass along the pattern
func buildRuns(obs []Observation, maxGap time.Duration) []track {
	type key struct {
		vid string
		pid int
	}
	byVehicle := make(map[key]track)
	for _, o := range obs {
		k := key{o.VehicleId, o.PatternId}
		byVehicle[k] = append(byVehicle[k], o)
	}

	var runs []track
	for _, t := range byVehicle {
		slices.SortFunc(t, func(a, b Observation) int { return a.Timestamp.Compare(b.Timestamp) })
		// The same vehicle report shows up in consecutive polls if the bus hasn't sent a new ping
		t = slices.CompactFunc(t, func(a, b Observation) bool { return a.Timestamp.Equal(b.Timestamp) })
		current := track{t[0]}
		for _, o := range t[1:] {
			prev := current[len(current)-1]
			if o.PatternDistanceFt < prev.PatternDistanceFt-RUN_BACKTRACK_TOLERANCE_FT || o.Timestamp.Sub(prev.Timestamp) > maxGap {
				runs = append(runs, current)
				current = track{}
			}
			current = append(current, o)
		}
		runs = append(runs, current)
	}
	return runs
}

// Linear interpolation of when the bus passed distance d, given it was at d0 at t0 and d1 at t1
func interpolateCrossing(t0 time.Time, d0 float64, t1 time.Time, d1 float64, d float64) time.Time {
	if d1 == d0 {
		return t1
	}
	frac := (d - d0) / (d1 - d0)
	return t0.Add(time.Duration(frac * float64(t1.Sub(t0))))
}

// When (if at all) this run passed distance d along its pattern. Only the first crossing counts so GPS jitter
// around d doesn't produce phantom buses.
func (r track) crossing(d float64) (time.Time, bool) {
	for i := 1; i < len(r); i++ {
		d0 := float64(r[i-1].PatternDistanceFt)
		d1 := float64(r[i].PatternDistanceFt)
		if d0 < d && d <= d1 {
			return interpolateCrossing(r[i-1].Timestamp, d0, r[i].Timestamp, d1, d), true
		}
	}
	return time.Time{}, false
}

type interval struct {
	start time.Time
	end   time.Time
}

// Stretches where the logger saw nothing for longer than maxGap (logger down, API errors, no service).
// Polls are the timestamps at which we got data for the route.
func coverageGaps(polls []time.Time, maxGap time.Duration) []interval {
	sorted := slices.Clone(polls)
	slices.SortFunc(sorted, func(a, b time.Time) int { return a.Compare(b) })
	sorted = slices.CompactFunc(sorted, func(a, b time.Time) bool { return a.Equal(b) })
	var gaps []interval
	for i := 1; i < len(sorted); i++ {
		if sorted[i].Sub(sorted[i-1]) > maxGap {
			gaps = append(gaps, interval{sorted[i-1], sorted[i]})
		}
	}
	return gaps
}

type headway struct {
	stop    string
	start   time.Time // when the first bus passed
	at      time.Time // when the second bus passed
	minutes float64
}

// Headways between consecutive crossings. A headway that overlaps a coverage gap is dropped -
// we can't tell whether a bus went by while we weren't looking.
func headways(stop string, crossings []time.Time, gaps []interval) []headway {
	sorted := slices.Clone(crossings)
	slices.SortFunc(sorted, func(a, b time.Time) int { return a.Compare(b) })
	var hs []headway
	for i := 1; i < len(sorted); i++ {
		prev, cur := sorted[i-1], sorted[i]
		if overlapsAny(prev, cur, gaps) {
			continue
		}
		hs = append(hs, headway{stop: stop, start: prev, at: cur, minutes: cur.Sub(prev).Minutes()})
	}
	return hs
}

func overlapsAny(start, end time.Time, gaps []interval) bool {
	for _, g := range gaps {
		if g.start.Before(end) && g.end.After(start) {
			return true
		}
	}
	return false
}

// Headways in a row at one stop, where each one starts when the previous one ended. Dropped headways and
// changes of stop break the chain, so a bunched run never spans a stretch we didn't see.
func chains(route string, direction string, hs []headway) []cta.Headways {
	var out []cta.Headways
	for i, h := range hs {
		if i == 0 || h.stop != hs[i-1].stop || !h.start.Equal(hs[i-1].at) {
			out = append(out, cta.Headways{Route: route, Direction: direction})
		}
		c := &out[len(out)-1]
		c.Gaps = append(c.Gaps, minutesToDuration(h.minutes))
	}
	return out
}

type HeadwayStats struct {
	Count           int
	MeanMinutes     float64
	MedianMinutes   float64
	CV              float64 // coefficient of variation, stddev / mean
	ExpectedWait    float64 // minutes, E[H^2] / (2 E[H])
	EvenSpacingWait float64 // minutes, E[H] / 2
	ThresholdMin    float64 // headways under this many minutes count as bunched
	BunchedShare    float64 // share of headways that are bunched
	Clusters        int     // runs of bunched headways in a row, each one a group of 2 or more buses
	BunchedBuses    int     // buses in those clusters
}

// Same definitions as the live site: the wait comes from cta.Headways and bunching from bunchinglib.
// The headways are chains of consecutive headways (see chains) and are pooled for everything but clusters.
func computeStats(chains []cta.Headways, rule bunchinglib.Rule) HeadwayStats {
	pooled := cta.Headways{}
	for _, c := range chains {
		pooled.Gaps = append(pooled.Gaps, c.Gaps...)
	}
	if pooled.Count() == 0 {
		nan := math.NaN()
		return HeadwayStats{MeanMinutes: nan, MedianMinutes: nan, CV: nan, ExpectedWait: nan, EvenSpacingWait: nan, ThresholdMin: nan, BunchedShare: nan}
	}
	m := pooled.Mean().Minutes()
	var variance float64
	for _, g := range pooled.Gaps {
		variance += (g.Minutes() - m) * (g.Minutes() - m)
	}
	variance /= float64(pooled.Count())

	threshold := rule.Threshold(pooled)
	stats := HeadwayStats{
		Count:           pooled.Count(),
		MeanMinutes:     m,
		MedianMinutes:   pooled.Median().Minutes(),
		CV:              math.Sqrt(variance) / m,
		ExpectedWait:    pooled.ExpectedWait().Minutes(),
		EvenSpacingWait: pooled.EvenSpacingWait().Minutes(),
		ThresholdMin:    threshold.Minutes(),
	}
	bunchedGaps := 0
	for _, c := range chains {
		bunched := bunchinglib.Bunched(c, threshold)
		for _, b := range bunched {
			if b {
				bunchedGaps++
			}
		}
		for _, run := range bunchinglib.Convoys(bunched) {
			stats.Clusters++
			stats.BunchedBuses += run[1] - run[0] + 1
		}
	}
	stats.BunchedShare = float64(bunchedGaps) / float64(pooled.Count())
	return stats
}

func minutesToDuration(m float64) time.Duration {
	return time.Duration(m * float64(time.Minute))
}
