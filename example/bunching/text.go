package main

import (
	"fmt"
	"math"
	"strings"
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

func plural(n int, one string, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

func routeTitle(r *liveRoute) string {
	if r.Name == "" {
		return "#" + r.Route
	}
	return "#" + r.Route + " " + r.Name
}

func snapshotCaption(r *liveRoute) string {
	caption := "Live snapshot, " + r.FetchedAt.In(chicago).Format("Mon Jan 2 2006, 3:04 PM MST") + ". One moment, not a trend."
	if r.Synthetic {
		caption = "SYNTHETIC DATA - simulated buses, not the CTA. " + caption
	}
	return caption
}

// The headline in two parts: the bus count and bunching, then the wait
func headlineLines(d liveDirection) (string, string) {
	if d.InService == 0 {
		return "Right now: no buses in service.", ""
	}
	count := fmt.Sprintf("Right now: %s, %s.", plural(d.InService, "bus", "buses"), plural(d.BunchedPairs, "bunched pair", "bunched pairs"))
	if math.IsNaN(d.ExpectedWait) {
		return count, "Too few buses to estimate a wait."
	}
	return count, fmt.Sprintf("Expected wait for a rider arriving now ≈ %s min vs %s min if evenly spaced.",
		formatMinutes(d.ExpectedWait), formatMinutes(d.EvenSpacingWait))
}

func headline(d liveDirection) string {
	a, b := headlineLines(d)
	return strings.TrimSpace(a + " " + b)
}

func speedNote(d liveDirection) string {
	if d.SpeedIsReal {
		return fmt.Sprintf("Gaps in minutes assume %.0f mph, the average speed the buses report right now.", d.SpeedMph)
	}
	return fmt.Sprintf("Gaps in minutes assume %.0f mph (a typical CTA bus average) because too few buses reported a usable speed.", d.SpeedMph)
}

func gapList(d liveDirection) string {
	if len(d.Gaps) == 0 {
		return ""
	}
	parts := make([]string, len(d.Gaps))
	for i, g := range d.Gaps {
		parts[i] = formatMinutes(g.Minutes)
		if g.Bunched {
			parts[i] += " (bunched)"
		}
	}
	return "Gaps front to back, in minutes: " + strings.Join(parts, ", ")
}

func snapshotMarkdown(r *liveRoute, svgFile string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s - bunching snapshot\n\n", routeTitle(r))
	fmt.Fprintf(&sb, "*%s*\n\n", snapshotCaption(r))
	if svgFile != "" {
		fmt.Fprintf(&sb, "![Snapshot](%s)\n\n", svgFile)
	}
	for _, d := range r.Directions {
		fmt.Fprintf(&sb, "**%s:** %s\n\n", d.Direction, headline(d))
		if gl := gapList(d); gl != "" {
			fmt.Fprintf(&sb, "%s. %s\n\n", gl, speedNote(d))
		}
	}
	sb.WriteString(METHOD_MARKDOWN)
	return sb.String()
}

const METHOD_MARKDOWN = `## How this is computed

- Every bus is placed along its direction's longest pattern, by distance along the pattern (the API's ` + "`pdist`" + ` for buses on that pattern, projection of the GPS position for buses on other variants). Buses within 1000 ft of either end are treated as laying over and left out.
- The gap between consecutive buses is a distance, turned into minutes with one speed per direction: the average speed the buses currently report (10 mph if fewer than 3 buses report a usable speed).
- **Bunched**: a gap under 2 minutes, or under 25% of the median gap in that direction, whichever is larger.
- **Expected wait for a rider arriving now**: E[H²] / (2·E[H]) over the current gaps H. A rider is more likely to arrive during a long gap, which is why uneven spacing makes the average wait longer. **If evenly spaced**: E[H] / 2, the wait with the same buses spread out evenly. The open-ended gaps ahead of the first bus and behind the last are not included.
`
