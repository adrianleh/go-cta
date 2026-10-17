package main

import (
	"cmp"
	"context"
	"log/slog"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"adrianlehmann.net/cta/example/bunchinglib"
)

// The all-routes view: one row per route with both directions pooled, and every bunched pair on one map.
// Fetching every route costs a couple hundred Bus Tracker requests, so it's done lazily: only when someone has
// the page open and the last snapshot is older than OVERVIEW_REFRESH_EVERY. Everyone else is served the snapshot.
const OVERVIEW_REFRESH_EVERY = 10 * time.Minute

// A route with both directions pooled. Waits are in minutes, NaN when there are too few buses to say.
type routeSummary struct {
	Route        string
	Name         string
	Buses        int // in service
	BunchedBuses int // buses in a bunched cluster: three in a row count 3, not 2 pairs
	Gaps         int
	Estimated    int // gaps from distance rather than predictions
	Wait         float64
	WaitEven     float64
	Worst        float64
}

// A convoy on the network map: two or more buses in a row, each bunched with the one ahead
type networkConvoy struct {
	Route     string       `json:"route"`
	Name      string       `json:"name"`
	Direction string       `json:"direction"`
	Buses     int          `json:"buses"`
	Gaps      string       `json:"gaps"` // front to back, e.g. "1.6, 0.8"
	Pos       [][2]float64 `json:"pos"`  // lat, lon of each bus, front to back
}

type overview struct {
	At      time.Time
	Routes  []routeSummary // routes with at least one bus in service, in route list order
	Convoys []networkConvoy
	Idle    int // routes with no bus in service
	Failed  int // routes Bus Tracker didn't answer for
}

// Pools both directions: a rider on a route waits in one direction, so the wait is taken over all gaps of the
// route together (as if a rider picks a gap at random) rather than averaging two per-direction numbers.
func summarize(live *liveRoute) routeSummary {
	s := routeSummary{Route: live.Route, Name: live.Name, Wait: math.NaN(), WaitEven: math.NaN(), Worst: math.NaN()}
	var minutes []float64
	for _, d := range live.Directions {
		s.Buses += d.InService
		for _, run := range d.Convoys {
			s.BunchedBuses += run[1] - run[0] + 1
		}
		s.Estimated += d.Estimated
		for _, g := range d.Gaps {
			minutes = append(minutes, g.Minutes)
		}
	}
	s.Gaps = len(minutes)
	if len(minutes) > 0 {
		s.Worst = slices.Max(minutes)
	}
	if headways := bunchinglib.Headways(live.Route, "", minutes); headways.Count() >= 2 {
		s.Wait = headways.ExpectedWait().Minutes()
		s.WaitEven = headways.EvenSpacingWait().Minutes()
	}
	return s
}

func convoys(live *liveRoute) []networkConvoy {
	var out []networkConvoy
	for _, d := range live.Directions {
		for _, run := range d.Convoys {
			c := networkConvoy{Route: live.Route, Name: live.Name, Direction: d.Direction, Buses: run[1] - run[0] + 1}
			gaps := make([]string, 0, run[1]-run[0])
			for _, g := range d.Gaps[run[0]:run[1]] {
				gaps = append(gaps, formatGap(g))
			}
			c.Gaps = strings.Join(gaps, ", ")
			for _, b := range d.Buses[run[0] : run[1]+1] {
				if b.Latitude != 0 { // no position from the API
					c.Pos = append(c.Pos, [2]float64{b.Latitude, b.Longitude})
				}
			}
			if len(c.Pos) >= 2 {
				out = append(out, c)
			}
		}
	}
	return out
}

// How many times the average wait the longest wait is, NaN without an estimate
func (s routeSummary) worstFactor() float64 {
	if math.IsNaN(s.Wait) || s.Wait == 0 {
		return math.NaN()
	}
	return s.Worst / s.Wait
}

// NaN (no estimate) always goes last
func compareWaits(a, b float64, descending bool) int {
	switch {
	case math.IsNaN(a) && math.IsNaN(b):
		return 0
	case math.IsNaN(a):
		return 1
	case math.IsNaN(b):
		return -1
	case descending:
		return cmp.Compare(b, a)
	}
	return cmp.Compare(a, b)
}

const DEFAULT_OVERVIEW_SORT = "bunched"

// "route" keeps the Bus Tracker order; every other column puts the worst routes first.
// Routes without a wait estimate (shown as "-") always go last, whatever the sort.
func sortSummaries(rs []routeSummary, by string) {
	var f func(a, b routeSummary) int
	switch by {
	case "buses":
		f = func(a, b routeSummary) int { return cmp.Compare(b.Buses, a.Buses) }
	case "bunched":
		f = func(a, b routeSummary) int { return cmp.Compare(b.BunchedBuses, a.BunchedBuses) }
	case "wait":
		f = func(a, b routeSummary) int { return compareWaits(a.Wait, b.Wait, true) }
	case "even":
		f = func(a, b routeSummary) int { return compareWaits(a.WaitEven, b.WaitEven, true) }
	case "worst":
		f = func(a, b routeSummary) int { return compareWaits(a.Worst, b.Worst, true) }
	case "factor":
		f = func(a, b routeSummary) int { return compareWaits(a.worstFactor(), b.worstFactor(), true) }
	}
	if f != nil {
		slices.SortStableFunc(rs, f)
	}
	slices.SortStableFunc(rs, func(a, b routeSummary) int {
		return cmp.Compare(boolToInt(math.IsNaN(a.worstFactor())), boolToInt(math.IsNaN(b.worstFactor())))
	})
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

type overviewSource struct {
	source *liveSource
	every  time.Duration

	mu         sync.Mutex
	snap       *overview
	refreshing bool
	done       int // routes fetched so far in the refresh under way
	total      int
}

// What the page needs to show: the snapshot and, while a refresh is under way, how far along it is
type overviewStatus struct {
	Snap       *overview // nil until the first refresh finishes
	Refreshing bool
	Done       int
	Total      int
}

func newOverviewSource(source *liveSource, every time.Duration) *overviewSource {
	return &overviewSource{source: source, every: every}
}

// The latest snapshot. Starts a refresh in the background if it's stale.
func (o *overviewSource) get() overviewStatus {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.refreshing && (o.snap == nil || time.Since(o.snap.At) >= o.every) {
		o.refreshing = true
		o.done, o.total = 0, 0
		go o.refresh(context.Background())
	}
	return overviewStatus{Snap: o.snap, Refreshing: o.refreshing, Done: o.done, Total: o.total}
}

func (o *overviewSource) refresh(ctx context.Context) {
	start := time.Now()
	snap := o.build(ctx)
	slog.Info("Refreshed overview", slog.Int("routes", len(snap.Routes)), slog.Int("idle", snap.Idle),
		slog.Int("failed", snap.Failed), slog.Duration("took", time.Since(start)))
	o.mu.Lock()
	o.snap = snap
	o.refreshing = false
	o.mu.Unlock()
}

func (o *overviewSource) setProgress(done, total int) {
	o.mu.Lock()
	o.done, o.total = done, total
	o.mu.Unlock()
}

// One route at a time through getLive: gentle on the API, and route pages never wait behind more than one fetch
func (o *overviewSource) build(ctx context.Context) *overview {
	snap := &overview{At: time.Now()}
	routes, err := o.source.getRoutes(ctx)
	if err != nil {
		slog.Error("Getting routes for overview", slog.Any("err", err))
		snap.Failed = 1
		return snap
	}
	o.setProgress(0, len(routes))
	for i, r := range routes {
		live, err := o.source.getLive(ctx, r.RouteId)
		o.setProgress(i+1, len(routes))
		if err != nil {
			snap.Failed++
		}
		if live == nil {
			continue
		}
		s := summarize(live)
		if s.Buses == 0 {
			snap.Idle++
			continue
		}
		snap.Routes = append(snap.Routes, s)
		snap.Convoys = append(snap.Convoys, convoys(live)...)
	}
	return snap
}
