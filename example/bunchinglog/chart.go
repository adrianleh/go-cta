package main

import (
	"fmt"
	"math"
	"strings"
)

// Plain SVG so we don't pull in a plotting dependency
const (
	CHART_WIDTH        = 720
	CHART_PANEL_HEIGHT = 190
	CHART_MARGIN_LEFT  = 52
	CHART_MARGIN_RIGHT = 16
	CHART_HEADER       = 64
	CHART_MAX_MINUTES  = 30

	COLOR_SURFACE   = "#fcfcfb"
	COLOR_TEXT      = "#0b0b0b"
	COLOR_TEXT_MUTE = "#52514e"
	COLOR_GRID      = "#e4e3df"
	COLOR_HEADWAY   = "#2a78d6"
	COLOR_BUNCHED   = "#eb6834"
)

// One histogram per route/direction (all periods pooled), share of headways per 1-minute bin.
// Bins under the bunching threshold are drawn in the "bunched" color.
func headwayChartSVG(a *analysis, cfg analyzerConfig) string {
	panels := make([]directionResult, 0, len(a.directions))
	for _, d := range a.directions {
		if len(d.headways) > 0 {
			panels = append(panels, d)
		}
	}
	height := CHART_HEADER + len(panels)*CHART_PANEL_HEIGHT + 8
	plotW := float64(CHART_WIDTH - CHART_MARGIN_LEFT - CHART_MARGIN_RIGHT)
	nBins := CHART_MAX_MINUTES + 1 // last bin is "30+"
	binW := plotW / float64(nBins)

	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="-apple-system, Helvetica, Arial, sans-serif" role="img">`+"\n",
		CHART_WIDTH, height, CHART_WIDTH, height)
	title := "Headway distribution"
	if a.synthetic {
		title += " - SYNTHETIC DATA"
	}
	fmt.Fprintf(&sb, `<title>%s</title>`+"\n", title)
	fmt.Fprintf(&sb, `<rect width="100%%" height="100%%" fill="%s"/>`+"\n", COLOR_SURFACE)
	fmt.Fprintf(&sb, `<text x="%d" y="22" font-size="15" font-weight="600" fill="%s">%s</text>`+"\n", CHART_MARGIN_LEFT, COLOR_TEXT, title)

	// Legend
	lx := CHART_MARGIN_LEFT
	fmt.Fprintf(&sb, `<rect x="%d" y="34" width="12" height="12" rx="2" fill="%s"/>`, lx, COLOR_BUNCHED)
	fmt.Fprintf(&sb, `<text x="%d" y="44" font-size="12" fill="%s">Bunched (under %g min)</text>`, lx+18, COLOR_TEXT_MUTE, cfg.bunchMinutes)
	fmt.Fprintf(&sb, `<rect x="%d" y="34" width="12" height="12" rx="2" fill="%s"/>`, lx+190, COLOR_HEADWAY)
	fmt.Fprintf(&sb, `<text x="%d" y="44" font-size="12" fill="%s">Other headways</text>`+"\n", lx+208, COLOR_TEXT_MUTE)

	// Same y scale on every panel so directions compare at a glance
	maxShare := 0.0
	for _, d := range panels {
		for _, s := range binShares(d.headways, nBins) {
			maxShare = max(maxShare, s)
		}
	}
	yMax, step := niceAxis(maxShare)

	for i, d := range panels {
		top := float64(CHART_HEADER + i*CHART_PANEL_HEIGHT)
		plotTop := top + 26
		plotBottom := top + CHART_PANEL_HEIGHT - 34
		plotH := plotBottom - plotTop

		counts := binCounts(d.headways, nBins)
		shares := binShares(d.headways, nBins)

		fmt.Fprintf(&sb, `<text x="%d" y="%.0f" font-size="13" font-weight="600" fill="%s">#%s %s</text>`, CHART_MARGIN_LEFT, top+14, COLOR_TEXT, d.route, d.direction)
		fmt.Fprintf(&sb, `<text x="%d" y="%.0f" font-size="12" text-anchor="end" fill="%s">%d headways, all periods</text>`+"\n",
			CHART_WIDTH-CHART_MARGIN_RIGHT, top+14, COLOR_TEXT_MUTE, len(d.headways))

		// Grid and y labels
		for v := 0.0; v <= yMax+1e-9; v += step {
			y := plotBottom - v/yMax*plotH
			fmt.Fprintf(&sb, `<line x1="%d" x2="%d" y1="%.1f" y2="%.1f" stroke="%s" stroke-width="1"/>`, CHART_MARGIN_LEFT, CHART_WIDTH-CHART_MARGIN_RIGHT, y, y, COLOR_GRID)
			fmt.Fprintf(&sb, `<text x="%d" y="%.1f" font-size="11" text-anchor="end" fill="%s">%.0f%%</text>`+"\n", CHART_MARGIN_LEFT-6, y+4, COLOR_TEXT_MUTE, 100*v)
		}

		// Bars, with a rounded top and a square base on the baseline
		for b, s := range shares {
			if counts[b] == 0 {
				continue
			}
			x := float64(CHART_MARGIN_LEFT) + float64(b)*binW + 1
			w := binW - 2
			h := s / yMax * plotH
			color := COLOR_HEADWAY
			if float64(b+1) <= cfg.bunchMinutes {
				color = COLOR_BUNCHED
			}
			label := fmt.Sprintf("%d-%d min", b, b+1)
			if b == nBins-1 {
				label = fmt.Sprintf("%d+ min", b)
			}
			fmt.Fprintf(&sb, `<path d="%s" fill="%s"><title>%s: %.1f%% (%d headways)</title></path>`+"\n",
				roundedTopBar(x, plotBottom, w, h, 3), color, label, 100*s, counts[b])
		}

		// X axis
		fmt.Fprintf(&sb, `<line x1="%d" x2="%d" y1="%.1f" y2="%.1f" stroke="%s" stroke-width="1"/>`, CHART_MARGIN_LEFT, CHART_WIDTH-CHART_MARGIN_RIGHT, plotBottom, plotBottom, COLOR_TEXT_MUTE)
		for m := 0; m <= CHART_MAX_MINUTES; m += 5 {
			x := float64(CHART_MARGIN_LEFT) + float64(m)*binW
			label := fmt.Sprint(m)
			if m == CHART_MAX_MINUTES {
				label += "+"
			}
			fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" font-size="11" text-anchor="middle" fill="%s">%s</text>`, x, plotBottom+15, COLOR_TEXT_MUTE, label)
		}
		fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" font-size="11" text-anchor="middle" fill="%s">Headway (minutes)</text>`+"\n",
			float64(CHART_MARGIN_LEFT)+plotW/2, plotBottom+29, COLOR_TEXT_MUTE)
	}
	sb.WriteString("</svg>\n")
	return sb.String()
}

func binCounts(hs []headway, nBins int) []int {
	counts := make([]int, nBins)
	for _, h := range hs {
		bin := min(int(math.Floor(h.minutes)), nBins-1)
		counts[max(bin, 0)]++
	}
	return counts
}

func binShares(hs []headway, nBins int) []float64 {
	shares := make([]float64, nBins)
	for b, c := range binCounts(hs, nBins) {
		shares[b] = float64(c) / float64(len(hs))
	}
	return shares
}

func roundedTopBar(x, baseline, w, h, r float64) string {
	r = min(r, w/2, h)
	top := baseline - h
	return fmt.Sprintf("M%.1f,%.1f V%.1f Q%.1f,%.1f %.1f,%.1f H%.1f Q%.1f,%.1f %.1f,%.1f V%.1f Z",
		x, baseline, top+r, x, top, x+r, top, x+w-r, x+w, top, x+w, top+r, baseline)
}

// Rounds the axis max up to a tidy value, returns (max, tick step)
func niceAxis(v float64) (float64, float64) {
	for _, step := range []float64{0.02, 0.05, 0.1, 0.2, 0.25} {
		if v <= 4*step {
			return math.Ceil(v/step) * step, step
		}
	}
	return 1, 0.25
}
