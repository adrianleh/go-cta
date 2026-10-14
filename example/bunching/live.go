package main

import (
	"cmp"
	"math"
	"slices"
)

// Live view of one route: where every bus is right now along each direction's pattern,
// the gaps between them, and which gaps count as bunched.
//
// Gaps are measured in feet along the pattern and turned into minutes with one speed per direction
// (see directionSpeed). Using one speed for the whole direction keeps gaps comparable to each other -
// a bus sitting at a light doesn't suddenly make the gap behind it look huge.

const (
	DEFAULT_SPEED_MPH = 10 // rough CTA local bus average including stops, used when reported speeds are no good
	MIN_SPEED_MPH     = 3
	MAX_SPEED_MPH     = 25
	MIN_SPEED_SAMPLES = 3
	TERMINAL_ZONE_FT  = 1000 // buses this close to either end are laying over, not in service on the strip
	FEET_PER_MILE     = 5280
)

// Bunched means the gap to the bus ahead is under 2 minutes, or under 25% of the median gap in that direction,
// whichever is larger. The relative part matters on infrequent routes, where 2 minutes is too strict.
const (
	BUNCH_MINUTES  = 2
	BUNCH_FRACTION = 0.25
)

type liveBus struct {
	VehicleId   string
	Destination string
	PositionFt  float64 // along the direction's reference pattern
	SpeedMph    int     // as reported by the bus
	AtTerminal  bool
}

type liveGap struct {
	FromFt  float64 // bus behind
	ToFt    float64 // bus ahead
	Minutes float64
	Bunched bool
}

type liveDirection struct {
	Direction   string
	LengthFt    float64
	StartName   string
	EndName     string
	Buses       []liveBus // in service first, furthest along first, then buses laying over at a terminal
	Gaps        []liveGap // front to back
	SpeedMph    float64
	SpeedIsReal bool // false when we fell back to DEFAULT_SPEED_MPH

	InService       int
	BunchedPairs    int
	BunchMinutes    float64 // threshold actually used
	MedianGap       float64 // minutes
	ExpectedWait    float64 // minutes, NaN with fewer than 2 gaps
	EvenSpacingWait float64 // minutes, NaN with fewer than 2 gaps
}

func bunchThreshold(medianGap float64) float64 {
	if math.IsNaN(medianGap) {
		return BUNCH_MINUTES
	}
	return max(BUNCH_MINUTES, BUNCH_FRACTION*medianGap)
}

func feetToMinutes(ft float64, mph float64) float64 {
	return ft / FEET_PER_MILE / mph * 60
}

func buildLiveDirection(ref *refPattern, buses []liveBus) liveDirection {
	d := liveDirection{
		Direction: ref.direction,
		LengthFt:  ref.lengthFt,
		StartName: ref.startName(),
		EndName:   ref.endName(),
	}

	var inService, terminal []liveBus
	for _, b := range buses {
		b.PositionFt = min(max(b.PositionFt, 0), ref.lengthFt)
		b.AtTerminal = b.PositionFt < TERMINAL_ZONE_FT || b.PositionFt > ref.lengthFt-TERMINAL_ZONE_FT
		if b.AtTerminal {
			terminal = append(terminal, b)
		} else {
			inService = append(inService, b)
		}
	}
	slices.SortFunc(inService, func(a, b liveBus) int { return cmp.Compare(b.PositionFt, a.PositionFt) })
	d.Buses = append(inService, terminal...)
	d.InService = len(inService)

	speeds := make([]int, len(inService))
	for i, b := range inService {
		speeds[i] = b.SpeedMph
	}
	d.SpeedMph, d.SpeedIsReal = directionSpeed(speeds)

	minutes := make([]float64, 0, len(inService))
	for i := 1; i < len(inService); i++ {
		ahead, behind := inService[i-1], inService[i]
		m := feetToMinutes(ahead.PositionFt-behind.PositionFt, d.SpeedMph)
		d.Gaps = append(d.Gaps, liveGap{FromFt: behind.PositionFt, ToFt: ahead.PositionFt, Minutes: m})
		minutes = append(minutes, m)
	}

	d.MedianGap = median(minutes)
	d.BunchMinutes = bunchThreshold(d.MedianGap)
	for i := range d.Gaps {
		if d.Gaps[i].Minutes < d.BunchMinutes {
			d.Gaps[i].Bunched = true
			d.BunchedPairs++
		}
	}

	// The gap ahead of the lead bus and behind the last one aren't known, so they're not part of the wait
	d.ExpectedWait = math.NaN()
	d.EvenSpacingWait = math.NaN()
	if len(minutes) >= 2 {
		d.ExpectedWait = expectedWait(minutes)
		d.EvenSpacingWait = mean(minutes) / 2
	}
	return d
}

// One speed per direction: the average of the speeds the buses report right now. Averaged over many buses -
// some stopped at a stop or a light, some cruising - that's roughly the average running speed including stops.
// With too few buses, or a feed that reports all zeros, we fall back to DEFAULT_SPEED_MPH.
func directionSpeed(speeds []int) (float64, bool) {
	if len(speeds) < MIN_SPEED_SAMPLES {
		return DEFAULT_SPEED_MPH, false
	}
	total := 0
	for _, s := range speeds {
		total += s
	}
	avg := float64(total) / float64(len(speeds))
	if avg < MIN_SPEED_MPH {
		return DEFAULT_SPEED_MPH, false
	}
	return min(avg, MAX_SPEED_MPH), true
}

// Average wait for a rider who shows up at a random time.
// A rider lands in a gap with probability proportional to its length and then waits half of it on average,
// so W = sum(h * h/2) / sum(h) = E[H^2] / (2 E[H]). For evenly spaced buses this is just E[H] / 2.
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
