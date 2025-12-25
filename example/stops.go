//go:build stops
// +build stops

package main

import (
	"context"

	"adrianlehmann.net/cta"
)

func main() {
	client, err := cta.NewBusClientFromEnv()
	if err != nil {
		panic(err)
	}

	stops, err := client.GetStopsByRouteDirection(context.Background(), "J14", "Northbound")
	if err != nil {
		panic(err)
	}

	println("Stops for J14 Northbound:")
	for _, stop := range *stops {
		println("Stop ID:", stop.Id, "Name:", stop.Name, "Lat:", stop.Latitude, "Lon:", stop.Longitude)
	}
}
