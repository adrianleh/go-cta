//go:build routes
// +build routes

package main

import (
	"context"
	"sort"

	"adrianlehmann.net/cta"
)

func main() {
	client, err := cta.NewBusClientFromEnv()
	if err != nil {
		panic(err)
	}

	routestransform, err := client.GetRoutetransform(context.Background())
	if err != nil {
		panic(err)
	}

	// collect keys
	keys := make([]string, 0, len(routestransform))
	for k := range routestransform {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	colors := make(map[string]struct{})

	// iterate in sorted order
	for _, id := range keys {
		r := routestransform[id]
		println("Route ID:", id, "Route Name:", r.RouteName, "Route Color:", r.RouteColor)
		colors[r.RouteColor] = struct{}{}
	}

	println("Unique colors used:")
	for color := range colors {
		println(color)
	}

	if len(keys) == 0 {
		panic("no routes found - this should not happen")
	}

	lastRoute := routestransform[keys[len(keys)-1]]
	vehicles, err := lastRoute.GetVehicles(client, context.Background())
	if err != nil {
		panic(err)
	}
	println("Found ", len(*vehicles), " vehicles for route ", lastRoute.RouteId)
	for _, v := range *vehicles {
		println(" Vehicle ID:", v.Id, "Heading:", v.Heading, "Lat:", v.Latitude, "Lon:", v.Longitude)
	}
}
