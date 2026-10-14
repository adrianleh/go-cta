package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"time"
)

// One line in the observations file (JSONL - one JSON object per line, so appending is cheap and restarts are safe)
type Observation struct {
	PolledAt          time.Time `json:"polled"`
	Timestamp         time.Time `json:"tmstmp"`
	VehicleId         string    `json:"vid"`
	Route             string    `json:"rt"`
	Direction         string    `json:"dir,omitempty"`
	PatternId         int       `json:"pid"`
	PatternDistanceFt int       `json:"pdist"`
	Latitude          float64   `json:"lat"`
	Longitude         float64   `json:"lon"`
	Delay             bool      `json:"dly"`
	TaTripId          string    `json:"tatripid,omitempty"`
	Destination       string    `json:"des,omitempty"`
	Synthetic         bool      `json:"synthetic,omitempty"`
}

func readObservations(path string) ([]Observation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening observations: %w", err)
	}
	defer f.Close()

	var obs []Observation
	skipped := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var o Observation
		// A crash mid-write can leave a half line at the end. Skip it rather than giving up on a week of data.
		if err := json.Unmarshal(line, &o); err != nil {
			skipped++
			continue
		}
		obs = append(obs, o)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading observations: %w", err)
	}
	if skipped > 0 {
		slog.Warn("Skipped malformed observation lines", slog.Int("count", skipped))
	}
	return obs, nil
}

type PatternStop struct {
	StopId          string  `json:"stpid"`
	StopName        string  `json:"stpnm"`
	PatternDistance float64 `json:"pdist"`
}

// What we keep of a pattern: enough to map a stop to a distance along each pattern
type PatternInfo struct {
	Pid        int           `json:"pid"`
	Route      string        `json:"rt"`
	Direction  string        `json:"rtdir"`
	LengthFeet float64       `json:"ln"`
	Stops      []PatternStop `json:"stops"`
}

func (p *PatternInfo) stopDistance(stopId string) (float64, bool) {
	for _, s := range p.Stops {
		if s.StopId == stopId {
			return s.PatternDistance, true
		}
	}
	return 0, false
}

func loadPatterns(path string) (map[int]*PatternInfo, error) {
	patterns := make(map[int]*PatternInfo)
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return patterns, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading patterns: %w", err)
	}
	var list []*PatternInfo
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, fmt.Errorf("decoding patterns: %w", err)
	}
	for _, p := range list {
		patterns[p.Pid] = p
	}
	return patterns, nil
}

func savePatterns(path string, patterns map[int]*PatternInfo) error {
	list := make([]*PatternInfo, 0, len(patterns))
	for _, p := range patterns {
		list = append(list, p)
	}
	slices.SortFunc(list, func(a, b *PatternInfo) int { return a.Pid - b.Pid })
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding patterns: %w", err)
	}
	// Write to a temp file and rename so a crash never leaves a half-written patterns file
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("writing patterns: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("renaming patterns: %w", err)
	}
	return nil
}
