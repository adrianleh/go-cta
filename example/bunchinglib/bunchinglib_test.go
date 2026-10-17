package bunchinglib

import (
	"slices"
	"testing"
	"time"
)

func TestThreshold(t *testing.T) {
	// Frequent route: the 2 minute rule wins
	if th := DefaultRule.Threshold(Headways("6", "", []float64{6, 6, 6})); th != 2*time.Minute {
		t.Errorf("expected 2m, got %s", th)
	}
	// Infrequent route: 25% of a 20 minute median is 5 minutes
	if th := DefaultRule.Threshold(Headways("6", "", []float64{20, 20, 20})); th != 5*time.Minute {
		t.Errorf("expected 5m, got %s", th)
	}
	if th := DefaultRule.Threshold(Headways("6", "", nil)); th != 2*time.Minute {
		t.Errorf("expected 2m without gaps, got %s", th)
	}
}

func TestBunchedAndConvoys(t *testing.T) {
	// Gaps 1, 1, 18, 1, 20: the median is 1, so the 2 minute rule applies and the three 1 minute gaps are bunched
	h := Headways("6", "Northbound", []float64{1, 1, 18, 1, 20})
	bunched := Bunched(h, DefaultRule.Threshold(h))
	if want := []bool{true, true, false, true, false}; !slices.Equal(bunched, want) {
		t.Errorf("expected %v, got %v", want, bunched)
	}
	if runs := Convoys(bunched); len(runs) != 2 || runs[0] != [2]int{0, 2} || runs[1] != [2]int{3, 4} {
		t.Errorf("unexpected runs %v", runs)
	}
}
