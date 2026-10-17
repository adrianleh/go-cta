package main

import (
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Direction 1 has gaps 1 and 19 (a bunched pair), direction 2 has gaps 10 and 10
func twoDirectionRoute() *liveRoute {
	const minute = 880.0
	one := buildLiveDirection(straightRef(40*minute), []liveBus{
		{VehicleId: "a", PositionFt: 30 * minute, SpeedMph: 10, Latitude: 41.8, Longitude: -87.6},
		{VehicleId: "b", PositionFt: 29 * minute, SpeedMph: 10, Latitude: 41.79, Longitude: -87.6},
		{VehicleId: "c", PositionFt: 10 * minute, SpeedMph: 10, Latitude: 41.7, Longitude: -87.6},
	})
	two := buildLiveDirection(straightRef(40*minute), []liveBus{
		{VehicleId: "d", PositionFt: 30 * minute, SpeedMph: 10},
		{VehicleId: "e", PositionFt: 20 * minute, SpeedMph: 10},
		{VehicleId: "f", PositionFt: 10 * minute, SpeedMph: 10},
	})
	return &liveRoute{Route: "6", Name: "Jackson Park Express", Directions: []liveDirection{one, two}}
}

func TestSummarizePoolsDirections(t *testing.T) {
	s := summarize(twoDirectionRoute())
	if s.Buses != 6 || s.BunchedBuses != 2 || s.Gaps != 4 {
		t.Errorf("unexpected counts %+v", s)
	}
	// Pooled gaps 1, 19, 10, 10: sum 40, sum of squares 562
	if !almostEqual(s.Wait, 562.0/(2*40)) {
		t.Errorf("expected pooled wait %f, got %f", 562.0/80, s.Wait)
	}
	if !almostEqual(s.WaitEven, 5) {
		t.Errorf("expected even wait 5, got %f", s.WaitEven)
	}
	if !almostEqual(s.Worst, 19) {
		t.Errorf("expected worst 19, got %f", s.Worst)
	}
}

func TestSummarizeTooFewBuses(t *testing.T) {
	const minute = 880.0
	d := buildLiveDirection(straightRef(40*minute), []liveBus{
		{VehicleId: "a", PositionFt: 30 * minute}, {VehicleId: "b", PositionFt: 20 * minute},
	})
	s := summarize(&liveRoute{Route: "9", Directions: []liveDirection{d}})
	if s.Buses != 2 || s.Gaps != 1 || !math.IsNaN(s.Wait) || !math.IsNaN(s.WaitEven) || s.Worst != d.Gaps[0].Minutes {
		t.Errorf("unexpected summary %+v", s)
	}
	if empty := summarize(&liveRoute{Route: "9"}); empty.Buses != 0 || !math.IsNaN(empty.Worst) {
		t.Errorf("unexpected empty summary %+v", empty)
	}
}

func TestConvoys(t *testing.T) {
	cs := convoys(twoDirectionRoute())
	if len(cs) != 1 {
		t.Fatalf("expected 1 convoy, got %+v", cs)
	}
	c := cs[0]
	if c.Route != "6" || c.Name != "Jackson Park Express" || c.Buses != 2 || c.Gaps != "~1.0" || len(c.Pos) != 2 || c.Pos[0] != [2]float64{41.8, -87.6} {
		t.Errorf("unexpected convoy %+v", c)
	}
}

func TestConvoyOfThreeIsOneClusterButTwoPairs(t *testing.T) {
	const minute = 880.0
	// Gaps 1, 1, 18, 1: the first three buses are a convoy, the last two a pair, and 20 is far from both
	d := buildLiveDirection(straightRef(60*minute), []liveBus{
		{VehicleId: "a", PositionFt: 50 * minute, SpeedMph: 10, Latitude: 41.8, Longitude: -87.6},
		{VehicleId: "b", PositionFt: 49 * minute, SpeedMph: 10, Latitude: 41.79, Longitude: -87.6},
		{VehicleId: "c", PositionFt: 48 * minute, SpeedMph: 10, Latitude: 41.78, Longitude: -87.6},
		{VehicleId: "d", PositionFt: 30 * minute, SpeedMph: 10, Latitude: 41.7, Longitude: -87.6},
		{VehicleId: "e", PositionFt: 29 * minute, SpeedMph: 10, Latitude: 41.69, Longitude: -87.6},
	})
	if runs := d.Convoys; len(runs) != 2 || runs[0] != [2]int{0, 2} || runs[1] != [2]int{3, 4} {
		t.Fatalf("unexpected runs %v (gaps %+v)", runs, d.Gaps)
	}
	live := &liveRoute{Route: "6", Directions: []liveDirection{d}}
	if s := summarize(live); s.BunchedBuses != 5 {
		t.Errorf("expected 5 buses in 2 convoys, got %+v", s)
	}
	cs := convoys(live)
	if len(cs) != 2 || cs[0].Buses != 3 || len(cs[0].Pos) != 3 || cs[0].Gaps != "~1.0, ~1.0" || cs[1].Buses != 2 {
		t.Errorf("unexpected convoys %+v", cs)
	}
	// A bus without a position is left off the line, and a convoy with fewer than 2 positions isn't drawn at all
	live.Directions[0].Buses[1].Latitude = 0
	if cs := convoys(live); cs[0].Buses != 3 || len(cs[0].Pos) != 2 {
		t.Errorf("expected the line to skip the bus without a position, got %+v", cs[0])
	}
	live.Directions[0].Buses[0].Latitude = 0
	if cs := convoys(live); len(cs) != 1 || cs[0].Buses != 2 {
		t.Errorf("expected only the second convoy to be drawn, got %+v", cs)
	}
}

func TestSortSummaries(t *testing.T) {
	rs := []routeSummary{
		{Route: "A", Buses: 3, BunchedBuses: 0, Wait: 4, Worst: 9},
		{Route: "B", Buses: 9, BunchedBuses: 4, Wait: math.NaN(), Worst: math.NaN()},
		{Route: "C", Buses: 5, BunchedBuses: 5, Wait: 12, Worst: 30},
		{Route: "D", Buses: 5, BunchedBuses: 3, Wait: 8, Worst: 20},
	}
	order := func(by string) string {
		sorted := append([]routeSummary(nil), rs...)
		sortSummaries(sorted, by)
		var s string
		for _, r := range sorted {
			s += r.Route
		}
		return s
	}
	for by, want := range map[string]string{
		"route":   "ACDB", // B has no estimate, so it goes last in every sort
		"bunched": "CDAB", // most buses in bunched clusters first
		"buses":   "CDAB", // ties keep the route order
		"wait":    "CDAB",
		"worst":   "CDAB",
		"factor":  "CDAB", // C and D are 2.5x, A is 2.25x
		"bogus":   "ACDB",
	} {
		if got := order(by); got != want {
			t.Errorf("sort by %s: expected %s, got %s", by, want, got)
		}
	}
}

func TestOverviewViewFormatting(t *testing.T) {
	if v := newOverviewView(overviewStatus{Refreshing: true}, ""); !v.Loading || !v.Refreshing {
		t.Errorf("expected a loading view, got %+v", v)
	}
	snap := &overview{
		At:     time.Now(),
		Routes: []routeSummary{summarize(twoDirectionRoute()), {Route: "9", Buses: 1, Wait: math.NaN(), WaitEven: math.NaN(), Worst: math.NaN()}},
	}
	v := newOverviewView(overviewStatus{Snap: snap}, "")
	if v.Sort != DEFAULT_OVERVIEW_SORT || len(v.Rows) != 2 {
		t.Fatalf("unexpected view %+v", v)
	}
	if r := v.Rows[0]; r.Route != "6" || r.Wait != "7.0" || r.WaitEven != "5.0" || r.Worst != "19.0" || r.Factor != "2.7×" || r.Bunched != 2 {
		t.Errorf("unexpected row %+v", r)
	}
	if r := v.Rows[1]; r.Wait != "-" || r.WaitEven != "-" || r.Worst != "-" || r.Factor != "-" {
		t.Errorf("expected dashes without an estimate, got %+v", r)
	}
}

func TestOverviewBuild(t *testing.T) {
	o := newOverviewSource(newLiveSource(nil), time.Minute)
	snap := o.build(t.Context())
	if len(snap.Routes) != 1 || snap.Routes[0].Route != SYNTH_ROUTE || snap.Routes[0].Buses == 0 || snap.Failed != 0 {
		t.Errorf("unexpected snapshot %+v", snap)
	}
}

func TestOverviewServer(t *testing.T) {
	srv := httptest.NewServer(newServer(newLiveSource(nil), time.Minute))
	defer srv.Close()
	get := func(path string) string {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var body strings.Builder
		_, _ = io.Copy(&body, resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status %d", path, resp.StatusCode)
		}
		return body.String()
	}

	// The first request starts the refresh and gets the loading page; the snapshot shows up shortly after
	if body := get("/"); !strings.Contains(body, `id="map"`) {
		t.Errorf("expected the map on the overview page")
	}
	deadline := time.Now().Add(5 * time.Second)
	body := get("/?sort=wait")
	for !strings.Contains(body, `class="overview"`) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		body = get("/?sort=wait")
	}
	if !strings.Contains(body, `class="overview"`) || !strings.Contains(body, "/route/") || !strings.Contains(body, `id="q"`) {
		t.Errorf("expected the table once the snapshot is ready, got %s", body)
	}
	if !strings.Contains(get("/overview.json"), `"convoys"`) || !strings.Contains(get("/overview-map.js"), "L.map") {
		t.Errorf("expected overview data and script")
	}
}
