package main

import (
	"fmt"
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
