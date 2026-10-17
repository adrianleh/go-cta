package main

import (
	"math"
	"testing"
	"time"

	"adrianlehmann.net/cta"
	"adrianlehmann.net/cta/example/bunchinglib"
)

var t0 = time.Date(2026, time.October, 6, 17, 0, 0, 0, chicago) // a Tuesday, PM peak

func at(minutes float64) time.Time {
	return t0.Add(time.Duration(minutes * float64(time.Minute)))
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func oneChain(minutes ...float64) []cta.Headways {
	return []cta.Headways{bunchinglib.Headways("6", "Northbound", minutes)}
}

func TestExpectedWaitMatchesCVForm(t *testing.T) {
	// W = E[H]/2 * (1 + CV^2) is the same formula written differently
	s := computeStats(oneChain(2, 7, 11, 4, 16, 9, 1, 12), bunchinglib.DefaultRule)
	alt := s.MeanMinutes / 2 * (1 + s.CV*s.CV)
	if !almostEqual(s.ExpectedWait, alt) {
		t.Errorf("E[H^2]/(2E[H]) = %f but E[H]/2 (1+CV^2) = %f", s.ExpectedWait, alt)
	}
	if !almostEqual(s.EvenSpacingWait, s.MeanMinutes/2) {
		t.Errorf("even spacing wait should be half the mean headway")
	}
	if s.ExpectedWait < s.EvenSpacingWait {
		t.Errorf("observed wait can never be below the even-spacing wait")
	}
}

func TestComputeStatsEmpty(t *testing.T) {
	s := computeStats(nil, bunchinglib.DefaultRule)
	if !math.IsNaN(s.ExpectedWait) || !math.IsNaN(s.MeanMinutes) || !math.IsNaN(s.BunchedShare) {
		t.Errorf("expected NaN for no headways, got %+v", s)
	}
}

func TestComputeStatsClusters(t *testing.T) {
	// Pooled gaps 1, 2, 10, 10, 1.5, 20 have median 6, so the threshold is max(2, 1.5) = 2 minutes and 2 is not bunched.
	// Chain one has a pair (gap 1), chain two has a pair (gap 1.5)
	chains := []cta.Headways{bunchinglib.Headways("6", "Northbound", []float64{1, 2, 10}), bunchinglib.Headways("6", "Northbound", []float64{10, 1.5, 20})}
	s := computeStats(chains, bunchinglib.DefaultRule)
	if s.Count != 6 || !almostEqual(s.MedianMinutes, 6) || !almostEqual(s.ThresholdMin, 2) {
		t.Fatalf("unexpected count/median/threshold %+v", s)
	}
	if !almostEqual(s.BunchedShare, 2.0/6) || s.Clusters != 2 || s.BunchedBuses != 4 {
		t.Errorf("expected 2 of 6 bunched in 2 clusters of 4 buses, got %+v", s)
	}
	// Three buses in a row are one cluster of 3, not two pairs
	s = computeStats(oneChain(1, 1, 10, 10, 10), bunchinglib.DefaultRule)
	if s.Clusters != 1 || s.BunchedBuses != 3 {
		t.Errorf("expected one cluster of 3 buses, got %+v", s)
	}
}

func TestChainsBreakOnDroppedHeadwaysAndStops(t *testing.T) {
	hs := []headway{
		{stop: "a", start: at(0), at: at(1), minutes: 1},
		{stop: "a", start: at(1), at: at(3), minutes: 2},
		{stop: "a", start: at(10), at: at(12), minutes: 2}, // the one from minute 3 to 10 was dropped
		{stop: "b", start: at(12), at: at(13), minutes: 1}, // different stop
	}
	cs := chains("6", "Northbound", hs)
	if len(cs) != 3 || cs[0].Count() != 2 || cs[1].Count() != 1 || cs[2].Count() != 1 {
		t.Errorf("unexpected chains %+v", cs)
	}
}

func TestInterpolateCrossing(t *testing.T) {
	// 1000 ft at 0 min, 3000 ft at 2 min, so 2500 ft is passed at 1.5 min
	got := interpolateCrossing(at(0), 1000, at(2), 3000, 2500)
	if !got.Equal(at(1.5)) {
		t.Errorf("expected %s, got %s", at(1.5), got)
	}
}

func obsAt(vid string, pid int, minutes float64, pdist int) Observation {
	return Observation{PolledAt: at(minutes), Timestamp: at(minutes), VehicleId: vid, Route: "6", PatternId: pid, PatternDistanceFt: pdist}
}

func TestTrackCrossing(t *testing.T) {
	r := track{
		obsAt("1", 1, 0, 0),
		obsAt("1", 1, 1, 1000),
		obsAt("1", 1, 2, 2000),
		obsAt("1", 1, 3, 1990), // GPS jitter backwards
		obsAt("1", 1, 4, 3000),
	}
	got, ok := r.crossing(1500)
	if !ok || !got.Equal(at(1.5)) {
		t.Errorf("expected crossing at %s, got %s (%v)", at(1.5), got, ok)
	}
	// Jitter around 1995 must only count once, at the first pass
	got, ok = r.crossing(1995)
	if !ok || !got.Equal(at(1.995)) {
		t.Errorf("expected crossing at %s, got %s (%v)", at(1.995), got, ok)
	}
	if _, ok := r.crossing(5000); ok {
		t.Errorf("bus never got to 5000 ft")
	}
	// Bus was already past 0 at the first report, so we can't say when it passed it
	if _, ok := r.crossing(0); ok {
		t.Errorf("no crossing expected at the first report's distance")
	}
}

func TestBuildRunsSplitsTripsAndGaps(t *testing.T) {
	obs := []Observation{
		obsAt("1", 1, 0, 0),
		obsAt("1", 1, 1, 1000),
		obsAt("1", 1, 1, 1000), // same report seen in two polls
		obsAt("1", 1, 2, 2000),
		obsAt("1", 1, 3, 100), // new trip on the same pattern
		obsAt("1", 1, 4, 1100),
		obsAt("1", 1, 30, 9000), // lost sight of it for 26 minutes
		obsAt("2", 1, 0, 500),
	}
	runs := buildRuns(obs, 10*time.Minute)
	lengths := map[int]int{}
	for _, r := range runs {
		lengths[len(r)]++
	}
	// vehicle 1: [0,1,2] [3,4] [30], vehicle 2: [0]
	if len(runs) != 4 || lengths[3] != 1 || lengths[2] != 1 || lengths[1] != 2 {
		t.Errorf("unexpected runs: %v", runs)
	}
}

func TestHeadwaysDropGaps(t *testing.T) {
	crossings := []time.Time{at(30), at(0), at(10), at(11), at(60)}
	// The logger saw nothing between minute 40 and 55
	gaps := coverageGaps([]time.Time{at(0), at(20), at(40), at(55), at(60)}, 25*time.Minute)
	if len(gaps) != 0 {
		t.Fatalf("expected no gaps over 25 min, got %v", gaps)
	}
	gaps = coverageGaps([]time.Time{at(0), at(5), at(10), at(15), at(20), at(25), at(30), at(35), at(40), at(55), at(60)}, 5*time.Minute)
	if len(gaps) != 1 {
		t.Fatalf("expected one gap, got %v", gaps)
	}
	hs := headways("a", crossings, gaps)
	want := []float64{10, 1, 19} // 30 -> 60 spans the gap and is dropped
	if len(hs) != len(want) {
		t.Fatalf("expected %v, got %v", want, hs)
	}
	for i := range want {
		if !almostEqual(hs[i].minutes, want[i]) {
			t.Errorf("headway %d: expected %f, got %f", i, want[i], hs[i].minutes)
		}
	}
}

func TestPeriodOf(t *testing.T) {
	day := func(d, h int) time.Time { return time.Date(2026, time.October, d, h, 30, 0, 0, chicago) }
	cases := map[time.Time]string{
		day(6, 3):  PERIOD_OVERNIGHT,
		day(6, 7):  PERIOD_AM_PEAK,
		day(6, 12): PERIOD_MIDDAY,
		day(6, 17): PERIOD_PM_PEAK,
		day(6, 21): PERIOD_EVENING,
		day(10, 8): PERIOD_WEEKEND, // Saturday
		day(11, 8): PERIOD_WEEKEND, // Sunday
		// UTC input is converted to Chicago time first: 23:30 UTC is 18:30 CDT
		time.Date(2026, time.October, 6, 23, 30, 0, 0, time.UTC): PERIOD_PM_PEAK,
	}
	for in, want := range cases {
		if got := periodOf(in); got != want {
			t.Errorf("periodOf(%s): expected %q, got %q", in, want, got)
		}
	}
}

func TestPickReferenceStops(t *testing.T) {
	p := synthPattern(1, "Northbound")
	stops := pickReferenceStops(p, 5)
	if len(stops) != 5 {
		t.Fatalf("expected 5 stops, got %d", len(stops))
	}
	// First reference stop is near 10% of the way along
	d, _ := p.stopDistance(stops[0].stopId)
	if math.Abs(d-0.1*p.LengthFeet) > SYNTH_STOP_SPACING {
		t.Errorf("first stop at %f ft, expected near %f", d, 0.1*p.LengthFeet)
	}
}

func TestAnalyzePoolsPatterns(t *testing.T) {
	// Two patterns in the same direction serving the same stop at different distances (e.g. a short-turn variant).
	// Buses on both must count towards the headway at that stop.
	stop := PatternStop{StopId: "100", StopName: "Test & Stop", PatternDistance: 5000}
	patterns := map[int]*PatternInfo{
		1: {Pid: 1, Route: "6", Direction: "Northbound", LengthFeet: 10000, Stops: []PatternStop{{StopId: "0", PatternDistance: 0}, stop}},
		2: {Pid: 2, Route: "6", Direction: "Northbound", LengthFeet: 8000, Stops: []PatternStop{{StopId: "0b", PatternDistance: 0}, {StopId: "100", PatternDistance: 3000}}},
	}
	var obs []Observation
	// Bus A on pattern 1 passes 5000 ft at minute 5, bus B on pattern 2 passes 3000 ft at minute 6, bus C on pattern 1 at minute 15
	for m := 0.0; m <= 20; m++ {
		obs = append(obs, obsAt("A", 1, m, int(1000*m)))
		obs = append(obs, obsAt("B", 2, m, int(1000*(m-3))))
		obs = append(obs, obsAt("C", 1, m, int(1000*(m-10))))
		obs = append(obs, obsAt("C", 1, m, int(1000*(m-10))))
	}
	obs = append(obs, obsAt("A", 1, 0, 0)) // duplicate from a restart
	cfg := analyzerConfig{stops: 1, bunchMinutes: 2, bunchFraction: 0.25, maxGap: 5 * time.Minute, maxInterp: 10 * time.Minute}
	a, err := analyze(obs, patterns, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.directions) != 1 || len(a.directions[0].stops) != 1 || a.directions[0].stops[0].stopId != "100" {
		t.Fatalf("unexpected reference stops: %+v", a.directions)
	}
	hs := a.directions[0].headways
	if len(hs) != 2 || !almostEqual(hs[0].minutes, 1) || !almostEqual(hs[1].minutes, 9) {
		t.Fatalf("expected headways [1 9], got %+v", hs)
	}
	if len(a.groups) != 1 || a.groups[0].period != PERIOD_PM_PEAK {
		t.Fatalf("expected one PM peak group, got %+v", a.groups)
	}
	s := a.groups[0].stats
	if !almostEqual(s.BunchedShare, 0.5) || s.Clusters != 1 || !almostEqual(s.ExpectedWait, (1+81)/20.0) {
		t.Errorf("unexpected stats %+v", s)
	}
}
