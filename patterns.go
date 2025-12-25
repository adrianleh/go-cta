package cta

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type busTrackerPatternResponse struct {
	BustimeResponse struct {
		Error []BusTrackerPatternError `json:"error,omitempty"`
		Ptr   []Pattern                `json:"ptr"`
	} `json:"bustime-response"`
}

type BusTrackerPatternError struct {
	RtPidatafeed string `json:"rtpidatafeed,omitempty"`
	Pid          string `json:"pid,omitempty"`
	Rt           string `json:"rt,omitempty"`
	Msg          string `json:"msg"`
}

func (e BusTrackerPatternError) String() string {
	if e.Pid != "" {
		return "Pattern " + e.Pid + ": " + e.Msg
	}
	if e.Rt != "" {
		return "Route " + e.Rt + ": " + e.Msg
	}
	return e.Msg
}

type Pattern struct {
	Pid            int            `json:"pid"`
	LengthFeet     float64        `json:"ln"`
	RouteDirection string         `json:"rtdir"`
	Points         []PatternPoint `json:"pt"`
	DetourId       string         `json:"dtrid,omitempty"`
	DetourPoints   []PatternPoint `json:"dtrpt,omitempty"`
}

type PatternPoint struct {
	Sequence        int     `json:"seq"`
	Type            string  `json:"typ"`
	StopId          string  `json:"stpid,omitempty"`
	StopName        string  `json:"stpnm,omitempty"`
	PatternDistance float64 `json:"pdist,omitempty"`
	Latitude        float64 `json:"lat"`
	Longitude       float64 `json:"lon"`
}

func (p *PatternPoint) GetPosition() Position {
	return Position{
		Latitude:  p.Latitude,
		Longitude: p.Longitude,
	}
}

func (p *PatternPoint) IsStop() bool {
	return p.Type == "S"
}

func (p *PatternPoint) IsWayPoint() bool {
	return p.Type == "W"
}

func (p *PatternPoint) GetStop(c *BusClient, ctx context.Context) (*Stop, error) {
	if !p.IsStop() {
		return nil, errors.New("pattern point is not a stop")
	}
	stops, err := c.GetStopsById(ctx, []string{p.StopId})
	if err != nil {
		return nil, fmt.Errorf("getting stop for pattern point: %w", err)
	}
	if len(*stops) == 0 {
		return nil, errors.New("no stop found for pattern point")
	}
	return &(*stops)[0], nil
}

func (p *Pattern) GetStops(c *BusClient, ctx context.Context) (*[]Stop, error) {
	// Careful with this method as it can result in lots of API calls and rate limiting
	stopPoints := filter(p.Points, func(pt PatternPoint) bool {
		return pt.IsStop()
	})
	stopIds := transform(stopPoints, func(pt PatternPoint) string {
		return pt.StopId
	})
	chunkedStopIds := chunk(stopIds, 10) // API limit of 10 stop IDs per request - so chunking
	var allStops []Stop
	for _, chunk := range chunkedStopIds {
		stops, err := c.GetStopsById(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("getting stops for pattern: %w", err)
		}
		allStops = append(allStops, *stops...)
	}
	return &allStops, nil
}

func getPatterns(c *BusClient, ctx context.Context, query map[string]string) (*[]Pattern, error) {
	patResp, err := processRequest[busTrackerPatternResponse](c.c, ctx, "getpatterns", query)
	if err != nil {
		return nil, fmt.Errorf("getting patterns: %w", err)
	}
	errs := patResp.BustimeResponse.Error
	if len(errs) > 0 {
		lambda := func(e BusTrackerPatternError) string { return e.String() }
		return nil, errors.New(strings.Join(transform(errs, lambda), "; "))
	}
	return &patResp.BustimeResponse.Ptr, nil
}

func (c *BusClient) GetPatternsByRoute(ctx context.Context, routeId string) (*[]Pattern, error) {
	if routeId == "" {
		return nil, errors.New("no route provided")
	}
	query := map[string]string{
		"rt": routeId,
	}
	return getPatterns(c, ctx, query)
}

func (c *BusClient) GetPatternsByPid(ctx context.Context, pids []string) (*[]Pattern, error) {
	if len(pids) == 0 {
		return nil, errors.New("no pattern IDs provided")
	}
	if len(pids) > 10 {
		return nil, errors.New("maximum of 10 pattern IDs allowed")
	}
	query := map[string]string{
		"pid": strings.Join(pids, ","),
	}
	return getPatterns(c, ctx, query)
}
