package cta

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// busTrackerDetoursResponse mirrors the API top-level response for detours.
type busTrackerDetoursResponse struct {
	BustimeResponse struct {
		Error  []BusTrackerDetourError `json:"error,omitempty"`
		Detour []Detour                `json:"dtr"`
	} `json:"bustime-response"`
}

type Detour struct {
	ID              string                 `json:"id"`
	Version         int                    `json:"ver"`
	Status          int                    `json:"st"`
	Description     string                 `json:"desc"`
	RouteDirections []DetourRouteDirection `json:"rtdirs"`
	StartDateTime   string                 `json:"startdt"` // You cannot force this to be a unix timestamp
	EndDateTime     string                 `json:"enddt"`
	RTPIDataFeed    string                 `json:"rtpidatafeed,omitempty"`
}

type DetourRouteDirection struct {
	Route     string `json:"rt"`
	Direction string `json:"dir"`
}

type BusTrackerDetourError struct {
	Msg            string `json:"msg"`
	Route          string `json:"rt,omitempty"`
	RouteDirection string `json:"rtdir,omitempty"`
	RTPIDataFeed   string `json:"rtpidatafeed,omitempty"`
}

func (e *BusTrackerDetourError) String() string {
	if e.Route != "" && e.RouteDirection != "" {
		return fmt.Sprintf("Route %s %s: %s", e.Route, e.RouteDirection, e.Msg)
	}
	if e.Route != "" {
		return fmt.Sprintf("Route %s: %s", e.Route, e.Msg)
	}
	if e.RouteDirection != "" {
		return fmt.Sprintf("Direction %s: %s", e.RouteDirection, e.Msg)
	}
	return e.Msg
}

func (d *Detour) GetTimeStart() (time.Time, error) {
	return parseCtaDetourTime(d.StartDateTime)
}

func (d *Detour) GetUnixTimeStart() (int64, error) {
	time, err := d.GetTimeStart()
	if err != nil {
		return 0, err
	}
	return time.Unix(), nil
}

func (d *Detour) GetTimeEnd() (time.Time, error) {
	return parseCtaDetourTime(d.EndDateTime)
}
func (d *Detour) GetUnixTimeEnd() (int64, error) {
	time, err := d.GetTimeEnd()
	if err != nil {
		return 0, err
	}
	return time.Unix(), nil
}

func (c *BusClient) getDetours(ctx context.Context, query map[string]string) (*[]Detour, error) {
	detourResp, err := processRequest[busTrackerDetoursResponse](c.c, ctx, "getdetours", query)
	if err != nil {
		return nil, fmt.Errorf("getting detours: %w", err)
	}
	errs := detourResp.BustimeResponse.Error
	if len(errs) > 0 {
		lambda := func(e BusTrackerDetourError) string {
			return e.String()
		}
		return nil, errors.New(strings.Join(transform(errs, lambda), "; "))
	}
	return &detourResp.BustimeResponse.Detour, nil
}

func (c *BusClient) GetAllDetours(ctx context.Context) (*[]Detour, error) {
	return c.getDetours(ctx, map[string]string{})
}

func (c *BusClient) GetDetoursByRouteDirection(ctx context.Context, routeId string, direction string) (*[]Detour, error) {
	query := map[string]string{
		"rt":    routeId,
		"rtdir": direction,
	}
	return c.getDetours(ctx, query)
}

func (c *BusClient) GetDetoursByRoute(ctx context.Context, routeId string) (*[]Detour, error) {
	query := map[string]string{
		"rt": routeId,
	}
	return c.getDetours(ctx, query)
}
