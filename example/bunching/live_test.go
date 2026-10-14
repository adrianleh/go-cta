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
	if s, real := directionSpeed([]int{0, 12, 15, 9}); !real || s != 9 {
		t.Errorf("expected measured 9 mph, got %f (%v)", s, real)
	}
	if s, real := directionSpeed([]int{12, 15}); real || s != DEFAULT_SPEED_MPH {
		t.Errorf("expected default with too few buses, got %f (%v)", s, real)
	}
	if s, real := directionSpeed([]int{0, 0, 0, 0}); real || s != DEFAULT_SPEED_MPH {
		t.Errorf("expected default when nobody reports a speed, got %f (%v)", s, real)
	}
	if s, _ := directionSpeed([]int{60, 60, 60}); s != MAX_SPEED_MPH {
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
	if !d.SpeedIsReal || d.SpeedMph != 10 {
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

func TestBuildLiveDirectionFewBuses(t *testing.T) {
	d := buildLiveDirection(straightRef(50000), []liveBus{{VehicleId: "a", PositionFt: 20000}, {VehicleId: "b", PositionFt: 10000}})
	if len(d.Gaps) != 1 || !math.IsNaN(d.ExpectedWait) {
		t.Errorf("one gap isn't enough for a wait estimate: %+v", d)
	}
	if !strings.Contains(headline(d), "Too few buses") {
		t.Errorf("unexpected headline %q", headline(d))
	}
	d = buildLiveDirection(straightRef(50000), nil)
	if !strings.Contains(headline(d), "no buses") {
		t.Errorf("unexpected headline %q", headline(d))
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
	srv := httptest.NewServer(newServer(newLiveSource(nil)))
	defer srv.Close()

	cases := map[string]string{
		"/":                     "Jackson Park Express (synthetic)",
		"/route/6":              "expected wait if you arrive now",
		"/route/6/snapshot.svg": "SYNTHETIC DATA",
		"/route/6/snapshot.md":  "E[H²] / (2·E[H])",
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
