package main

import (
	"cmp"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strings"
	"time"

	"adrianlehmann.net/cta/example/bunchinglib"
)

type analyzerConfig struct {
	inPath        string
	patternsPath  string
	reportPath    string
	chartPath     string
	stops         int
	bunchMinutes  float64
	bunchFraction float64
	minHeadways   int
	maxGap        time.Duration
	maxInterp     time.Duration
}

// Time of day buckets (America/Chicago). Holidays count as weekdays.
const (
	PERIOD_AM_PEAK   = "Weekday AM peak (6-9am)"
	PERIOD_MIDDAY    = "Weekday midday (9am-3pm)"
	PERIOD_PM_PEAK   = "Weekday PM peak (3-7pm)"
	PERIOD_EVENING   = "Weekday evening (7pm-midnight)"
	PERIOD_OVERNIGHT = "Weekday overnight (midnight-6am)"
	PERIOD_WEEKEND   = "Weekend"
)

var periodOrder = []string{PERIOD_AM_PEAK, PERIOD_MIDDAY, PERIOD_PM_PEAK, PERIOD_EVENING, PERIOD_OVERNIGHT, PERIOD_WEEKEND}

var chicago = mustLoadLocation("America/Chicago")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func periodOf(t time.Time) string {
	t = t.In(chicago)
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return PERIOD_WEEKEND
	}
	switch h := t.Hour(); {
	case h < 6:
		return PERIOD_OVERNIGHT
	case h < 9:
		return PERIOD_AM_PEAK
	case h < 15:
		return PERIOD_MIDDAY
	case h < 19:
		return PERIOD_PM_PEAK
	default:
		return PERIOD_EVENING
	}
}

type routeDirection struct {
	route     string
	direction string
}

type referenceStop struct {
	stopId   string
	stopName string
}

type groupResult struct {
	route     string
	direction string
	period    string
	days      int
	stats     HeadwayStats
}

type directionResult struct {
	route     string
	direction string
	stops     []referenceStop
	headways  []headway
}

type analysis struct {
	observations int
	synthetic    bool
	first        time.Time
	last         time.Time
	directions   []directionResult
	groups       []groupResult
}

func analyze(obs []Observation, patterns map[int]*PatternInfo, cfg analyzerConfig) (*analysis, error) {
	if len(obs) == 0 {
		return nil, fmt.Errorf("no observations")
	}
	a := &analysis{observations: len(obs), first: obs[0].Timestamp, last: obs[0].Timestamp}

	// Direction comes from the pattern file - observations logged before their pattern was fetched have none
	byDirection := make(map[routeDirection][]Observation)
	pollsByRoute := make(map[string][]time.Time)
	missing := make(map[int]bool)
	for _, o := range obs {
		a.synthetic = a.synthetic || o.Synthetic
		if o.Timestamp.Before(a.first) {
			a.first = o.Timestamp
		}
		if o.Timestamp.After(a.last) {
			a.last = o.Timestamp
		}
		pollsByRoute[o.Route] = append(pollsByRoute[o.Route], o.PolledAt)
		p, ok := patterns[o.PatternId]
		if !ok {
			missing[o.PatternId] = true
			continue
		}
		rd := routeDirection{o.Route, p.Direction}
		byDirection[rd] = append(byDirection[rd], o)
	}
	if len(missing) > 0 {
		slog.Warn("Ignoring observations on patterns missing from the pattern file", slog.Int("patterns", len(missing)))
	}

	gapsByRoute := make(map[string][]interval)
	for route, polls := range pollsByRoute {
		gapsByRoute[route] = coverageGaps(polls, cfg.maxGap)
	}

	rds := make([]routeDirection, 0, len(byDirection))
	for rd := range byDirection {
		rds = append(rds, rd)
	}
	slices.SortFunc(rds, func(x, y routeDirection) int {
		return cmp.Or(compareRoutes(x.route, y.route), strings.Compare(x.direction, y.direction))
	})

	for _, rd := range rds {
		dr := analyzeDirection(rd, byDirection[rd], patterns, gapsByRoute[rd.route], cfg)
		a.directions = append(a.directions, dr)

		byPeriod := make(map[string][]headway)
		for _, h := range dr.headways {
			p := periodOf(h.at)
			byPeriod[p] = append(byPeriod[p], h)
		}
		for _, period := range periodOrder {
			hs := byPeriod[period]
			if len(hs) == 0 {
				continue
			}
			days := make(map[string]bool)
			for _, h := range hs {
				days[h.at.In(chicago).Format(time.DateOnly)] = true
			}
			a.groups = append(a.groups, groupResult{
				route:     rd.route,
				direction: rd.direction,
				period:    period,
				days:      len(days),
				stats:     computeStats(chains(rd.route, rd.direction, hs), bunchinglib.Rule{Minutes: cfg.bunchMinutes, Fraction: cfg.bunchFraction}),
			})
		}
	}
	return a, nil
}

// Routes sort numerically where possible so 6 comes before 55
func compareRoutes(a, b string) int {
	var x, y int
	_, errA := fmt.Sscanf(a, "%d", &x)
	_, errB := fmt.Sscanf(b, "%d", &y)
	if errA == nil && errB == nil && x != y {
		return cmp.Compare(x, y)
	}
	return strings.Compare(a, b)
}

func analyzeDirection(rd routeDirection, obs []Observation, patterns map[int]*PatternInfo, gaps []interval, cfg analyzerConfig) directionResult {
	dr := directionResult{route: rd.route, direction: rd.direction}

	// The pattern with the most observations defines the reference stops
	counts := make(map[int]int)
	for _, o := range obs {
		counts[o.PatternId]++
	}
	mainPid := 0
	for pid, c := range counts {
		if c > counts[mainPid] || (c == counts[mainPid] && pid < mainPid) {
			mainPid = pid
		}
	}
	dr.stops = pickReferenceStops(patterns[mainPid], cfg.stops)

	runs := buildRuns(obs, cfg.maxInterp)
	for _, stop := range dr.stops {
		// Pool buses on every pattern that serves this stop - a short-turn bus still picks people up
		var crossings []time.Time
		for _, r := range runs {
			d, ok := patterns[r[0].PatternId].stopDistance(stop.stopId)
			if !ok {
				continue
			}
			if t, ok := r.crossing(d); ok {
				crossings = append(crossings, t)
			}
		}
		dr.headways = append(dr.headways, headways(stop.stopId, crossings, gaps)...)
	}
	return dr
}

// Picks n stops spread evenly along the pattern, at 1/(2n), 3/(2n), ... of its length.
// Staying off the very ends avoids terminals, where buses lay over and "headways" mean something else.
func pickReferenceStops(p *PatternInfo, n int) []referenceStop {
	if p == nil || len(p.Stops) == 0 || n <= 0 {
		return nil
	}
	length := p.LengthFeet
	if length == 0 {
		length = p.Stops[len(p.Stops)-1].PatternDistance
	}
	var picked []referenceStop
	seen := make(map[string]bool)
	for i := 0; i < n; i++ {
		target := length * (float64(i) + 0.5) / float64(n)
		best := p.Stops[0]
		for _, s := range p.Stops {
			if math.Abs(s.PatternDistance-target) < math.Abs(best.PatternDistance-target) {
				best = s
			}
		}
		if best.PatternDistance <= 0 || seen[best.StopId] {
			continue
		}
		seen[best.StopId] = true
		picked = append(picked, referenceStop{stopId: best.StopId, stopName: best.StopName})
	}
	return picked
}
