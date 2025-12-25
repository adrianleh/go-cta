//go:build detours
// +build detours

package main

import (
	"context"
	"fmt"
	"log/slog"

	"adrianlehmann.net/cta"
)

func main() {
	client, err := cta.NewBusClientFromEnv()
	if err != nil {
		panic(err)
	}
	client.WithUnredactedLogger(slog.Default())

	// Example: list all active detours
	detours, err := client.GetAllDetours(context.Background())
	if err != nil {
		panic(err)
	}

	fmt.Printf("Found %d detours:\n", len(*detours))
	for _, d := range *detours {
		fmt.Printf("ID: %s  Version: %d  Status: %d\n", d.ID, d.Version, d.Status)
		fmt.Printf("Description: %s\n", d.Description)
		fmt.Printf("Start: %s  End: %s\n", d.StartDateTime, d.EndDateTime)
		if len(d.RouteDirections) > 0 {
			fmt.Printf("Affected routes/directions:\n")
			for _, rd := range d.RouteDirections {
				fmt.Printf("  Route: %s  Direction: %s\n", rd.Route, rd.Direction)
			}
		}
		fmt.Println()
	}

	// Example: get detours for a specific route
	routeID := "22"
	routeDetours, err := client.GetDetoursByRoute(context.Background(), routeID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Detours for route %s: %d\n", routeID, len(*routeDetours))

	// Example: get detours for a route + direction
	direction := "Northbound"
	rdDetours, err := client.GetDetoursByRouteDirection(context.Background(), routeID, direction)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Detours for route %s %s: %d\n", routeID, direction, len(*rdDetours))
}
