package main

import (
	"fmt"
	"html"
	"strings"
)

// The strip: one line per direction, travel left to right, buses as dots, gaps between them colored.
// Drawn in a small viewBox so text stays readable when a phone scales it down.
const (
	STRIP_WIDTH  = 360
	STRIP_HEIGHT = 92
	STRIP_PAD    = 14
	STRIP_LINE_Y = 46
)

func stripX(ft float64, lengthFt float64) float64 {
	if lengthFt <= 0 {
		return STRIP_PAD
	}
	return STRIP_PAD + ft/lengthFt*(STRIP_WIDTH-2*STRIP_PAD)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func stripSVG(d liveDirection) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg class="strip" viewBox="0 0 %d %d" role="img" aria-label="%s">`, STRIP_WIDTH, STRIP_HEIGHT, html.EscapeString(fmt.Sprintf("%s, %d buses in service", d.Direction, d.InService)))
	fmt.Fprintf(&sb, `<line class="track" x1="%d" x2="%d" y1="%d" y2="%d" stroke-width="6" stroke-linecap="round"/>`,
		STRIP_PAD, STRIP_WIDTH-STRIP_PAD, STRIP_LINE_Y, STRIP_LINE_Y)

	for _, g := range d.Gaps {
		x1, x2 := stripX(g.FromFt, d.LengthFt), stripX(g.ToFt, d.LengthFt)
		class := "gap"
		if g.Bunched {
			class = "gap bunched"
		}
		// Estimated gaps are drawn dashed
		dash := ""
		if g.Estimated {
			dash = ` stroke-dasharray="6 3"`
		}
		fmt.Fprintf(&sb, `<line class="%s" x1="%.1f" x2="%.1f" y1="%d" y2="%d" stroke-width="6"%s><title>%s min gap</title></line>`,
			class, x1, x2, STRIP_LINE_Y, STRIP_LINE_Y, dash, formatGap(g))
		// Bunched gaps are labeled once per cluster below. Others are labeled if they fit.
		if !g.Bunched && x2-x1 >= 26 {
			mid := (x1 + x2) / 2
			label := formatWholeMinutes(g.Minutes) + "m"
			if g.Estimated {
				label = "~" + label
			}
			fmt.Fprintf(&sb, `<text class="label" x="%.1f" y="%d" font-size="10" text-anchor="middle">%s</text>`, mid, STRIP_LINE_Y-10, label)
		}
	}

	// One label per bunched cluster, centered over it with a tick: "bunched" for a pair, "3 bunched" for three in a row
	inCluster := make(map[int]bool)
	for _, run := range d.Convoys {
		for i := run[0]; i <= run[1]; i++ {
			inCluster[i] = true
		}
		mid := (stripX(d.Gaps[run[1]-1].FromFt, d.LengthFt) + stripX(d.Gaps[run[0]].ToFt, d.LengthFt)) / 2
		label := "bunched"
		if n := run[1] - run[0] + 1; n > 2 {
			label = fmt.Sprintf("%d bunched", n)
		}
		fmt.Fprintf(&sb, `<line class="gap bunched" x1="%.1f" x2="%.1f" y1="%d" y2="%d" stroke-width="1.5"/>`, mid, mid, STRIP_LINE_Y-20, STRIP_LINE_Y-6)
		fmt.Fprintf(&sb, `<text class="label strong" x="%.1f" y="%d" font-size="10" font-weight="700" text-anchor="middle">%s</text>`, mid, STRIP_LINE_Y-24, label)
	}

	for i, b := range d.Buses {
		class := "bus"
		if b.AtTerminal {
			class = "bus terminal"
		} else if inCluster[i] {
			class = "bus bunched"
		}
		fmt.Fprintf(&sb, `<circle class="%s" cx="%.1f" cy="%d" r="5" stroke-width="2"><title>Bus %s to %s</title></circle>`,
			class, stripX(b.PositionFt, d.LengthFt), STRIP_LINE_Y, html.EscapeString(b.VehicleId), html.EscapeString(b.Destination))
	}

	fmt.Fprintf(&sb, `<text class="label" x="%d" y="%d" font-size="11">%s</text>`, STRIP_PAD-4, STRIP_LINE_Y+26, html.EscapeString(truncate(d.StartName, 26)))
	fmt.Fprintf(&sb, `<text class="label" x="%d" y="%d" font-size="11" text-anchor="end">%s</text>`, STRIP_WIDTH-STRIP_PAD+4, STRIP_LINE_Y+26, html.EscapeString(truncate(d.EndName, 26)))
	fmt.Fprintf(&sb, `<text class="label" x="%d" y="%d" font-size="11" text-anchor="middle">→</text>`, STRIP_WIDTH/2, STRIP_LINE_Y+26)
	sb.WriteString(`</svg>`)
	return sb.String()
}
