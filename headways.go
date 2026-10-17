package cta

import (
	"math"
	"slices"
	"time"
)

// Headways are the gaps between consecutive vehicles on one route, optionally in one direction.
// Leave Direction empty to pool every direction of the route, or Route empty to pool routes.
// Statistics of an empty set are zero.
type Headways struct {
	Route     string
	Direction string
	Gaps      []time.Duration
}

// NewHeadwaysFromTimes builds headways from the times vehicles passed (or are predicted to pass) one point,
// in any order.
func NewHeadwaysFromTimes(route string, direction string, times []time.Time) Headways {
	sorted := slices.SortedFunc(slices.Values(times), time.Time.Compare)
	h := Headways{Route: route, Direction: direction}
	for i := 1; i < len(sorted); i++ {
		h.Gaps = append(h.Gaps, sorted[i].Sub(sorted[i-1]))
	}
	return h
}

func (h Headways) Count() int {
	return len(h.Gaps)
}

func (h Headways) Mean() time.Duration {
	if len(h.Gaps) == 0 {
		return 0
	}
	var sum time.Duration
	for _, g := range h.Gaps {
		sum += g
	}
	return sum / time.Duration(len(h.Gaps))
}

func (h Headways) Median() time.Duration {
	if len(h.Gaps) == 0 {
		return 0
	}
	sorted := slices.Sorted(slices.Values(h.Gaps))
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// Longest is the longest gap: the worst wait for a rider who just missed a vehicle.
func (h Headways) Longest() time.Duration {
	if len(h.Gaps) == 0 {
		return 0
	}
	return slices.Max(h.Gaps)
}

// ExpectedWait is the average wait for a rider who shows up at a random time.
// A rider lands in a gap with probability proportional to its length and then waits half of it on average,
// so W = sum(g * g/2) / sum(g) = E[G^2] / (2 E[G]). For evenly spaced vehicles this is just E[G] / 2.
func (h Headways) ExpectedWait() time.Duration {
	var sum, sumSq float64
	for _, g := range h.Gaps {
		sum += float64(g)
		sumSq += float64(g) * float64(g)
	}
	if sum == 0 {
		return 0
	}
	return time.Duration(math.Round(sumSq / (2 * sum)))
}

// EvenSpacingWait is the average wait if the same vehicles were evenly spaced: E[G] / 2.
func (h Headways) EvenSpacingWait() time.Duration {
	return h.Mean() / 2
}

// ShareUnder is the fraction of gaps shorter than d.
func (h Headways) ShareUnder(d time.Duration) float64 {
	if len(h.Gaps) == 0 {
		return 0
	}
	count := 0
	for _, g := range h.Gaps {
		if g < d {
			count++
		}
	}
	return float64(count) / float64(len(h.Gaps))
}
