package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"time"
)

// Synthetic data for trying the analyzer without an API key (and for the README example).
// Every observation is flagged synthetic so the analyzer can say so loudly. None of this is CTA data.

type synthConfig struct {
	outPath      string
	patternsPath string
	days         int
	seed         uint64
}

const (
	SYNTH_ROUTE        = "6"
	SYNTH_LENGTH_FT    = 58000
	SYNTH_STOP_SPACING = 1300
	SYNTH_RUNTIME_MIN  = 55
)

// Scheduled headway in minutes for a dispatch at time t, 0 means no service
func synthScheduledHeadway(t time.Time) float64 {
	switch periodOf(t) {
	case PERIOD_AM_PEAK, PERIOD_PM_PEAK:
		return 8
	case PERIOD_MIDDAY:
		return 10
	case PERIOD_EVENING:
		return 12
	case PERIOD_WEEKEND:
		if t.In(chicago).Hour() < 5 {
			return 0
		}
		return 12
	default:
		if t.In(chicago).Hour() < 5 {
			return 0
		}
		return 15
	}
}

// Peaks are made worse on purpose so the example output has something to show
func synthLateChance(t time.Time) float64 {
	switch periodOf(t) {
	case PERIOD_PM_PEAK:
		return 0.5
	case PERIOD_AM_PEAK:
		return 0.4
	default:
		return 0.25
	}
}

type synthTrip struct {
	vid      string
	pid      int
	dispatch time.Time
	offset   float64 // minutes late leaving the terminal
	runtime  float64 // minutes end to end
}

// Where the trip is at time t as a fraction of the pattern, ok=false if it isn't running
func (s synthTrip) fraction(t time.Time) (float64, bool) {
	x := (t.Sub(s.dispatch).Minutes() - s.offset) / s.runtime
	return x, x >= 0 && x <= 1
}

func runSynth(cfg synthConfig) error {
	rng := rand.New(rand.NewPCG(cfg.seed, cfg.seed))
	patterns := map[int]*PatternInfo{
		99001: synthPattern(99001, "Northbound"),
		99002: synthPattern(99002, "Southbound"),
	}

	// Start on a Monday so every period shows up
	start := time.Date(2026, time.October, 5, 4, 0, 0, 0, chicago)
	end := start.AddDate(0, 0, cfg.days)

	var trips []synthTrip
	vehicle := 0
	for pid := range patterns {
		for t := start; t.Before(end); {
			h := synthScheduledHeadway(t)
			if h == 0 {
				t = t.Add(10 * time.Minute)
				continue
			}
			// A late trip picks up more riders and falls further behind, while its follower catches up -
			// that's the bunching mechanism, roughly. Some trips just run a bit fast or slow.
			runtime := SYNTH_RUNTIME_MIN + rng.NormFloat64()*3
			if rng.Float64() < synthLateChance(t) {
				runtime += h * (0.4 + 0.5*rng.Float64())
			}
			vehicle++
			trips = append(trips, synthTrip{
				vid:      fmt.Sprintf("%d", 8000+vehicle%400),
				pid:      pid,
				dispatch: t,
				offset:   max(rng.NormFloat64()*1.5, -1),
				runtime:  max(runtime, 35),
			})
			t = t.Add(time.Duration(h * float64(time.Minute)))
		}
	}

	f, err := os.Create(cfg.outPath)
	if err != nil {
		return fmt.Errorf("creating output: %w", err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)

	// A simulated 25 minute logger outage on the Wednesday afternoon, to exercise gap handling
	outageStart := start.AddDate(0, 0, 2).Add(12 * time.Hour)
	outageEnd := outageStart.Add(25 * time.Minute)

	for poll := start; poll.Before(end); poll = poll.Add(time.Minute) {
		if !poll.Before(outageStart) && poll.Before(outageEnd) {
			continue
		}
		for _, trip := range trips {
			// Each bus reports a few seconds before the poll
			ts := poll.Add(-time.Duration(rng.IntN(30)) * time.Second)
			x, ok := trip.fraction(ts)
			if !ok {
				continue
			}
			o := Observation{
				PolledAt:          poll,
				Timestamp:         ts,
				VehicleId:         trip.vid,
				Route:             SYNTH_ROUTE,
				Direction:         patterns[trip.pid].Direction,
				PatternId:         trip.pid,
				PatternDistanceFt: int(x * SYNTH_LENGTH_FT),
				Synthetic:         true,
			}
			b, err := json.Marshal(o)
			if err != nil {
				return fmt.Errorf("encoding observation: %w", err)
			}
			if _, err := w.Write(append(b, '\n')); err != nil {
				return fmt.Errorf("writing observation: %w", err)
			}
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("flushing observations: %w", err)
	}
	return savePatterns(cfg.patternsPath, patterns)
}

func synthPattern(pid int, direction string) *PatternInfo {
	p := &PatternInfo{Pid: pid, Route: SYNTH_ROUTE, Direction: direction, LengthFeet: SYNTH_LENGTH_FT}
	for i, d := 0, 0; d <= SYNTH_LENGTH_FT; i, d = i+1, d+SYNTH_STOP_SPACING {
		p.Stops = append(p.Stops, PatternStop{
			StopId:          fmt.Sprintf("S%d-%02d", pid, i),
			StopName:        fmt.Sprintf("Synthetic stop %d", i),
			PatternDistance: float64(d),
		})
	}
	return p
}
