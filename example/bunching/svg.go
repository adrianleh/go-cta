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

// Colors for the standalone snapshot. On the page the same classes are styled by page.html (with dark mode).
const SNAPSHOT_STYLE = `<style>
.surface { fill: #fcfcfb; }
.track { stroke: #e4e3df; }
.gap { stroke: #2a78d6; }
.gap.bunched { stroke: #d03b3b; }
.bus { fill: #0b0b0b; stroke: #fcfcfb; }
.bus.terminal { fill: #fcfcfb; stroke: #52514e; }
.label { fill: #52514e; }
.label.strong, .title { fill: #0b0b0b; }
</style>`

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
	fmt.Fprintf(&sb, `<svg class="strip" viewBox="0 0 %d %d" role="img" aria-label="%s">`, STRIP_WIDTH, STRIP_HEIGHT, html.EscapeString(headline(d)))
	fmt.Fprintf(&sb, `<line class="track" x1="%d" x2="%d" y1="%d" y2="%d" stroke-width="6" stroke-linecap="round"/>`,
		STRIP_PAD, STRIP_WIDTH-STRIP_PAD, STRIP_LINE_Y, STRIP_LINE_Y)

	for _, g := range d.Gaps {
		x1, x2 := stripX(g.FromFt, d.LengthFt), stripX(g.ToFt, d.LengthFt)
		class := "gap"
		if g.Bunched {
			class = "gap bunched"
		}
		fmt.Fprintf(&sb, `<line class="%s" x1="%.1f" x2="%.1f" y1="%d" y2="%d" stroke-width="6"><title>%s min gap</title></line>`,
			class, x1, x2, STRIP_LINE_Y, STRIP_LINE_Y, formatMinutes(g.Minutes))
		mid := (x1 + x2) / 2
		// Bunched gaps are always labeled (they're short, so the label sits above with a tick). Others only if they fit.
		if g.Bunched {
			fmt.Fprintf(&sb, `<line class="gap bunched" x1="%.1f" x2="%.1f" y1="%d" y2="%d" stroke-width="1.5"/>`, mid, mid, STRIP_LINE_Y-20, STRIP_LINE_Y-6)
			fmt.Fprintf(&sb, `<text class="label strong" x="%.1f" y="%d" font-size="10" font-weight="700" text-anchor="middle">bunched</text>`, mid, STRIP_LINE_Y-24)
		} else if x2-x1 >= 22 {
			fmt.Fprintf(&sb, `<text class="label" x="%.1f" y="%d" font-size="10" text-anchor="middle">%sm</text>`, mid, STRIP_LINE_Y-10, formatWholeMinutes(g.Minutes))
		}
	}

	for _, b := range d.Buses {
		class := "bus"
		if b.AtTerminal {
			class = "bus terminal"
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

// A standalone picture of the whole route at one moment, for sharing
func snapshotSVG(r *liveRoute) string {
	const width = 2 * STRIP_WIDTH
	const blockHeight = 2*STRIP_HEIGHT + 64
	height := 84 + len(r.Directions)*blockHeight

	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="-apple-system, Helvetica, Arial, sans-serif">`, width, height, width, height)
	sb.WriteString(SNAPSHOT_STYLE)
	fmt.Fprintf(&sb, `<rect class="surface" width="%d" height="%d"/>`, width, height)
	fmt.Fprintf(&sb, `<text class="title" x="20" y="34" font-size="20" font-weight="700">%s</text>`, html.EscapeString(routeTitle(r)))
	fmt.Fprintf(&sb, `<text class="label" x="20" y="58" font-size="13">%s</text>`, html.EscapeString(snapshotCaption(r)))

	for i, d := range r.Directions {
		top := 84 + i*blockHeight
		fmt.Fprintf(&sb, `<text class="title" x="20" y="%d" font-size="16" font-weight="700">%s</text>`, top+18, html.EscapeString(d.Direction))
		line1, line2 := headlineLines(d)
		fmt.Fprintf(&sb, `<text class="label strong" x="20" y="%d" font-size="15">%s</text>`, top+40, html.EscapeString(line1))
		fmt.Fprintf(&sb, `<text class="label" x="20" y="%d" font-size="15">%s</text>`, top+60, html.EscapeString(line2))
		// Nest the strip and let it scale up 2x
		strip := strings.Replace(stripSVG(d), `<svg class="strip"`, fmt.Sprintf(`<svg class="strip" x="0" y="%d" width="%d" height="%d"`, top+64, width, 2*STRIP_HEIGHT), 1)
		sb.WriteString(strip)
	}
	sb.WriteString(`</svg>` + "\n")
	return sb.String()
}
