package main

import (
	"fmt"
	"math"
	"time"
)

var chicago = mustLoadLocation("America/Chicago")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func formatWholeMinutes(m float64) string {
	return fmt.Sprintf("%.0f", m)
}

func routeTitle(r *liveRoute) string {
	if r.Name == "" {
		return "#" + r.Route
	}
	return "#" + r.Route + " " + r.Name
}

func formatGap(g liveGap) string {
	if g.Estimated {
		return "~" + formatMinutes(g.Minutes)
	}
	return formatMinutes(g.Minutes)
}

// "4 bunched into 2 clusters": the buses in bunched clusters, and how many clusters they form.
// Three buses in a row are one cluster of 3, not two pairs.
func bunchedSummary(d liveDirection) string {
	if len(d.Convoys) == 0 {
		return "none bunched"
	}
	buses := 0
	for _, run := range d.Convoys {
		buses += run[1] - run[0] + 1
	}
	noun := "clusters"
	if len(d.Convoys) == 1 {
		noun = "cluster"
	}
	return fmt.Sprintf("%d bunched into %d %s", buses, len(d.Convoys), noun)
}

// The longest wait as a multiple of the wait if the same buses were evenly spaced, "" without an estimate
func worstVsEven(d liveDirection) string {
	if math.IsNaN(d.EvenSpacingWait) || d.EvenSpacingWait <= 0 {
		return ""
	}
	return fmt.Sprintf("%.1f×", d.WorstGap.Minutes/d.EvenSpacingWait)
}
