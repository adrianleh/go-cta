package cta

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"strings"
)

type busTrackerRouteResponse struct {
	BustimeResponse struct {
		Error []string `json:"error,omitempty"`
		Route []Route  `json:"routes"`
	} `json:"bustime-response"`
}

type Route struct {
	RouteId          string `json:"rt"`
	RouteName        string `json:"rtnm"`
	RouteColor       string `json:"rtclr"`
	RouteDesignation string `json:"rtdd"`
}

func (r *Route) GetColor() (color.RGBA, error) {
	return hexToRGBA(r.RouteColor)
}

func (r *Route) GetVehicles(c *BusClient, ctx context.Context) (*[]Vehicle, error) {
	return c.GetVehiclesByRoute(ctx, []string{r.RouteId})
}

func (c *BusClient) GetRoutes(ctx context.Context) (*[]Route, error) {
	routesResp, err := processRequest[busTrackerRouteResponse](c.c, ctx, "getroutes", map[string]string{})
	if err != nil {
		return nil, fmt.Errorf("getting routes: %w", err)
	}
	if len(routesResp.BustimeResponse.Error) > 0 {
		return nil, errors.New(strings.Join(routesResp.BustimeResponse.Error, "; "))
	}
	return &routesResp.BustimeResponse.Route, nil
}

func (c *BusClient) GetRoutetransform(ctx context.Context) (map[string]*Route, error) {
	routes, err := c.GetRoutes(ctx)
	if err != nil {
		return nil, err
	}
	routetransform := make(map[string]*Route)
	for i, route := range *routes {
		routetransform[route.RouteId] = &(*routes)[i]
	}
	return routetransform, nil
}

func (r *Route) GetDirections(c *BusClient, ctx context.Context) (*[]RouteDirection, error) {
	return c.GetRouteDirections(ctx, r.RouteId)
}

func (r *Route) GetStopsForRawDirection(c *BusClient, ctx context.Context, dir string) (*[]Stop, error) {
	return c.GetStopsByRouteDirection(ctx, r.RouteId, dir)
}

func (r *Route) GetStopsForDirection(c *BusClient, ctx context.Context, dir RouteDirection) (*[]Stop, error) {
	return c.GetStopsByRouteDirection(ctx, r.RouteId, dir.Id)
}
