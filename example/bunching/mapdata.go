package main

// What map.js draws: each direction's pattern, its buses, and lines between bunched pairs

type mapBus struct {
	VehicleId    string     `json:"vid"`
	Position     [2]float64 `json:"pos"` // lat, lon
	Heading      float64    `json:"hdg"` // degrees clockwise from north
	AtTerminal   bool       `json:"terminal"`
	Bunched      bool       `json:"bunched"`          // in a bunched cluster, including the lead bus of one
	Convoy       int        `json:"convoy,omitempty"` // buses in that cluster
	BunchedAhead bool       `json:"bunchedAhead"`     // the gap to the bus ahead is bunched
	GapAhead     string     `json:"gapAhead,omitempty"`
	GapBehind    string     `json:"gapBehind,omitempty"`
}

type mapDirection struct {
	Direction string         `json:"direction"`
	Path      [][2]float64   `json:"path"`
	Buses     []mapBus       `json:"buses"`
	Convoys   [][][2]float64 `json:"convoys"` // one line through the buses of each bunched cluster
}

func mapData(r *liveRoute) []mapDirection {
	out := make([]mapDirection, 0, len(r.Directions))
	for _, d := range r.Directions {
		md := mapDirection{Direction: d.Direction, Buses: []mapBus{}, Convoys: [][][2]float64{}}
		for _, p := range d.Path {
			md.Path = append(md.Path, [2]float64{p.Latitude, p.Longitude})
		}
		convoySize := make(map[int]int) // bus index -> buses in its cluster
		for _, run := range d.Convoys {
			var line [][2]float64
			for i := run[0]; i <= run[1]; i++ {
				convoySize[i] = run[1] - run[0] + 1
				if d.Buses[i].Latitude != 0 { // no position from the API
					line = append(line, [2]float64{d.Buses[i].Latitude, d.Buses[i].Longitude})
				}
			}
			if len(line) >= 2 {
				md.Convoys = append(md.Convoys, line)
			}
		}
		// Buses are in service first, front to back, so Gaps[i-1] is the gap ahead of bus i and Gaps[i] the one behind
		for i, b := range d.Buses {
			mb := mapBus{VehicleId: b.VehicleId, Position: [2]float64{b.Latitude, b.Longitude}, Heading: b.Heading, AtTerminal: b.AtTerminal}
			mb.Convoy = convoySize[i]
			mb.Bunched = mb.Convoy > 0
			if !b.AtTerminal && i > 0 {
				mb.GapAhead = formatGap(d.Gaps[i-1])
				mb.BunchedAhead = d.Gaps[i-1].Bunched
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
