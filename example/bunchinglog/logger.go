package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"adrianlehmann.net/cta"
)

const (
	MAX_ROUTES_PER_REQUEST = 10 // API limit
	MAX_BACKOFF            = 15 * time.Minute
	REQUEST_TIMEOUT        = 30 * time.Second
)

type loggerConfig struct {
	routes       []string
	interval     time.Duration
	outPath      string
	patternsPath string
}

type busLogger struct {
	client   *cta.BusClient
	cfg      loggerConfig
	patterns map[int]*PatternInfo
	out      *bufio.Writer
}

func runLogger(ctx context.Context, client *cta.BusClient, cfg loggerConfig) error {
	patterns, err := loadPatterns(cfg.patternsPath)
	if err != nil {
		return err
	}
	// Append only - restarting the logger just keeps adding to the same file
	f, err := os.OpenFile(cfg.outPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening output: %w", err)
	}
	defer f.Close()

	l := &busLogger{client: client, cfg: cfg, patterns: patterns, out: bufio.NewWriter(f)}
	slog.Info("Starting bus logger",
		slog.String("routes", strings.Join(cfg.routes, ",")),
		slog.Duration("interval", cfg.interval),
		slog.String("out", cfg.outPath))

	failures := 0
	for {
		written, ok := l.poll(ctx)
		if ok {
			failures = 0
			slog.Info("Poll done", slog.Int("observations", written))
		} else {
			failures++
		}
		wait := backoff(cfg.interval, failures)
		if failures > 0 {
			slog.Warn("Poll failed, backing off", slog.Int("consecutiveFailures", failures), slog.Duration("wait", wait))
		}
		select {
		case <-ctx.Done():
			slog.Info("Stopping bus logger")
			return nil
		case <-time.After(wait):
		}
	}
}

// Doubles the wait for every consecutive failure (rate limits, outages), capped at MAX_BACKOFF
func backoff(interval time.Duration, failures int) time.Duration {
	wait := interval
	for i := 0; i < failures && wait < MAX_BACKOFF; i++ {
		wait *= 2
	}
	return min(wait, max(interval, MAX_BACKOFF))
}

// Returns the number of observations written and whether any route request succeeded
func (l *busLogger) poll(ctx context.Context) (int, bool) {
	polledAt := time.Now()
	var vehicles []cta.Vehicle
	anyOk := false
	for start := 0; start < len(l.cfg.routes); start += MAX_ROUTES_PER_REQUEST {
		routes := l.cfg.routes[start:min(start+MAX_ROUTES_PER_REQUEST, len(l.cfg.routes))]
		reqCtx, cancel := context.WithTimeout(ctx, REQUEST_TIMEOUT)
		vs, err := l.client.GetVehiclesByRoute(reqCtx, routes)
		cancel()
		if err != nil {
			// The API reports "no vehicles right now" (e.g. outside service hours) as an error.
			// That's a successful poll for our purposes.
			if strings.Contains(err.Error(), "No data found") {
				anyOk = true
				continue
			}
			slog.Error("Getting vehicles", slog.String("routes", strings.Join(routes, ",")), slog.Any("err", err))
			continue
		}
		anyOk = true
		vehicles = append(vehicles, *vs...)
	}

	l.fetchMissingPatterns(ctx, vehicles)

	written := 0
	for _, v := range vehicles {
		o, err := l.toObservation(v, polledAt)
		if err != nil {
			slog.Warn("Skipping vehicle", slog.String("vid", v.Id), slog.Any("err", err))
			continue
		}
		b, err := json.Marshal(o)
		if err != nil {
			slog.Warn("Encoding observation", slog.String("vid", v.Id), slog.Any("err", err))
			continue
		}
		if _, err := l.out.Write(append(b, '\n')); err != nil {
			slog.Error("Writing observation", slog.Any("err", err))
			return written, false
		}
		written++
	}
	if err := l.out.Flush(); err != nil {
		slog.Error("Flushing observations", slog.Any("err", err))
		return written, false
	}
	return written, anyOk
}

func (l *busLogger) toObservation(v cta.Vehicle, polledAt time.Time) (Observation, error) {
	ts, err := v.GetTimestamp()
	if err != nil {
		return Observation{}, err
	}
	o := Observation{
		PolledAt:          polledAt,
		Timestamp:         ts,
		VehicleId:         v.Id,
		Route:             v.Route,
		PatternId:         v.PatternId,
		PatternDistanceFt: v.PatternDistanceFt,
		Delay:             v.Delay,
		TaTripId:          v.TaTripId,
		Destination:       v.Destination,
	}
	// Vehicles don't carry a direction, but their pattern does
	if p, ok := l.patterns[v.PatternId]; ok {
		o.Direction = p.Direction
	}
	pos, err := v.GetPosition()
	if err == nil {
		o.Latitude = pos.Position.Latitude
		o.Longitude = pos.Position.Longitude
	} else {
		slog.Warn("Parsing vehicle position", slog.String("vid", v.Id), slog.Any("err", err))
	}
	return o, nil
}

// Looks up patterns we haven't seen yet. Failures are logged and retried on the next poll -
// the observation is still written, the analyzer only needs the pattern file to exist by the time it runs.
func (l *busLogger) fetchMissingPatterns(ctx context.Context, vehicles []cta.Vehicle) {
	routeByPid := make(map[int]string)
	for _, v := range vehicles {
		if _, ok := l.patterns[v.PatternId]; !ok && v.PatternId != 0 {
			routeByPid[v.PatternId] = v.Route
		}
	}
	if len(routeByPid) == 0 {
		return
	}
	pids := make([]string, 0, len(routeByPid))
	for pid := range routeByPid {
		pids = append(pids, strconv.Itoa(pid))
	}
	added := 0
	for start := 0; start < len(pids); start += MAX_ROUTES_PER_REQUEST {
		batch := pids[start:min(start+MAX_ROUTES_PER_REQUEST, len(pids))]
		reqCtx, cancel := context.WithTimeout(ctx, REQUEST_TIMEOUT)
		patterns, err := l.client.GetPatternsByPid(reqCtx, batch)
		cancel()
		if err != nil {
			slog.Error("Getting patterns", slog.String("pids", strings.Join(batch, ",")), slog.Any("err", err))
			continue
		}
		for _, p := range *patterns {
			info := &PatternInfo{
				Pid:        p.Pid,
				Route:      routeByPid[p.Pid],
				Direction:  p.RouteDirection,
				LengthFeet: p.LengthFeet,
			}
			for _, pt := range p.GetStopPoints() {
				info.Stops = append(info.Stops, PatternStop{
					StopId:          pt.StopId,
					StopName:        pt.StopName,
					PatternDistance: pt.PatternDistance,
				})
			}
			l.patterns[p.Pid] = info
			added++
		}
	}
	if added == 0 {
		return
	}
	if err := savePatterns(l.cfg.patternsPath, l.patterns); err != nil {
		slog.Error("Saving patterns", slog.Any("err", err))
		return
	}
	slog.Info("Saved new patterns", slog.Int("added", added), slog.Int("total", len(l.patterns)))
}
