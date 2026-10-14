package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

func pct(x float64) string {
	if math.IsNaN(x) {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", 100*x)
}

func mins(x float64) string {
	if math.IsNaN(x) {
		return "-"
	}
	return fmt.Sprintf("%.1f", x)
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// The short plain-language part. Groups with too few headways are left out here (they're still in the table).
func (a *analysis) quickRead(cfg analyzerConfig) []string {
	var lines []string
	for _, g := range a.groups {
		if g.stats.Count < cfg.minHeadways {
			continue
		}
		lines = append(lines, fmt.Sprintf(
			"#%s %s, %s: average observed wait %s min, versus %s min if the same buses were evenly spaced; "+
				"%s of arrivals came within %g min of the previous bus. (%s over %s, median headway %s min.)",
			g.route, g.direction, lowerFirst(g.period),
			mins(g.stats.ExpectedWait), mins(g.stats.EvenSpacingWait),
			pct(g.stats.ShareUnderAbs), cfg.bunchMinutes,
			plural(g.stats.Count, "headway"), plural(g.days, "day"), mins(g.stats.MedianMinutes)))
	}
	return lines
}

func (a *analysis) span() string {
	first := a.first.In(chicago)
	last := a.last.In(chicago)
	days := int(math.Ceil(last.Sub(first).Hours() / 24))
	return fmt.Sprintf("%s to %s (about %s)", first.Format("Mon 2006-01-02 15:04"), last.Format("Mon 2006-01-02 15:04"), plural(max(days, 1), "day"))
}

const SYNTHETIC_BANNER = "**SYNTHETIC DATA.** These numbers come from simulated buses, not the CTA. They show what the output looks like and are not a finding."

func (a *analysis) summaryText(cfg analyzerConfig) string {
	var sb strings.Builder
	if a.synthetic {
		sb.WriteString("*** SYNTHETIC DATA - simulated buses, not a real finding ***\n\n")
	}
	fmt.Fprintf(&sb, "%d observations, %s\n\n", a.observations, a.span())
	lines := a.quickRead(cfg)
	if len(lines) == 0 {
		fmt.Fprintf(&sb, "Not enough data yet: no route/direction/period has %d or more headways.\n", cfg.minHeadways)
	}
	for _, l := range lines {
		sb.WriteString("- " + l + "\n")
	}
	return sb.String()
}

func (a *analysis) markdown(cfg analyzerConfig, chartFile string) string {
	var sb strings.Builder
	sb.WriteString("# Bus bunching quick read\n\n")
	if a.synthetic {
		sb.WriteString("> " + SYNTHETIC_BANNER + "\n\n")
	}
	fmt.Fprintf(&sb, "Data: %d vehicle observations, %s. Generated %s.\n\n", a.observations, a.span(), time.Now().In(chicago).Format("2006-01-02 15:04"))

	sb.WriteString("## Quick read\n\n")
	lines := a.quickRead(cfg)
	if len(lines) == 0 {
		fmt.Fprintf(&sb, "Not enough data yet: no route/direction/period has %d or more headways.\n", cfg.minHeadways)
	}
	for _, l := range lines {
		sb.WriteString("- " + l + "\n")
	}
	sb.WriteString("\n")

	if chartFile != "" {
		fmt.Fprintf(&sb, "![Headway distribution](%s)\n\n", chartFile)
	}

	sb.WriteString("## All groups\n\n")
	fmt.Fprintf(&sb, "| Route | Direction | Period | Days | Headways | Mean headway | Median headway | CV | Observed wait | Even-spacing wait | Excess wait | Under %g min | Under %.0f%% of median |\n",
		cfg.bunchMinutes, 100*cfg.bunchFraction)
	sb.WriteString("|---|---|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|\n")
	for _, g := range a.groups {
		s := g.stats
		thin := ""
		if s.Count < cfg.minHeadways {
			thin = " (thin)"
		}
		fmt.Fprintf(&sb, "| %s | %s | %s | %d | %d%s | %s | %s | %.2f | %s | %s | %s | %s | %s |\n",
			g.route, g.direction, g.period, g.days, s.Count, thin,
			mins(s.MeanMinutes), mins(s.MedianMinutes), s.CV,
			mins(s.ExpectedWait), mins(s.EvenSpacingWait), mins(s.ExpectedWait-s.EvenSpacingWait),
			pct(s.ShareUnderAbs), pct(s.ShareUnderRel))
	}
	sb.WriteString("\nTimes in minutes. \"(thin)\" marks groups with fewer than " + fmt.Sprint(cfg.minHeadways) + " headways - don't read much into those.\n\n")

	sb.WriteString("## Reference stops\n\n")
	for _, d := range a.directions {
		names := make([]string, len(d.stops))
		for i, s := range d.stops {
			names[i] = fmt.Sprintf("%s (%s)", s.stopName, s.stopId)
		}
		fmt.Fprintf(&sb, "- #%s %s: %s\n", d.route, d.direction, strings.Join(names, "; "))
	}
	sb.WriteString("\n")

	sb.WriteString("## Definitions\n\n")
	sb.WriteString("- **Headway**: time between consecutive buses passing a reference stop, from any pattern serving that stop. " +
		"Passing times are interpolated linearly between position reports using distance along the pattern (`pdist`).\n")
	sb.WriteString("- **Observed wait**: average wait for a rider arriving at a random time, E[H²] / (2·E[H]).\n")
	sb.WriteString("- **Even-spacing wait**: E[H] / 2, the wait if the same number of buses were perfectly evenly spaced. " +
		"The difference is the excess wait caused by uneven spacing. It is not a comparison with the schedule.\n")
	fmt.Fprintf(&sb, "- **Bunched**: a headway under %g minutes, i.e. the bus arrived within %g minutes of the one before it. "+
		"The relative column uses under %.0f%% of the group's median headway instead.\n", cfg.bunchMinutes, cfg.bunchMinutes, 100*cfg.bunchFraction)
	sb.WriteString("- **Period**: by when the second bus passed, Chicago time. Holidays count as weekdays.\n")
	fmt.Fprintf(&sb, "- Headways spanning a stretch of more than %s with no data for the route (logger down, API errors, no service) are dropped.\n", cfg.maxGap)
	return sb.String()
}
