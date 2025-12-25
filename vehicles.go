package cta

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type busTrackerVehicleResponse struct {
	BustimeResponse struct {
		Error   []BusTrackerVehicleError `json:"error,omitempty"`
		Vehicle []Vehicle                `json:"vehicle"`
	} `json:"bustime-response"`
}

type BusTrackerVehicleError struct {
	VehicleId string `json:"vid,omitempty"`
	Route     string `json:"rt,omitempty"`
	Message   string `json:"msg"`
}

func (e BusTrackerVehicleError) String() string {
	if e.VehicleId != "" {
		return "Vehicle " + e.VehicleId + ": " + e.Message
	}
	if e.Route == "" {
		return "Route " + e.Route + ": " + e.Message
	}
	return e.Message
}

const (
	PASSENGER_LOAD_UNKNOWN    = "N/A"
	PASSENGER_LOAD_EMPTY      = "EMPTY"
	PASSENGER_LOAD_HALF_EMPTY = "HALF_EMPTY"
	PASSENGER_LOAD_FULL       = "FULL"
)

type Vehicle struct {
	Id                                string  `json:"vid"`
	RtpiDataFeed                      string  `json:"rtpidatafeed,omitempty"`
	Timestamp                         string  `json:"tmstmp"`
	Latitude                          string  `json:"lat"`
	Longitude                         string  `json:"lon"`
	Heading                           string  `json:"hdg"`
	PatternId                         int     `json:"pid"`
	PatternDistanceFt                 int     `json:"pdist"`
	Route                             string  `json:"rt"`
	Destination                       string  `json:"des"`
	Delay                             bool    `json:"dly,omitempty"`
	StopStatus                        *int    `json:"stopstatus,omitempty"`
	TimepointId                       *int    `json:"timepointid,omitempty"`
	StopId                            *string `json:"stopid,omitempty"`
	Sequence                          *int    `json:"sequence,omitempty"`
	GtfsSequence                      *int    `json:"gtfsseq,omitempty"`
	ServiceTimestamp                  string  `json:"srvtmstmp,omitempty"`
	Speed                             int     `json:"spd"`
	Blk                               *int    `json:"blk,omitempty"`
	TaBlockId                         string  `json:"tablockid"`
	TaTripId                          string  `json:"tatripid"`
	TaOriginalTripNo                  string  `json:"origtatripno"`
	Zone                              string  `json:"zone"`
	PassengerLoad                     string  `json:"psgld"`
	ScheduledStartSecondsPastMidnight int     `json:"stst"`
	ScheduledStartDate                string  `json:"stsd"`
}

func (v *Vehicle) GetPosition() (*DirectedPosition, error) {
	lat, err := strconv.ParseFloat(v.Latitude, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing latitude: %w", err)
	}
	lon, err := strconv.ParseFloat(v.Longitude, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing longitude: %w", err)
	}
	hdg, err := strconv.ParseFloat(v.Heading, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing heading: %w", err)
	}
	return &DirectedPosition{
		Position: Position{
			Latitude:  lat,
			Longitude: lon},
		Heading: hdg,
	}, nil
}

func (v *Vehicle) GetScheduleStartTimeUnix() (int64, error) {
	if v.ScheduledStartDate == "" || v.ScheduledStartSecondsPastMidnight == 0 {
		return 0, nil
	}
	t, err := time.Parse("2006-01-02", v.ScheduledStartDate)
	if err != nil {
		return 0, err
	}
	finalTime := t.Add(time.Duration(v.ScheduledStartSecondsPastMidnight) * time.Second)
	unixTimestamp := finalTime.Unix()
	return unixTimestamp, nil
}

func (v *Vehicle) GetRoute(c *BusClient, ctx context.Context) (*Route, error) {
	routestransform, err := c.GetRoutetransform(ctx)
	if err != nil {
		return nil, err
	}
	route, exists := routestransform[v.Route]
	if !exists {
		return nil, errors.New("route not found")
	}
	return route, nil
}

func (v *Vehicle) Update(c *BusClient, ctx context.Context) (*Vehicle, error) {
	vehicles, err := c.GetVehiclesById(ctx, []string{v.Id})
	if err != nil {
		return nil, err
	}
	if len(*vehicles) == 0 {
		return nil, errors.New("vehicle not found")
	}
	return &(*vehicles)[0], nil
}

func (v *Vehicle) GetPassengerLoadPct() float64 {
	switch v.PassengerLoad {
	case PASSENGER_LOAD_EMPTY:
		return 0.0
	case PASSENGER_LOAD_HALF_EMPTY:
		return 0.5
	case PASSENGER_LOAD_FULL:
		return 1.0
	default:
		return math.NaN()
	}
}

func (c *BusClient) getVehicles(ctx context.Context, query map[string]string) (*[]Vehicle, error) {
	query["tmres"] = "s"
	vehiclesResp, err := processRequest[busTrackerVehicleResponse](c.c, ctx, "getvehicles", query)
	if err != nil {
		return nil, fmt.Errorf("getting vehicles: %w", err)
	}
	if len(vehiclesResp.BustimeResponse.Error) > 0 {
		lambda := func(e BusTrackerVehicleError) string {
			return e.String()
		}
		return nil, errors.New(strings.Join(transform(vehiclesResp.BustimeResponse.Error, lambda), "; "))
	}
	return &vehiclesResp.BustimeResponse.Vehicle, nil
}

func (c *BusClient) GetVehiclesById(ctx context.Context, vehicleIds []string) (*[]Vehicle, error) {
	if len(vehicleIds) == 0 {
		return nil, errors.New("no vehicle IDs provided")
	}
	if len(vehicleIds) > 10 {
		return nil, errors.New("maximum of 10 vehicle IDs allowed")
	}
	query := map[string]string{
		"vid": strings.Join(vehicleIds, ","),
	}
	return c.getVehicles(ctx, query)
}

func (c *BusClient) GetVehiclesByRoute(ctx context.Context, routes []string) (*[]Vehicle, error) {
	if len(routes) == 0 {
		return nil, errors.New("no routes provided")
	}
	if len(routes) > 10 {
		return nil, errors.New("maximum of 10 routes allowed")
	}
	query := map[string]string{
		"rt": strings.Join(routes, ","),
	}
	return c.getVehicles(ctx, query)
}
