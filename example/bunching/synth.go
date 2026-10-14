package main

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"time"

	"adrianlehmann.net/cta"
)

// Fake buses for trying the site without an API key (-synthetic). Everything here is made up and labeled as such.
// A straight line from roughly Jackson Park to downtown, buses every 9 minutes, and some trips running slow so
// the bus behind catches up - the basic bunching mechanism.

const (
	SYNTH_ROUTE       = "6"
	SYNTH_HEADWAY     = 9 * time.Minute
	SYNTH_RUNTIME     = 60 * time.Minute
	SYNTH_STOPS       = 40
	SYNTH_SOUTH_LAT   = 41.765
	SYNTH_SOUTH_LON   = -87.575
	SYNTH_NORTH_LAT   = 41.885
	SYNTH_NORTH_LON   = -87.627
	SYNTH_NORTHBOUND  = 9601
	SYNTH_SOUTHBOUND  = 9602
	SYNTH_SLOW_CHANCE = 0.3
)

func synthRoute() cta.Route {
	return cta.Route{RouteId: SYNTH_ROUTE, RouteName: "Jackson Park Express (synthetic)", RouteColor: "#2a78d6"}
}

func synthPatterns() []cta.Pattern {
	return []cta.Pattern{
		synthPattern(SYNTH_NORTHBOUND, "Northbound", false),
		synthPattern(SYNTH_SOUTHBOUND, "Southbound", true),
	}
}

func synthPattern(pid int, direction string, reverse bool) cta.Pattern {
	p := cta.Pattern{Pid: pid, RouteDirection: direction}
	for i := 0; i <= SYNTH_STOPS; i++ {
		f := float64(i) / SYNTH_STOPS
		if reverse {
			f = 1 - f
		}
		name := fmt.Sprintf("Synthetic stop %d", i)
		if i == 0 || i == SYNTH_STOPS {
			name = "Downtown (synthetic)"
			if (i == 0) != reverse {
				name = "Jackson Park (synthetic)"
			}
		}
		p.Points = append(p.Points, cta.PatternPoint{
			Sequence:  i + 1,
			Type:      "S",
			StopName:  name,
			Latitude:  SYNTH_SOUTH_LAT + f*(SYNTH_NORTH_LAT-SYNTH_SOUTH_LAT),
			Longitude: SYNTH_SOUTH_LON + f*(SYNTH_NORTH_LON-SYNTH_SOUTH_LON),
		})
	}
	p.LengthFeet = newRefPattern(p).lengthFt
	for i := range p.Points {
		p.Points[i].PatternDistance = p.LengthFeet * float64(i) / SYNTH_STOPS
	}
	return p
}

// Where every synthetic bus is at time now. Trip k leaves the terminal at k * SYNTH_HEADWAY;
// a slow trip takes up to 40% longer, so the next one gains on it.
func synthVehicles(now time.Time) []cta.Vehicle {
	var vehicles []cta.Vehicle
	for _, p := range synthPatterns() {
		first := now.Add(-2 * SYNTH_RUNTIME).Truncate(SYNTH_HEADWAY)
		for dispatch := first; dispatch.Before(now); dispatch = dispatch.Add(SYNTH_HEADWAY) {
			k := uint64(dispatch.Unix()/int64(SYNTH_HEADWAY.Seconds())) + uint64(p.Pid)
			rng := rand.New(rand.NewPCG(k, 6))
			runtime := SYNTH_RUNTIME.Minutes() * (0.95 + 0.1*rng.Float64())
			if rng.Float64() < SYNTH_SLOW_CHANCE {
				runtime *= 1.2 + 0.2*rng.Float64()
			}
			f := now.Sub(dispatch).Minutes() / runtime
			if f > 1 {
				continue
			}
			lat := p.Points[0].Latitude + f*(p.Points[len(p.Points)-1].Latitude-p.Points[0].Latitude)
			lon := p.Points[0].Longitude + f*(p.Points[len(p.Points)-1].Longitude-p.Points[0].Longitude)
			vehicles = append(vehicles, cta.Vehicle{
				Id:                strconv.FormatUint(8000+k%1000, 10),
				Timestamp:         now.In(chicago).Format("20060102 15:04:05"),
				Latitude:          strconv.FormatFloat(lat, 'f', 6, 64),
				Longitude:         strconv.FormatFloat(lon, 'f', 6, 64),
				Heading:           "0",
				PatternId:         p.Pid,
				PatternDistanceFt: int(f * p.LengthFeet),
				Route:             SYNTH_ROUTE,
				Destination:       p.Points[len(p.Points)-1].StopName,
				Speed:             int(p.LengthFeet / FEET_PER_MILE / (runtime / 60)),
			})
		}
	}
	return vehicles
}
