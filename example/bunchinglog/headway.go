package main

import (
	"math"
	"slices"
	"time"
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
	at      time.Time // when the second bus passed
	minutes float64
}

// Headways between consecutive crossings. A headway that overlaps a coverage gap is dropped -
// we can't tell whether a bus went by while we weren't looking.
func headways(crossings []time.Time, gaps []interval) []headway {
	sorted := slices.Clone(crossings)
	slices.SortFunc(sorted, func(a, b time.Time) int { return a.Compare(b) })
	var hs []headway
	for i := 1; i < len(sorted); i++ {
		prev, cur := sorted[i-1], sorted[i]
		if overlapsAny(prev, cur, gaps) {
			continue
		}
		hs = append(hs, headway{at: cur, minutes: cur.Sub(prev).Minutes()})
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

type HeadwayStats struct {
	Count           int
	MeanMinutes     float64
	MedianMinutes   float64
	CV              float64 // coefficient of variation, stddev / mean
	ExpectedWait    float64 // minutes, E[H^2] / (2 E[H])
	EvenSpacingWait float64 // minutes, E[H] / 2
	ShareUnderAbs   float64 // share of headways under the absolute threshold
	ShareUnderRel   float64 // share of headways under the relative threshold (fraction of the median)
}

// Average wait for a rider who shows up at a random time.
// A rider lands in a headway with probability proportional to its length and then waits half of it on average,
// so W = sum(h * h/2) / sum(h) = E[H^2] / (2 E[H]). For evenly spaced buses this reduces to E[H] / 2.
func expectedWait(hs []float64) float64 {
	var sum, sumSq float64
	for _, h := range hs {
		sum += h
		sumSq += h * h
	}
	if sum == 0 {
		return math.NaN()
	}
	return sumSq / (2 * sum)
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	sorted := slices.Clone(xs)
	slices.Sort(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

func shareBelow(xs []float64, threshold float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	count := 0
	for _, x := range xs {
		if x < threshold {
			count++
		}
	}
	return float64(count) / float64(len(xs))
}

func computeStats(hs []float64, bunchMinutes float64, bunchFraction float64) HeadwayStats {
	m := mean(hs)
	var variance float64
	for _, h := range hs {
		variance += (h - m) * (h - m)
	}
	if len(hs) > 0 {
		variance /= float64(len(hs))
	}
	med := median(hs)
	return HeadwayStats{
		Count:           len(hs),
		MeanMinutes:     m,
		MedianMinutes:   med,
		CV:              math.Sqrt(variance) / m,
		ExpectedWait:    expectedWait(hs),
		EvenSpacingWait: m / 2,
		ShareUnderAbs:   shareBelow(hs, bunchMinutes),
		ShareUnderRel:   shareBelow(hs, bunchFraction*med),
	}
}
