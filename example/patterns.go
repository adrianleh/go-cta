//go:build patterns
// +build patterns

package main

import (
	"context"

	"adrianlehmann.net/cta"
)

const (
	I_UNDERSTAND_THIS_FUNCTION_MAKES_MANY_API_CALLS = true
)

func main() {
	client, err := cta.NewBusClientFromEnv()
	if err != nil {
		panic(err)
	}

	patterns, err := client.GetPatternsByRoute(context.Background(), "J14")
	if err != nil {
		panic(err)
	}

	println("Patterns for J14 Northbound:")
	for _, pattern := range *patterns {
		println("Pattern ID:", pattern.Pid, "Length in freedom feet:", pattern.LengthFeet)
		for _, point := range pattern.Points {
			print("  Point Lat:", point.Latitude, "Lon:", point.Longitude)
			if point.IsStop() {
				print(" Stop ID:", point.StopId)
			}
			println()
		}
	}

	if I_UNDERSTAND_THIS_FUNCTION_MAKES_MANY_API_CALLS {
		println("With stops:")
		stops, err := (*patterns)[0].GetStops(client, context.Background())
		if err != nil {
			panic(err)
		}
		for _, stop := range *stops {
			println("Stop ID:", stop.Id, "Name:", stop.Name)
		}
	}
}
