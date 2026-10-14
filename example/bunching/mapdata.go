package main

// What map.js draws: each direction's pattern, its buses, and lines between bunched pairs

type mapBus struct {
	VehicleId  string     `json:"vid"`
	Position   [2]float64 `json:"pos"` // lat, lon
	AtTerminal bool       `json:"terminal"`
	Bunched    bool       `json:"bunched"` // with the bus ahead
	GapAhead   string     `json:"gapAhead,omitempty"`
	GapBehind  string     `json:"gapBehind,omitempty"`
}

type mapDirection struct {
	Direction    string          `json:"direction"`
	Path         [][2]float64    `json:"path"`
	Buses        []mapBus        `json:"buses"`
	BunchedPairs [][2][2]float64 `json:"bunchedPairs"`
}

func mapData(r *liveRoute) []mapDirection {
	out := make([]mapDirection, 0, len(r.Directions))
	for _, d := range r.Directions {
		md := mapDirection{Direction: d.Direction, Buses: []mapBus{}, BunchedPairs: [][2][2]float64{}}
		for _, p := range d.Path {
			md.Path = append(md.Path, [2]float64{p.Latitude, p.Longitude})
		}
		// Buses are in service first, front to back, so Gaps[i-1] is the gap ahead of bus i and Gaps[i] the one behind
		for i, b := range d.Buses {
			mb := mapBus{VehicleId: b.VehicleId, Position: [2]float64{b.Latitude, b.Longitude}, AtTerminal: b.AtTerminal}
			if !b.AtTerminal && i > 0 {
				ahead := d.Gaps[i-1]
				mb.GapAhead = formatGap(ahead)
				mb.Bunched = ahead.Bunched
				if ahead.Bunched {
					md.BunchedPairs = append(md.BunchedPairs, [2][2]float64{{d.Buses[i-1].Latitude, d.Buses[i-1].Longitude}, mb.Position})
				}
			}
			if !b.AtTerminal && i < len(d.Gaps) {
				mb.GapBehind = formatGap(d.Gaps[i])
			}
			md.Buses = append(md.Buses, mb)
		}
		out = append(out, md)
	}
	return out
}
