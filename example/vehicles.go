//go:build vehicles
// +build vehicles

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"adrianlehmann.net/cta"
)

func main() {
	client, err := cta.NewBusClientFromEnv()
	if err != nil {
		panic(err)
	}

	routes := []string{"146", "36", "J14"}
	vehicles, err := client.GetVehiclesByRoute(context.Background(), routes)
	if err != nil {
		panic(err)
	}

	for _, vehicle := range *vehicles {
		b, err := json.Marshal(vehicle)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(b))
		time, err := vehicle.GetScheduleStartTimeUnix()
		if err != nil {
			panic(err)
		}
		println(time)
		pos, err := vehicle.GetPosition()
		if err != nil {
			panic(err)
		}
		println("Lat:", pos.Latitude, "Lon:", pos.Longitude)
	}
	println("Updating last vehicle")
	lastIdx := len(*vehicles) - 1
	firstVehicleUpdated, err := (*vehicles)[lastIdx].Update(client, context.Background())
	if err != nil {
		panic(err)
	}
	b, err := json.Marshal(firstVehicleUpdated)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
}
