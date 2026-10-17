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
	SYNTH_HORIZON     = 20 * time.Minute
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
			StopId:    fmt.Sprintf("%d-%d", pid, i),
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

type synthTrip struct {
	vid      string
	pattern  cta.Pattern
	dispatch time.Time
	runtime  time.Duration
}

// Trip k leaves the terminal at k * SYNTH_HEADWAY; a slow trip takes up to 40% longer, so the next one gains on it.
// Seeded by dispatch time so the same trip looks the same on every refresh.
func synthTrips(now time.Time) []synthTrip {
	var trips []synthTrip
	for _, p := range synthPatterns() {
		first := now.Add(-2 * SYNTH_RUNTIME).Truncate(SYNTH_HEADWAY)
		for dispatch := first; dispatch.Before(now); dispatch = dispatch.Add(SYNTH_HEADWAY) {
			k := uint64(dispatch.Unix() / int64(SYNTH_HEADWAY.Seconds()))
			rng := rand.New(rand.NewPCG(k, uint64(p.Pid)))
			runtime := SYNTH_RUNTIME.Minutes() * (0.95 + 0.1*rng.Float64())
			if rng.Float64() < SYNTH_SLOW_CHANCE {
				runtime *= 1.2 + 0.2*rng.Float64()
			}
			trip := synthTrip{
				vid:      fmt.Sprintf("%d%02d", p.Pid%10, k%100), // 1xx northbound, 2xx southbound
				pattern:  p,
				dispatch: dispatch,
				runtime:  time.Duration(runtime * float64(time.Minute)),
			}
			if now.Sub(dispatch) <= trip.runtime {
				trips = append(trips, trip)
			}
		}
	}
	return trips
}

// When the trip gets to fraction f of the way along. Simulated buses run at a constant speed.
func (t synthTrip) at(f float64) time.Time {
	return t.dispatch.Add(time.Duration(f * float64(t.runtime)))
}

func synthVehicles(now time.Time) []cta.Vehicle {
	var vehicles []cta.Vehicle
	for _, t := range synthTrips(now) {
		f := float64(now.Sub(t.dispatch)) / float64(t.runtime)
		first, last := t.pattern.Points[0], t.pattern.Points[len(t.pattern.Points)-1]
		vehicles = append(vehicles, cta.Vehicle{
			Id:                t.vid,
			Timestamp:         now.In(chicago).Format("20060102 15:04:05"),
			Latitude:          strconv.FormatFloat(first.Latitude+f*(last.Latitude-first.Latitude), 'f', 6, 64),
			Longitude:         strconv.FormatFloat(first.Longitude+f*(last.Longitude-first.Longitude), 'f', 6, 64),
			Heading:           "0",
			PatternId:         t.pattern.Pid,
			PatternDistanceFt: int(f * t.pattern.LengthFeet),
			Route:             SYNTH_ROUTE,
			Destination:       last.StopName,
			Speed:             int(t.pattern.LengthFeet / FEET_PER_MILE / t.runtime.Hours()),
		})
	}
	return vehicles
}

// Predictions for the stops each bus reaches within SYNTH_HORIZON. The horizon is short on purpose so the
// biggest gaps fall outside it and use the distance fallback, like they can on the real feed.
func synthPredictions(now time.Time) []cta.Prediction {
	var predictions []cta.Prediction
	for _, t := range synthTrips(now) {
		for i, pt := range t.pattern.Points {
			at := t.at(float64(i) / SYNTH_STOPS)
			if at.Before(now) || at.Sub(now) > SYNTH_HORIZON {
				continue
			}
			predictions = append(predictions, cta.Prediction{
				VehicleId:      t.vid,
				StopId:         pt.StopId,
				StopName:       pt.StopName,
				Route:          SYNTH_ROUTE,
				RouteDirection: t.pattern.RouteDirection,
				PredictedTime:  at.In(chicago).Format("20060102 15:04:05"),
				Type:           cta.PREDICTION_TYPE_ARRIVAL,
			})
		}
	}
	return predictions
}
