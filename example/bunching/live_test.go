package main

import (
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"adrianlehmann.net/cta"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestExpectedWait(t *testing.T) {
	// Evenly spaced buses: wait is half the gap
	if w := expectedWait([]float64{10, 10, 10}); !almostEqual(w, 5) {
		t.Errorf("expected 5, got %f", w)
	}
	// Same two buses per 20 minutes but bunched: E[H^2] = (1 + 361) / 2 = 181, E[H] = 10, W = 9.05
	if w := expectedWait([]float64{1, 19}); !almostEqual(w, 9.05) {
		t.Errorf("expected 9.05, got %f", w)
	}
	if !math.IsNaN(expectedWait(nil)) {
		t.Errorf("expected NaN for no gaps")
	}
}

func TestBunchThreshold(t *testing.T) {
	// Frequent route: the 2 minute rule wins
	if th := bunchThreshold(6); th != 2 {
		t.Errorf("expected 2, got %f", th)
	}
	// Infrequent route: 25% of a 20 minute median is 5 minutes
	if th := bunchThreshold(20); th != 5 {
		t.Errorf("expected 5, got %f", th)
	}
	if th := bunchThreshold(math.NaN()); th != 2 {
		t.Errorf("expected 2 without a median, got %f", th)
	}
}

func TestDirectionSpeed(t *testing.T) {
	if s := directionSpeed([]int{0, 12, 15, 9}); s != 9 {
		t.Errorf("expected measured 9 mph, got %f", s)
	}
	if s := directionSpeed([]int{12, 15}); s != DEFAULT_SPEED_MPH {
		t.Errorf("expected default with too few buses, got %f", s)
	}
	if s := directionSpeed([]int{0, 0, 0, 0}); s != DEFAULT_SPEED_MPH {
		t.Errorf("expected default when nobody reports a speed, got %f", s)
	}
	if s := directionSpeed([]int{60, 60, 60}); s != MAX_SPEED_MPH {
		t.Errorf("expected speed capped at %d, got %f", MAX_SPEED_MPH, s)
	}
}

func straightRef(lengthFt float64) *refPattern {
	return &refPattern{
		direction: "Northbound",
		lengthFt:  lengthFt,
		stops:     []refStop{{name: "Start", atFt: 0}, {name: "End", atFt: lengthFt}},
	}
}

func TestBuildLiveDirection(t *testing.T) {
	// At 10 mph a minute is 880 ft
	const minute = 880.0
	buses := []liveBus{
		{VehicleId: "c", PositionFt: 10 * minute, SpeedMph: 10},
		{VehicleId: "a", PositionFt: 30 * minute, SpeedMph: 10},
		{VehicleId: "b", PositionFt: 29 * minute, SpeedMph: 10}, // 1 minute behind a: bunched
		{VehicleId: "d", PositionFt: 100, SpeedMph: 0},          // laying over at the start
		{VehicleId: "e", PositionFt: 2 * minute, SpeedMph: 10},  // just out of the terminal zone
	}
	d := buildLiveDirection(straightRef(40*minute), buses)

	if d.InService != 4 {
		t.Fatalf("expected 4 buses in service, got %d", d.InService)
	}
	order := ""
	for _, b := range d.Buses {
		order += b.VehicleId
	}
	if order != "abced" {
		t.Errorf("expected front to back order abce then terminal d, got %s", order)
	}
	if d.SpeedMph != 10 {
		t.Errorf("expected measured 10 mph, got %f", d.SpeedMph)
	}

	want := []float64{1, 19, 8}
	if len(d.Gaps) != len(want) {
		t.Fatalf("expected %d gaps, got %d", len(want), len(d.Gaps))
	}
	for i, w := range want {
		if !almostEqual(d.Gaps[i].Minutes, w) {
			t.Errorf("gap %d: expected %f min, got %f", i, w, d.Gaps[i].Minutes)
		}
	}
	if d.BunchedPairs != 1 || !d.Gaps[0].Bunched {
		t.Errorf("expected only the first gap bunched, got %+v", d.Gaps)
	}
	if !almostEqual(d.ExpectedWait, (1+19*19+8*8)/(2*28.0)) {
		t.Errorf("unexpected expected wait %f", d.ExpectedWait)
	}
	if !almostEqual(d.EvenSpacingWait, 28.0/3/2) {
		t.Errorf("unexpected even spacing wait %f", d.EvenSpacingWait)
	}
}

var now = time.Date(2026, time.October, 14, 17, 0, 0, 0, chicago)

func predictionsAt(stops ...any) []stopPrediction {
	// Arguments alternate stop id and minutes from now: "s1", 1.0, "s2", 3.5, ...
	var ps []stopPrediction
	for i := 0; i < len(stops); i += 2 {
		ps = append(ps, stopPrediction{StopId: stops[i].(string), At: now.Add(time.Duration(stops[i+1].(float64) * float64(time.Minute)))})
	}
	return ps
}

func TestPredictedGap(t *testing.T) {
	// A is next at stop 2 in 1 minute, B gets there in 4.5 minutes
	ahead := predictionsAt("2", 1.0, "3", 3.0, "4", 6.0)
	behind := predictionsAt("1", 0.5, "2", 4.5, "3", 7.0)
	if gap, ok := predictedGap(ahead, behind); !ok || !almostEqual(gap, 3.5) {
		t.Errorf("expected 3.5 min at A's next stop, got %f (%v)", gap, ok)
	}
	// B's predictions don't reach A's next stop, but do reach a later one
	ahead = predictionsAt("2", 1.0, "3", 3.0)
	behind = predictionsAt("0", 1.0, "3", 9.0)
	if gap, ok := predictedGap(ahead, behind); !ok || !almostEqual(gap, 6) {
		t.Errorf("expected 6 min at stop 3, got %f (%v)", gap, ok)
	}
	// No shared stop: the gap is longer than the prediction horizon
	if _, ok := predictedGap(predictionsAt("5", 1.0), predictionsAt("1", 2.0)); ok {
		t.Errorf("expected no gap without a shared stop")
	}
	// B predicted at the stop before A: stale prediction, don't trust it
	if _, ok := predictedGap(predictionsAt("2", 5.0), predictionsAt("2", 4.0)); ok {
		t.Errorf("expected no gap when the predictions contradict the order")
	}
	if _, ok := predictedGap(nil, predictionsAt("2", 4.0)); ok {
		t.Errorf("expected no gap without predictions")
	}
}

func TestBuildLiveDirectionPredictionsAndFallback(t *testing.T) {
	const minute = 880.0 // at 10 mph
	buses := []liveBus{
		{VehicleId: "a", PositionFt: 30 * minute, SpeedMph: 10, Predictions: predictionsAt("s31", 1.0, "s32", 2.0)},
		// 10 minutes behind a by distance, but predicted 1.5 minutes behind at s31: traffic ahead of a, bunched
		{VehicleId: "b", PositionFt: 20 * minute, SpeedMph: 10, Predictions: predictionsAt("s25", 1.0, "s31", 2.5)},
		// Too far behind b to share a predicted stop: estimated from distance, 15 minutes
		{VehicleId: "c", PositionFt: 5 * minute, SpeedMph: 10, Predictions: predictionsAt("s6", 1.0)},
	}
	d := buildLiveDirection(straightRef(40*minute), buses)
	if len(d.Gaps) != 2 {
		t.Fatalf("expected 2 gaps, got %d", len(d.Gaps))
	}
	first, second := d.Gaps[0], d.Gaps[1]
	if first.Estimated || !almostEqual(first.Minutes, 1.5) || !first.Bunched || first.Ahead != "a" || first.Behind != "b" {
		t.Errorf("unexpected first gap %+v", first)
	}
	if !second.Estimated || !almostEqual(second.Minutes, 15) || second.Bunched {
		t.Errorf("unexpected second gap %+v", second)
	}
	if d.Estimated != 1 || d.BunchedPairs != 1 {
		t.Errorf("expected 1 estimated and 1 bunched, got %d and %d", d.Estimated, d.BunchedPairs)
	}
	// Same wait formula on the mixed gaps
	if !almostEqual(d.ExpectedWait, (1.5*1.5+15*15)/(2*16.5)) {
		t.Errorf("unexpected expected wait %f", d.ExpectedWait)
	}
	if d.WorstGap.Behind != "c" || formatGap(d.WorstGap) != "~15.0" {
		t.Errorf("expected the 15 minute estimated gap as worst case marked with ~, got %+v", d.WorstGap)
	}
	if formatGap(d.Gaps[0]) != "1.5" || !d.Gaps[0].Bunched {
		t.Errorf("expected the predicted gap unmarked and bunched, got %+v", d.Gaps[0])
	}
}

func TestMapData(t *testing.T) {
	const minute = 880.0
	buses := []liveBus{
		{VehicleId: "a", PositionFt: 30 * minute, Latitude: 41.8, Longitude: -87.6, Predictions: predictionsAt("x", 1.0)},
		{VehicleId: "b", PositionFt: 29 * minute, Latitude: 41.79, Longitude: -87.6, Predictions: predictionsAt("x", 2.0)},
		{VehicleId: "t", PositionFt: 0},
	}
	r := &liveRoute{Route: "6", Directions: []liveDirection{buildLiveDirection(straightRef(40*minute), buses)}}
	md := mapData(r)[0]
	if len(md.Buses) != 3 || len(md.Convoys) != 1 {
		t.Fatalf("unexpected map data %+v", md)
	}
	a, b, term := md.Buses[0], md.Buses[1], md.Buses[2]
	// Both buses of the pair are in the cluster, including the lead one
	if !a.Bunched || a.Convoy != 2 || a.BunchedAhead || a.GapAhead != "" || a.GapBehind != "1.0" {
		t.Errorf("unexpected lead bus %+v", a)
	}
	if !b.Bunched || b.Convoy != 2 || !b.BunchedAhead || b.GapAhead != "1.0" || b.GapBehind != "" {
		t.Errorf("unexpected second bus %+v", b)
	}
	if !term.AtTerminal || term.Bunched || term.Convoy != 0 || term.GapAhead != "" {
		t.Errorf("unexpected terminal bus %+v", term)
	}
	if len(md.Convoys[0]) != 2 || md.Convoys[0][0] != [2]float64{41.8, -87.6} || md.Convoys[0][1] != [2]float64{41.79, -87.6} {
		t.Errorf("unexpected convoy line %v", md.Convoys[0])
	}
}

// Gaps 1, 1, 18, 1: buses a, b, c in a row, then d and e a pair
func threeThenTwo() liveDirection {
	const minute = 880.0
	return buildLiveDirection(straightRef(60*minute), []liveBus{
		{VehicleId: "a", PositionFt: 50 * minute, SpeedMph: 10, Latitude: 41.8, Longitude: -87.6},
		{VehicleId: "b", PositionFt: 49 * minute, SpeedMph: 10, Latitude: 41.79, Longitude: -87.6},
		{VehicleId: "c", PositionFt: 48 * minute, SpeedMph: 10, Latitude: 41.78, Longitude: -87.6},
		{VehicleId: "d", PositionFt: 30 * minute, SpeedMph: 10, Latitude: 41.7, Longitude: -87.6},
		{VehicleId: "e", PositionFt: 29 * minute, SpeedMph: 10, Latitude: 41.69, Longitude: -87.6},
	})
}

func TestMapDataConvoyOfThree(t *testing.T) {
	md := mapData(&liveRoute{Route: "6", Directions: []liveDirection{threeThenTwo()}})[0]
	if len(md.Convoys) != 2 || len(md.Convoys[0]) != 3 || len(md.Convoys[1]) != 2 {
		t.Fatalf("expected one line through 3 buses and one through 2, got %v", md.Convoys)
	}
	want := []int{3, 3, 3, 2, 2}
	for i, b := range md.Buses {
		if !b.Bunched || b.Convoy != want[i] {
			t.Errorf("bus %s: expected cluster of %d, got %+v", b.VehicleId, want[i], b)
		}
	}
	// The middle bus is bunched with the one ahead and the one behind
	if m := md.Buses[1]; !m.BunchedAhead || m.GapAhead != "~1.0" || m.GapBehind != "~1.0" {
		t.Errorf("unexpected middle bus %+v", m)
	}
	// The big gap between the clusters is not bunched
	if md.Buses[3].BunchedAhead {
		t.Errorf("d is far behind c: %+v", md.Buses[3])
	}
}

func TestBunchedSummary(t *testing.T) {
	d := threeThenTwo()
	if got := bunchedSummary(d); got != "5 bunched into 2 clusters" {
		t.Errorf("unexpected summary %q", got)
	}
	// Just the pair
	d.Convoys = d.Convoys[1:]
	if got := bunchedSummary(d); got != "2 bunched into 1 cluster" {
		t.Errorf("unexpected summary %q", got)
	}
	d.Convoys = nil
	if got := bunchedSummary(d); got != "none bunched" {
		t.Errorf("unexpected summary %q", got)
	}
}

func TestStripLabelsClustersOnce(t *testing.T) {
	svg := stripSVG(threeThenTwo())
	if strings.Count(svg, ">3 bunched<") != 1 || strings.Count(svg, ">bunched<") != 1 {
		t.Errorf("expected one label per cluster (3 bunched, bunched), got %s", svg)
	}
	if strings.Count(svg, `class="bus bunched"`) != 5 {
		t.Errorf("expected all 5 buses marked as bunched")
	}
}

func TestSynthPredictions(t *testing.T) {
	// Every synthetic bus in service gets predictions, and the horizon leaves some gaps to the fallback
	s := newLiveSource(nil)
	live, err := s.getLive(t.Context(), SYNTH_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	gaps, estimated := 0, 0
	for _, d := range live.Directions {
		gaps += len(d.Gaps)
		estimated += d.Estimated
		for _, b := range d.Buses {
			if !b.AtTerminal && len(b.Predictions) == 0 {
				t.Errorf("bus %s has no predictions", b.VehicleId)
			}
		}
	}
	if gaps == 0 || estimated == gaps {
		t.Errorf("expected mostly predicted gaps, got %d of %d estimated", estimated, gaps)
	}
}

func TestBuildLiveDirectionFewBuses(t *testing.T) {
	d := buildLiveDirection(straightRef(50000), []liveBus{{VehicleId: "a", PositionFt: 20000}, {VehicleId: "b", PositionFt: 10000}})
	if len(d.Gaps) != 1 || !math.IsNaN(d.ExpectedWait) {
		t.Errorf("one gap isn't enough for a wait estimate: %+v", d)
	}
	d = buildLiveDirection(straightRef(50000), nil)
	if d.InService != 0 || len(d.Gaps) != 0 || !math.IsNaN(d.ExpectedWait) {
		t.Errorf("expected nothing for a route without buses, got %+v", d)
	}
}

func TestProjectAndPlace(t *testing.T) {
	p := synthPattern(1, "Northbound", false)
	ref := newRefPattern(p)
	if !almostEqual(ref.lengthFt, p.LengthFeet) {
		t.Fatalf("length mismatch %f vs %f", ref.lengthFt, p.LengthFeet)
	}
	// Halfway along, nudged a bit off the line
	mid := cta.Position{
		Latitude:  (SYNTH_SOUTH_LAT+SYNTH_NORTH_LAT)/2 + 0.0005,
		Longitude: (SYNTH_SOUTH_LON+SYNTH_NORTH_LON)/2 - 0.0005,
	}
	along, off := ref.project(mid)
	if math.Abs(along-ref.lengthFt/2) > 300 || off > 300 {
		t.Errorf("expected about %f ft along and close to the line, got %f along, %f off", ref.lengthFt/2, along, off)
	}

	// A bus on the reference pattern uses its pdist, a bus on another pattern gets projected
	onRef := cta.Vehicle{PatternId: 1, PatternDistanceFt: 1000}
	if at, ok := ref.place(onRef); !ok || !almostEqual(at, 1000) {
		t.Errorf("expected 1000 ft, got %f (%v)", at, ok)
	}
	other := cta.Vehicle{PatternId: 2, Latitude: "41.825", Longitude: "-87.601", Heading: "0"}
	if at, ok := ref.place(other); !ok || math.Abs(at-ref.lengthFt/2) > 500 {
		t.Errorf("expected about halfway, got %f (%v)", at, ok)
	}
}

func TestReferencePatternsPicksLongest(t *testing.T) {
	long := synthPattern(1, "Northbound", false)
	short := synthPattern(2, "Northbound", false)
	short.LengthFeet = long.LengthFeet / 2
	refs := referencePatterns([]cta.Pattern{short, long, synthPattern(3, "Southbound", true)})
	if len(refs) != 2 || refs["Northbound"].pid != 1 || refs["Southbound"].pid != 3 {
		t.Errorf("unexpected references %+v", refs)
	}
}

func TestParseRouteList(t *testing.T) {
	got := strings.Join(parseRouteList(" 6, J14,,6,55,X9,147"), "|")
	if got != "6|J14|55|X9" {
		t.Errorf("unexpected routes %s", got)
	}
}

func TestLiveCache(t *testing.T) {
	s := newLiveSource(nil)
	first, err := s.getLive(t.Context(), SYNTH_ROUTE)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := s.getLive(t.Context(), SYNTH_ROUTE)
	if first != second {
		t.Errorf("expected the cached result within %s", LIVE_CACHE_FOR)
	}
	s.live[SYNTH_ROUTE].fetchedAt = time.Now().Add(-LIVE_CACHE_FOR)
	third, _ := s.getLive(t.Context(), SYNTH_ROUTE)
	if third == first {
		t.Errorf("expected a fresh result after the cache expired")
	}
}

func TestServer(t *testing.T) {
	srv := httptest.NewServer(newServer(newLiveSource(nil), time.Minute))
	defer srv.Close()

	cases := map[string]string{
		"/":                 `id="map"`,
		"/route/6":          "expected wait if you arrive now",
		"/route/6/map":      `id="map"`,
		"/route/6/map.json": `"convoys"`,
		"/map.js":           "L.map",
	}
	for path, want := range cases {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		var body strings.Builder
		_, _ = io.Copy(&body, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status %d", path, resp.StatusCode)
		}
		if !strings.Contains(body.String(), want) {
			t.Errorf("%s: expected %q in the response", path, want)
		}
	}
}
