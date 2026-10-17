package cta

import (
	"testing"
	"time"
)

func minutes(ms ...float64) []time.Duration {
	var out []time.Duration
	for _, m := range ms {
		out = append(out, time.Duration(m*float64(time.Minute)))
	}
	return out
}

func TestExpectedWait(t *testing.T) {
	// Evenly spaced buses: wait is half the gap
	if w := (Headways{Gaps: minutes(10, 10, 10)}).ExpectedWait(); w != 5*time.Minute {
		t.Errorf("expected 5m, got %s", w)
	}
	// Same 20 minutes of buses, bunched: 1 and 19 gives (1+361)/(2*20) = 9.05
	if w := (Headways{Gaps: minutes(1, 19)}).ExpectedWait(); w != minutes(9.05)[0] {
		t.Errorf("expected 9.05m, got %s", w)
	}
	if w := (Headways{}).ExpectedWait(); w != 0 {
		t.Errorf("expected 0 without gaps, got %s", w)
	}
}

func TestEvenSpacingWait(t *testing.T) {
	if w := (Headways{Gaps: minutes(1, 19)}).EvenSpacingWait(); w != 5*time.Minute {
		t.Errorf("expected 5m, got %s", w)
	}
}

func TestMedian(t *testing.T) {
	if m := (Headways{Gaps: minutes(3, 1, 2)}).Median(); m != 2*time.Minute {
		t.Errorf("expected 2m, got %s", m)
	}
	if m := (Headways{Gaps: minutes(4, 1, 3, 2)}).Median(); m != 150*time.Second {
		t.Errorf("expected 2m30s, got %s", m)
	}
}

func TestLongest(t *testing.T) {
	if l := (Headways{Gaps: minutes(3, 19, 2)}).Longest(); l != 19*time.Minute {
		t.Errorf("expected 19m, got %s", l)
	}
}

func TestShareUnder(t *testing.T) {
	if s := (Headways{Gaps: minutes(1, 2, 3, 4)}).ShareUnder(3 * time.Minute); s != 0.5 {
		t.Errorf("expected 0.5, got %f", s)
	}
}

func TestNewHeadwaysFromTimes(t *testing.T) {
	t0 := time.Date(2026, time.October, 6, 17, 0, 0, 0, time.UTC)
	h := NewHeadwaysFromTimes("6", "Northbound", []time.Time{t0.Add(12 * time.Minute), t0, t0.Add(5 * time.Minute)})
	if h.Route != "6" || h.Direction != "Northbound" {
		t.Errorf("unexpected route/direction %q %q", h.Route, h.Direction)
	}
	if h.Count() != 2 || h.Gaps[0] != 5*time.Minute || h.Gaps[1] != 7*time.Minute {
		t.Errorf("expected gaps of 5m and 7m, got %v", h.Gaps)
	}
}
