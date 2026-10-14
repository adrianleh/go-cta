package main

import (
	"context"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"adrianlehmann.net/cta"
)

// Everything the site shows comes through here. We only call Bus Tracker for routes someone is looking at,
// and cache results so a room full of viewers costs the same as one (the API has a daily request limit).
const (
	LIVE_CACHE_FOR     = 30 * time.Second
	PATTERNS_CACHE_FOR = 6 * time.Hour
	ROUTES_CACHE_FOR   = 24 * time.Hour
	REQUEST_TIMEOUT    = 20 * time.Second
)

type liveRoute struct {
	Route      string
	Name       string
	FetchedAt  time.Time
	Synthetic  bool
	Directions []liveDirection
	Unplaced   int // buses on a pattern we don't know
}

type routePatterns struct {
	refs        map[string]*refPattern // reference pattern per direction
	directionOf map[int]string         // pid -> direction, for every pattern of the route
	fetchedAt   time.Time
}

type cachedLive struct {
	live      *liveRoute // last good result, may be older than fetchedAt
	err       error
	fetchedAt time.Time
}

type liveSource struct {
	client *cta.BusClient // nil means synthetic buses

	// One lock for everything keeps this simple. Concurrent viewers of a route wait for the one fetch in flight.
	mu       sync.Mutex
	routes   []cta.Route
	routesAt time.Time
	patterns map[string]*routePatterns
	live     map[string]*cachedLive
}

func newLiveSource(client *cta.BusClient) *liveSource {
	return &liveSource{
		client:   client,
		patterns: make(map[string]*routePatterns),
		live:     make(map[string]*cachedLive),
	}
}

func (s *liveSource) getRoutes(ctx context.Context) ([]cta.Route, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.routes != nil && time.Since(s.routesAt) < ROUTES_CACHE_FOR {
		return s.routes, nil
	}
	if s.client == nil {
		s.routes = []cta.Route{synthRoute()}
		s.routesAt = time.Now()
		return s.routes, nil
	}
	ctx, cancel := context.WithTimeout(ctx, REQUEST_TIMEOUT)
	defer cancel()
	routes, err := s.client.GetRoutes(ctx)
	if err != nil {
		return nil, err
	}
	s.routes = *routes
	s.routesAt = time.Now()
	return s.routes, nil
}

func (s *liveSource) routeName(ctx context.Context, rt string) string {
	routes, err := s.getRoutes(ctx)
	if err != nil {
		return ""
	}
	for _, r := range routes {
		if r.RouteId == rt {
			return r.RouteName
		}
	}
	return ""
}

// Returns the live view of a route, at most LIVE_CACHE_FOR old. If the latest fetch failed you get the
// last good view (if any) together with the error.
func (s *liveSource) getLive(ctx context.Context, rt string) (*liveRoute, error) {
	name := s.routeName(ctx, rt)

	s.mu.Lock()
	defer s.mu.Unlock()
	cached := s.live[rt]
	if cached != nil && time.Since(cached.fetchedAt) < LIVE_CACHE_FOR {
		return cached.live, cached.err
	}
	if cached == nil {
		cached = &cachedLive{}
		s.live[rt] = cached
	}
	cached.fetchedAt = time.Now()

	live, err := s.fetchLive(ctx, rt)
	cached.err = err
	if err != nil {
		slog.Error("Fetching live route", slog.String("route", rt), slog.Any("err", err))
		return cached.live, err
	}
	live.Name = name
	cached.live = live
	return live, nil
}

func (s *liveSource) fetchLive(ctx context.Context, rt string) (*liveRoute, error) {
	ctx, cancel := context.WithTimeout(ctx, REQUEST_TIMEOUT)
	defer cancel()

	vehicles, err := s.fetchVehicles(ctx, rt)
	if err != nil {
		return nil, err
	}
	patterns, err := s.getPatterns(ctx, rt, vehicles)
	if err != nil {
		return nil, err
	}
	live := buildLiveRoute(rt, patterns, vehicles)
	live.Synthetic = s.client == nil
	return live, nil
}

func (s *liveSource) fetchVehicles(ctx context.Context, rt string) ([]cta.Vehicle, error) {
	if s.client == nil {
		return synthVehicles(time.Now()), nil
	}
	vehicles, err := s.client.GetVehiclesByRoute(ctx, []string{rt})
	// The API reports "no buses right now" as an error
	if err != nil && strings.Contains(err.Error(), "No data found") {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return *vehicles, nil
}

// Patterns barely change, so they're cached for hours - unless a bus shows up on one we don't know (e.g. a new detour)
func (s *liveSource) getPatterns(ctx context.Context, rt string, vehicles []cta.Vehicle) (*routePatterns, error) {
	cached := s.patterns[rt]
	if cached != nil && time.Since(cached.fetchedAt) < PATTERNS_CACHE_FOR {
		stale := slices.ContainsFunc(vehicles, func(v cta.Vehicle) bool {
			_, known := cached.directionOf[v.PatternId]
			return !known
		})
		if !stale || time.Since(cached.fetchedAt) < 10*time.Minute {
			return cached, nil
		}
	}

	var patterns []cta.Pattern
	if s.client == nil {
		patterns = synthPatterns()
	} else {
		ps, err := s.client.GetPatternsByRoute(ctx, rt)
		if err != nil {
			return nil, err
		}
		patterns = *ps
	}

	rp := &routePatterns{
		refs:        referencePatterns(patterns),
		directionOf: make(map[int]string),
		fetchedAt:   time.Now(),
	}
	for _, p := range patterns {
		rp.directionOf[p.Pid] = p.RouteDirection
	}
	s.patterns[rt] = rp
	return rp, nil
}

func buildLiveRoute(rt string, patterns *routePatterns, vehicles []cta.Vehicle) *liveRoute {
	live := &liveRoute{Route: rt, FetchedAt: time.Now()}
	busesByDirection := make(map[string][]liveBus)
	for _, v := range vehicles {
		ref := patterns.refs[patterns.directionOf[v.PatternId]]
		if ref == nil {
			live.Unplaced++
			continue
		}
		at, ok := ref.place(v)
		if !ok {
			live.Unplaced++
			continue
		}
		busesByDirection[ref.direction] = append(busesByDirection[ref.direction], liveBus{
			VehicleId:   v.Id,
			Destination: v.Destination,
			PositionFt:  at,
			SpeedMph:    v.Speed,
		})
	}

	directions := make([]string, 0, len(patterns.refs))
	for dir := range patterns.refs {
		directions = append(directions, dir)
	}
	slices.Sort(directions)
	for _, dir := range directions {
		live.Directions = append(live.Directions, buildLiveDirection(patterns.refs[dir], busesByDirection[dir]))
	}
	return live
}

// "6" or "6,J14" -> ["6", "J14"], at most 4 routes per page
func parseRouteList(s string) []string {
	var routes []string
	for _, r := range strings.Split(s, ",") {
		r = strings.TrimSpace(r)
		if r != "" && len(routes) < 4 && !slices.Contains(routes, r) {
			routes = append(routes, r)
		}
	}
	return routes
}

func formatMinutes(m float64) string {
	return strconv.FormatFloat(m, 'f', 1, 64)
}
