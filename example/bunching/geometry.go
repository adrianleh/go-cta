package main

import (
	"math"

	"adrianlehmann.net/cta"
)

const FEET_PER_DEGREE_LAT = 364_000

type refStop struct {
	name string
	atFt float64
}

// A pattern turned into something we can measure along: cumulative distance for every point.
// Each direction of a route gets one reference pattern (its longest) and every bus in that direction is placed on it,
// so short-turn and branch variants share one axis.
type refPattern struct {
	pid       int
	direction string
	points    []cta.Position
	cumFt     []float64
	lengthFt  float64
	apiLength float64
	stops     []refStop
}

func newRefPattern(p cta.Pattern) *refPattern {
	r := &refPattern{pid: p.Pid, direction: p.RouteDirection, apiLength: p.LengthFeet}
	for i, pt := range p.Points {
		pos := pt.GetPosition()
		if i > 0 {
			r.lengthFt += distanceFt(r.points[i-1], pos)
		}
		r.points = append(r.points, pos)
		r.cumFt = append(r.cumFt, r.lengthFt)
		if pt.IsStop() {
			r.stops = append(r.stops, refStop{name: pt.StopName, atFt: r.lengthFt})
		}
	}
	return r
}

func (r *refPattern) startName() string {
	if len(r.stops) == 0 {
		return ""
	}
	return r.stops[0].name
}

func (r *refPattern) endName() string {
	if len(r.stops) == 0 {
		return ""
	}
	return r.stops[len(r.stops)-1].name
}

// Picks the longest pattern of every direction as that direction's reference
func referencePatterns(patterns []cta.Pattern) map[string]*refPattern {
	refs := make(map[string]*refPattern)
	for _, p := range patterns {
		if existing, ok := refs[p.RouteDirection]; ok && existing.apiLength >= p.LengthFeet {
			continue
		}
		refs[p.RouteDirection] = newRefPattern(p)
	}
	return refs
}

// Flat-earth distance - fine at city scale
func distanceFt(a, b cta.Position) float64 {
	x, y := toLocalFt(b, a.Latitude)
	x0, y0 := toLocalFt(a, a.Latitude)
	return math.Hypot(x-x0, y-y0)
}

func toLocalFt(p cta.Position, refLat float64) (float64, float64) {
	return p.Longitude * FEET_PER_DEGREE_LAT * math.Cos(refLat*math.Pi/180), p.Latitude * FEET_PER_DEGREE_LAT
}

// Distance along the pattern of the point on it closest to pos, and how far off the pattern pos is
func (r *refPattern) project(pos cta.Position) (float64, float64) {
	if len(r.points) == 0 {
		return 0, math.Inf(1)
	}
	px, py := toLocalFt(pos, pos.Latitude)
	bestAlong, bestOff := 0.0, math.Inf(1)
	for i := 1; i < len(r.points); i++ {
		ax, ay := toLocalFt(r.points[i-1], pos.Latitude)
		bx, by := toLocalFt(r.points[i], pos.Latitude)
		dx, dy := bx-ax, by-ay
		segLen2 := dx*dx + dy*dy
		t := 0.0
		if segLen2 > 0 {
			t = min(max(((px-ax)*dx+(py-ay)*dy)/segLen2, 0), 1)
		}
		off := math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
		if off < bestOff {
			bestOff = off
			bestAlong = r.cumFt[i-1] + t*(r.cumFt[i]-r.cumFt[i-1])
		}
	}
	return bestAlong, bestOff
}

// Where along its direction's reference pattern a vehicle is. Buses on the reference pattern itself use the
// API's pdist (scaled to our measured length), everyone else gets projected by position.
func (r *refPattern) place(v cta.Vehicle) (float64, bool) {
	if v.PatternId == r.pid && r.apiLength > 0 {
		return float64(v.PatternDistanceFt) * r.lengthFt / r.apiLength, true
	}
	pos, err := v.GetPosition()
	if err != nil {
		return 0, false
	}
	along, _ := r.project(pos.Position)
	return along, true
}
