package cta

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type busTrackerRouteDirectionResponse struct {
	BustimeResponse struct {
		Error []string         `json:"error,omitempty"`
		Route []RouteDirection `json:"directions"`
	} `json:"bustime-response"`
}

type RouteDirection struct {
	Id            string `json:"id"`
	LocalizedName string `json:"name"`
}

func (c *BusClient) GetRouteDirections(ctx context.Context, routeId string) (*[]RouteDirection, error) {
	query := map[string]string{
		"rt": routeId,
	}
	dirResp, err := processRequest[busTrackerRouteDirectionResponse](c.c, ctx, "getdirections", query)
	if err != nil {
		return nil, fmt.Errorf("getting route directions: %w", err)
	}
	if len(dirResp.BustimeResponse.Error) > 0 {
		return nil, errors.New(strings.Join(dirResp.BustimeResponse.Error, "; "))
	}
	return &dirResp.BustimeResponse.Route, nil
}
