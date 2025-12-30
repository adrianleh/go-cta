package cta

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
)

type trainTrackerArrivalsResponse struct {
	Timestamp    string    `json:"tmst"`
	ErrorCode    uint8     `json:"errCd"`
	ErrorMessage string    `json:"errNm"`
	Arrivvals    []Arrival `json:"eta"`
}

// Eta represents the nested eta elements in the JSON response.
type Arrival struct {
	GTFSStationId            string `json:"staId"`
	GTFSStopId               string `json:"stpId"`
	StationName              string `json:"staNm"`
	StationDescription       string `json:"stpDe"`
	RunNumber                string `json:"rn"`
	RouteName                string `json:"rt"`
	GTFSDestinationStationId string `json:"destSt"`
	DestinationName          string `json:"destNm"`
	TrainRouteDirectionCode  string `json:"trDr"`
	PredictionGenerationTime string `json:"prdt"`
	ArrivalTime              string `json:"arrT"`
	IsApproachingRaw         string `json:"isApp"`
	IsScheduledRaw           string `json:"isSch"`
	IsDelayedRaw             string `json:"isDly"`
	IsFaultRaw               string `json:"isFlt"`
	FlagsUnused              string `json:"flags"` // unused
	Latitude                 string `json:"lat"`
	Longitude                string `json:"lon"`
	Heading                  string `json:"heading"`
}

func (arrival *Arrival) GetPredictionGenerationTime() (time.Time, error) {
	return parseCtaTrainTrackerTime(arrival.PredictionGenerationTime)
}
func (arrival *Arrival) GetUnixPredictionGenerationTime() (int64, error) {
	t, err := arrival.GetPredictionGenerationTime()
	return errMap(t, err, func(t time.Time) int64 {
		return t.Unix()
	})
}

func (arrival *Arrival) GetArrivalTime() (time.Time, error) {
	return parseCtaTrainTrackerTime(arrival.ArrivalTime)
}

func (arrival *Arrival) GetUnixArrivalTime() (int64, error) {
	t, err := arrival.GetArrivalTime()
	return errMap(t, err, func(t time.Time) int64 {
		return t.Unix()
	})
}

func (arrival *Arrival) GetTrainDirection() (int64, error) {
	return strconv.ParseInt(arrival.TrainRouteDirectionCode, 10, 64)
}

func (arrival *Arrival) GetPosition() (DirectedPosition, error) {
	lat, err := strconv.ParseFloat(arrival.Latitude, 64)
	if err != nil {
		return DirectedPosition{}, fmt.Errorf("parsing latitude: %w", err)
	}
	lon, err := strconv.ParseFloat(arrival.Longitude, 64)
	if err != nil {
		return DirectedPosition{}, fmt.Errorf("parsing longitude: %w", err)
	}
	heading, err := strconv.ParseFloat(arrival.Heading, 64)
	if err != nil {
		return DirectedPosition{}, fmt.Errorf("parsing heading: %w", err)
	}
	return DirectedPosition{
		Position: Position{
			Latitude:  lat,
			Longitude: lon,
		},
		Heading: heading,
	}, nil
}

func (arrival *Arrival) IsApproaching() bool {
	return arrival.IsApproachingRaw == "1"
}

func (arrival *Arrival) IsDue() bool {
	return arrival.IsApproaching()
}

func (arrival *Arrival) IsScheduled() bool {
	return arrival.IsScheduledRaw == "1"
}

func (arrival *Arrival) IsDelayed() bool {
	return arrival.IsDelayedRaw == "1"
}

func (arrival *Arrival) IsFault() bool {
	return arrival.IsFaultRaw == "1"
}

func (arrival *Arrival) GetArrivalFriendlyTrainRouteName() string {
	return GetFriendlyTrainRouteName(arrival.RouteName)
}

func (arrival *Arrival) IsGhostTrain() bool {
	return arrival.IsScheduled() && !arrival.IsApproaching() && !arrival.IsDelayed() && !arrival.IsFault()
}

func (arrival *Arrival) IsRegularCountDown() bool {
	return !arrival.IsScheduled() && !arrival.IsDelayed() && !arrival.IsFault()
}

func (arrival *Arrival) GetArrivalMinutesFractionalSeconds() float64 {
	if !arrival.IsRegularCountDown() {
		return -1
	}
	if arrival.IsDue() {
		return 0
	}
	arrivalTime, err := arrival.GetArrivalTime()
	if err != nil {
		return -1
	}
	predictonGenerationTime, err := arrival.GetPredictionGenerationTime()
	if err != nil {
		return -1
	}
	return arrivalTime.Sub(predictonGenerationTime).Minutes()
}

func (arrival *Arrival) GetArrivalMinutes() int64 {
	minutesFractional := arrival.GetArrivalMinutesFractionalSeconds()
	if minutesFractional < 0 {
		return -1
	}
	return int64(minutesFractional + 0.5) // Round to nearest minute
}

func (arrival *Arrival) GetFriendlyTime(allowScheduledTrains bool) string {
	if arrival.IsApproaching() {
		return "Due"
	}
	if arrival.IsDelayed() {
		return "Delayed"
	}
	if arrival.IsFault() {
		return "Fault"
	}
	mins := strconv.FormatInt(arrival.GetArrivalMinutes(), 10)
	if arrival.IsScheduled() {
		if allowScheduledTrains {
			return mins + " min (Scheduled)"
		}
		return "Ghost"
	}
	return mins + " min"
}

func (c *TrainClient) getArrivals(ctx context.Context, query map[string]string) (*[]Arrival, error) {
	query["outputType"] = "JSON"
	detourResp, err := processRequest[trainTrackerArrivalsResponse](c.c, ctx, "ttarrivals.aspx", query)
	if err != nil {
		return nil, fmt.Errorf("getting detours: %w", err)
	}
	errs := detourResp.ErrorMessage
	if len(errs) > 0 {
		return nil, errors.New(errs)
	}
	return &detourResp.Arrivvals, nil
}

func (c *TrainClient) getArrivalsWithStation(ctx context.Context, stationId string, query map[string]string) (*[]Arrival, error) {
	query["mapid"] = stationId
	return c.getArrivals(ctx, query)
}

func (c *TrainClient) getArrivalsWithStop(ctx context.Context, stopId string, query map[string]string) (*[]Arrival, error) {
	query["stpid"] = stopId
	return c.getArrivals(ctx, query)
}

func (c *TrainClient) getArrivalsWithStopOrStation(ctx context.Context, id string, query map[string]string, station bool) (*[]Arrival, error) {
	if station {
		return c.getArrivalsWithStation(ctx, id, query)
	} else {
		return c.getArrivalsWithStop(ctx, id, query)
	}
}

func (c *TrainClient) getArrivalsByStopOrStationAndRouteWithLimit(ctx context.Context, route string, stopId string, limit uint64, station bool) (*[]Arrival, error) {
	routeCode := route
	if !IsValidTrainRoute(route) {
		if IsValidFriendlyTrainRouteName(route) {
			routeCode, _ = FriendlyNameToTrainRoute(route)
		}
		return nil, fmt.Errorf("invalid train route: %s", route)
	}
	query := map[string]string{
		"rt": routeCode,
	}
	if limit > 0 {
		query["max"] = strconv.FormatUint(limit, 10)
	}
	return c.getArrivalsWithStopOrStation(ctx, stopId, query, station)
}

func (c *TrainClient) GetArrivalsByStopIdWithLimit(ctx context.Context, stopId string, limit uint64) (*[]Arrival, error) {
	query := map[string]string{}
	if limit > 0 {
		query["max"] = strconv.FormatUint(limit, 10)
	}
	return c.getArrivalsWithStop(ctx, stopId, query)
}

func (c *TrainClient) GetArrivalsByStopId(ctx context.Context, stopId string) (*[]Arrival, error) {
	return c.GetArrivalsByStopIdWithLimit(ctx, stopId, 0)
}

func (c *TrainClient) GetArrivalsByStationIdWithLimit(ctx context.Context, stationId string, limit uint64) (*[]Arrival, error) {
	query := map[string]string{}
	if limit > 0 {
		query["max"] = strconv.FormatUint(limit, 10)
	}
	return c.getArrivalsWithStop(ctx, stationId, query)
}

func (c *TrainClient) GetArrivalsByStationId(ctx context.Context, stationId string) (*[]Arrival, error) {
	return c.GetArrivalsByStationIdWithLimit(ctx, stationId, 0)
}

func (c *TrainClient) GetArrivalsByStopAndRouteWithLimit(ctx context.Context, route string, stopId string, limit uint64) (*[]Arrival, error) {
	return c.getArrivalsByStopOrStationAndRouteWithLimit(ctx, route, stopId, limit, false)
}

func (c *TrainClient) GetArrivalsByStationAndRouteWithLimit(ctx context.Context, route string, stationId string, limit uint64) (*[]Arrival, error) {
	return c.getArrivalsByStopOrStationAndRouteWithLimit(ctx, route, stationId, limit, true)
}

func (c *TrainClient) GetArrivalsByStationAndRoute(ctx context.Context, stationId string, route string) (*[]Arrival, error) {
	return c.GetArrivalsByStationAndRouteWithLimit(ctx, route, stationId, 0)
}

func (c *TrainClient) GetArrivalsByStopAndRoute(ctx context.Context, stopId string, route string) (*[]Arrival, error) {
	return c.GetArrivalsByStopAndRouteWithLimit(ctx, route, stopId, 0)
}
