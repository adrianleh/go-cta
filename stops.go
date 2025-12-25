package cta

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type busTrackerStopResponse struct {
	BustimeResponse struct {
		Error []BusTrackerStopError `json:"error,omitempty"`
		Stops []Stop                `json:"stops"`
	} `json:"bustime-response"`
}

type BusTrackerStopError struct {
	VehicleId string `json:"vid,omitempty"`
	Direction string `json:"dir,omitempty"`
	StopId    string `json:"stpid,omitempty"`
	Message   string `json:"msg"`
}

func (e BusTrackerStopError) String() string {
	if e.VehicleId != "" {
		return "Vehicle " + e.VehicleId + ": " + e.Message
	}
	if e.StopId != "" {
		return "Stop " + e.StopId + ": " + e.Message
	}
	if e.Direction != "" {
		return "Direction " + e.Direction + ": " + e.Message
	}
	return e.Message
}

type Stop struct {
	Id                 string   `json:"stpid"`
	Name               string   `json:"stpnm"`
	Latitude           float64  `json:"lat"` // Yes, this is inconsitent with Vehicle but inconsistent is the name of the game in this API
	Longitude          float64  `json:"lon"`
	DetoursAddingIds   []string `json:"dtradd,omitempty"`
	DetoursRemovingIds []string `json:"dtrrem,omitempty"`
	ADAAccessible      bool     `json:"ada,omitempty"`
}

func (s *Stop) GetPosition() Position {
	return Position{
		Latitude:  s.Latitude,
		Longitude: s.Longitude,
	}
}

func getStops(c *BusClient, ctx context.Context, query map[string]string) (*[]Stop, error) {
	stopResp, err := processRequest[busTrackerStopResponse](c.c, ctx, "getstops", query)
	if err != nil {
		return nil, fmt.Errorf("getting stops by route: %w", err)
	}
	errs := stopResp.BustimeResponse.Error
	if len(errs) > 0 {
		lambda := func(e BusTrackerStopError) string {
			return e.String()
		}
		return nil, errors.New(strings.Join(transform(errs, lambda), "; "))
	}
	return &stopResp.BustimeResponse.Stops, nil
}

func (c *BusClient) GetStopsByRouteDirection(ctx context.Context, routeId string, direction string) (*[]Stop, error) {
	query := map[string]string{
		"rt":  routeId,
		"dir": direction,
	}
	return getStops(c, ctx, query)
}

func (c *BusClient) GetStopsById(ctx context.Context, stopIds []string) (*[]Stop, error) {
	if len(stopIds) == 0 {
		return nil, errors.New("no stop IDs provided")
	}
	if len(stopIds) > 10 {
		return nil, errors.New("maximum of 10 stop IDs allowed")
	}
	query := map[string]string{
		"stpid": strings.Join(stopIds, ","),
	}
	return getStops(c, ctx, query)
}
