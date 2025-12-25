package cta

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type busTrackerPredictionsResponse struct {
	BustimeResponse struct {
		Error      []BusTrackerPredictionsError `json:"error,omitempty"`
		Prediction []Prediction                 `json:"prd"`
	} `json:"bustime-response"`
}

type BusTrackerPredictionsError struct {
	Vid   string `json:"vid,omitempty"`
	StpId string `json:"stpid,omitempty"`
	Rt    string `json:"rt,omitempty"`
	Msg   string `json:"msg"`
}

const (
	PREDICTION_TYPE_ARRIVAL   = "A"
	PREDICTION_TYPE_DEPARTURE = "D"
)

type Prediction struct {
	Timestamp                             string  `json:"tmstmp"`
	Type                                  string  `json:"typ"`
	StopId                                string  `json:"stpid"`
	StopName                              string  `json:"stpnm"`
	VehicleId                             string  `json:"vid"`
	DistanceToStopFt                      float64 `json:"dstp"`
	Route                                 string  `json:"rt"`
	RouteDesignator                       string  `json:"rtdd"`
	RouteDirection                        string  `json:"rtdir"`
	Destination                           string  `json:"des"`
	PredictedTime                         string  `json:"prdtm"`
	Delay                                 bool    `json:"dly"`
	DynamicIndentifier                    int16   `json:"dyn,omitempty"`
	TaBlockId                             string  `json:"tablockid,omitempty"`
	TaTripId                              string  `json:"tatripid,omitempty"`
	TaScheduledTripid                     string  `json:"origtatripno,omitempty"`
	MinutesAway                           string  `json:"prdctdn,omitempty"` // This can be DLY
	Zone                                  string  `json:"zone,omitempty"`
	PassengerLoad                         string  `json:"psgld,omitempty"`
	ScheduledTripStartSecondsPastMidnight int     `json:"stst,omitempty"`
	ScheduledTripStartDate                string  `json:"stsd,omitempty"`
	Flagstop                              int     `json:"flagstop,omitempty"`
}

func (p *Prediction) IsMinutesAwayJustDelay() bool {
	return p.MinutesAway == "DLY"
}

func (p *Prediction) GetMinutesAwayAsInt() (uint64, error) {
	if p.IsMinutesAwayJustDelay() {
		return 0, errors.New("so delayed that there is no prediction available")
	}
	if p.MinutesAway == "" {
		return 0, errors.New("minutes away is empty")
	}
	mins, err := strconv.ParseUint(p.MinutesAway, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing minutes away: %w", err)
	}
	return mins, nil
}

func (p *Prediction) GetUnixTimestamp() (int64, error) {
	unixTime, err := strconv.ParseInt(p.Timestamp, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing unix time: %w", err)
	}
	return unixTime, nil
}

func (p *Prediction) GetTimestamp() (time.Time, error) {
	unixTime, err := p.GetUnixTimestamp()
	if err != nil {
		return time.Time{}, err
	}
	return unixTimeToTimeUnknownGranularity(unixTime), nil
}

func (p *Prediction) GetPredictedUnixTime() (int64, error) {
	unixTime, err := strconv.ParseInt(p.PredictedTime, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing predicted unix time: %w", err)
	}
	return unixTime, nil
}

func (p *Prediction) GetPredictedTime() (time.Time, error) {
	unixTime, err := p.GetPredictedUnixTime()
	if err != nil {
		return time.Time{}, err
	}
	return unixTimeToTimeUnknownGranularity(unixTime), nil
}

func (p *Prediction) IsArrival() bool {
	return p.Type == PREDICTION_TYPE_ARRIVAL
}

func (p *Prediction) IsDeparture() bool {
	return p.Type == PREDICTION_TYPE_DEPARTURE
}

func (p *Prediction) GetTypeFriendly() string {
	if p.IsArrival() {
		return "Arrival"
	} else if p.IsDeparture() {
		return "Departure"
	}
	return "Unknown"
}

func (p *Prediction) IsFlagstop() (bool, error) {
	if !p.IsFlagstopStateKnown() {
		return false, errors.New("flagstop state unknown")
	}
	return p.Flagstop == 1 || p.Flagstop == 2, nil
}

func (p *Prediction) IsNonFlagStop() (bool, error) {
	if !p.IsFlagstopStateKnown() {
		return false, errors.New("flagstop state unknown")
	}
	return p.Flagstop == 0, nil
}

func (p *Prediction) IsFlagstopStateKnown() bool {
	return p.Flagstop == 0 || p.Flagstop == 1 || p.Flagstop == 2
}

func (p *Prediction) IsDischargeOnlyFlagstop() (bool, error) {
	if !p.IsFlagstopStateKnown() {
		return false, errors.New("flagstop state unknown")
	}
	return p.Flagstop == 2, nil
}

func (p *Prediction) GetFlagStopFriendly() string {
	if !p.IsFlagstopStateKnown() {
		return "Unknown"
	}
	isFlag, _ := p.IsFlagstop()
	isDischargeOnly, _ := p.IsDischargeOnlyFlagstop()
	if !isFlag {
		return "Regular Stop"
	}
	if isDischargeOnly {
		return "Discharge Only Flagstop"
	}
	return "Flagstop"
}

func (p *Prediction) GetDynamicIndentifierName() string {
	return DynamicActionTypeName(int(p.DynamicIndentifier))
}

func (p *Prediction) GetDynamicIndentifierDescription() string {
	return DynamicActionTypeDescription(int(p.DynamicIndentifier))
}

func (p *Prediction) GetDynamicIndentifierNameES() string {
	return DynamicActionTypeNameES(int(p.DynamicIndentifier))
}

func (p *Prediction) GetDynamicIndentifierDescriptionES() string {
	return DynamicActionTypeDescriptionES(int(p.DynamicIndentifier))
}

func (p *Prediction) GetDynamicIndentifierNameByClientLocale(c *BusClient) string {
	if c.c.locale == "es" {
		return DynamicActionTypeNameES(int(p.DynamicIndentifier))
	}
	return DynamicActionTypeName(int(p.DynamicIndentifier))
}

func (p *Prediction) GetDynamicIndentifierDescriptionByClientLocale(c *BusClient) string {
	if c.c.locale == "es" {
		return DynamicActionTypeDescriptionES(int(p.DynamicIndentifier))
	}
	return DynamicActionTypeDescription(int(p.DynamicIndentifier))
}

const NO_PREDICTION_LIMIT = -1

func (e BusTrackerPredictionsError) String() string {
	if e.Vid != "" {
		return "Vehicle " + e.Vid + ": " + e.Msg
	}
	if e.StpId != "" {
		return "Stop " + e.StpId + ": " + e.Msg
	}
	if e.Rt != "" {
		return "Route " + e.Rt + ": " + e.Msg
	}
	return e.Msg
}

func (c *BusClient) getPredictions(ctx context.Context, query map[string]string) (*[]Prediction, error) {
	query["tmres"] = "s"
	query["unixtime"] = "True" // <- Sane time format without timezone issue. Convert in front-end if needed.
	preditionResponses, err := processRequest[busTrackerPredictionsResponse](c.c, ctx, "getpredictions", query)
	if err != nil {
		return nil, fmt.Errorf("getting patterns: %w", err)
	}
	errs := preditionResponses.BustimeResponse.Error
	if len(errs) > 0 {
		lambda := func(e BusTrackerPredictionsError) string {
			return e.String()
		}
		return nil, errors.New(strings.Join(transform(errs, lambda), "; "))
	}
	return &preditionResponses.BustimeResponse.Prediction, nil
}

func (c *BusClient) GetPredictionsByVehicleIdsWithLimit(ctx context.Context, vehicleIds []string, limit int) (*[]Prediction, error) {
	if len(vehicleIds) == 0 {
		return nil, errors.New("no vehicle IDs provided")
	}
	if len(vehicleIds) > 10 {
		return nil, errors.New("maximum of 10 vehicle IDs allowed")
	}
	query := map[string]string{
		"vid": strings.Join(vehicleIds, ","),
	}
	if limit > 0 {
		query["top"] = fmt.Sprintf("%d", limit)
	}
	return c.getPredictions(ctx, query)
}

func (c *BusClient) GetPredictionsByVehicleIds(ctx context.Context, vehicleIds []string) (*[]Prediction, error) {
	return c.GetPredictionsByVehicleIdsWithLimit(ctx, vehicleIds, NO_PREDICTION_LIMIT)
}

func (c *BusClient) GetPredictionsByStopIdsWithLimit(ctx context.Context, stopIds []string, limit int) (*[]Prediction, error) {
	if len(stopIds) == 0 {
		return nil, errors.New("no stop IDs provided")
	}
	if len(stopIds) > 10 {
		return nil, errors.New("maximum of 10 stop IDs allowed")
	}
	query := map[string]string{
		"stpid": strings.Join(stopIds, ","),
	}
	if limit > 0 {
		query["top"] = fmt.Sprintf("%d", limit)
	}
	return c.getPredictions(ctx, query)
}

func (c *BusClient) GetPredictionsByStopIds(ctx context.Context, stopIds []string) (*[]Prediction, error) {
	return c.GetPredictionsByStopIdsWithLimit(ctx, stopIds, NO_PREDICTION_LIMIT)
}

func (c *BusClient) GetPredictionsByStopAndRouteIdsWithLimit(ctx context.Context, stopIds []string, routeIds []string, limit int) (*[]Prediction, error) {
	if len(routeIds) == 0 {
		return nil, errors.New("no route IDs provided")
	}
	if len(routeIds) > 10 {
		return nil, errors.New("maximum of 10 route IDs allowed")
	}
	if len(stopIds) == 0 {
		return nil, errors.New("no stop IDs provided")
	}
	if len(stopIds) > 10 {
		return nil, errors.New("maximum of 10 stop IDs allowed")
	}
	query := map[string]string{
		"stpid": strings.Join(stopIds, ","),
		"rt":    strings.Join(routeIds, ","),
	}
	if limit > 0 {
		query["top"] = fmt.Sprintf("%d", limit)
	}
	return c.getPredictions(ctx, query)
}

func (c *BusClient) GetPredictionsByStopAndRouteIds(ctx context.Context, stopIds []string, routeIds []string) (*[]Prediction, error) {
	return c.GetPredictionsByStopAndRouteIdsWithLimit(ctx, stopIds, routeIds, NO_PREDICTION_LIMIT)
}
