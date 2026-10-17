// Package bunchinglib holds the bunching definitions shared by the live bunching site and the bunching logger.
package bunchinglib

import (
	"time"

	"adrianlehmann.net/cta"
)

// Bunched means a gap is under 2 minutes, or under 25% of the median gap in that direction,
// whichever is larger. The relative part matters on infrequent routes, where 2 minutes is too strict.
const (
	BunchMinutes  = 2
	BunchFraction = 0.25
)

// Rule says when a gap counts as bunched: under Minutes, or under Fraction of the median gap, whichever is larger.
type Rule struct {
	Minutes  float64
	Fraction float64
}

var DefaultRule = Rule{Minutes: BunchMinutes, Fraction: BunchFraction}

// Threshold is the gap under which buses count as bunched in these headways.
func (r Rule) Threshold(h cta.Headways) time.Duration {
	abs := minutesToDuration(r.Minutes)
	if h.Count() == 0 {
		return abs
	}
	return max(abs, time.Duration(r.Fraction*float64(h.Median())))
}

// Bunched reports for each gap in h whether it is under the threshold.
func Bunched(h cta.Headways, threshold time.Duration) []bool {
	bunched := make([]bool, len(h.Gaps))
	for i, g := range h.Gaps {
		bunched[i] = g < threshold
	}
	return bunched
}

// Convoys are the runs of consecutive bunched gaps, as [first gap, one past the last gap].
// Gap i is between buses i (ahead) and i+1 (behind), so a run [i, j) is the j-i+1 buses i..j.
func Convoys(bunched []bool) [][2]int {
	var runs [][2]int
	for i := 0; i < len(bunched); {
		if !bunched[i] {
			i++
			continue
		}
		j := i
		for j < len(bunched) && bunched[j] {
			j++
		}
		runs = append(runs, [2]int{i, j})
		i = j
	}
	return runs
}

// Headways wraps gaps in minutes, as the examples compute them, in the API type.
func Headways(route string, direction string, minutes []float64) cta.Headways {
	h := cta.Headways{Route: route, Direction: direction}
	for _, m := range minutes {
		h.Gaps = append(h.Gaps, minutesToDuration(m))
	}
	return h
}

func minutesToDuration(m float64) time.Duration {
	return time.Duration(m * float64(time.Minute))
}
