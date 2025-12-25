//go:build spanish
// +build spanish

package main

import (
	"context"
	"fmt"

	"adrianlehmann.net/cta"
)

func main() {
	client, err := cta.NewBusClientFromEnv()
	if err != nil {
		panic(err)
	}

	client.SpanishLocale()

	stateAndWashington := "1425"
	stopIDs := []string{stateAndWashington}

	preds, err := client.GetPredictionsByStopIds(context.Background(), stopIDs)
	if err != nil {
		panic(err)
	}

	for _, p := range *preds {
		fmt.Printf("Vehicle: %s  Route: %s  StopName: %s\n", p.VehicleId, p.Route, p.StopName)

		engName := p.GetDynamicIndentifierName()
		esName := p.GetDynamicIndentifierNameES()
		localeName := p.GetDynamicIndentifierNameByClientLocale(client)

		engDesc := p.GetDynamicIndentifierDescription()
		esDesc := p.GetDynamicIndentifierDescriptionES()
		localeDesc := p.GetDynamicIndentifierDescriptionByClientLocale(client)

		fmt.Printf(" Dynamic (EN): %s - %s\n", engName, engDesc)
		fmt.Printf(" Dynamic (ES): %s - %s\n", esName, esDesc)
		fmt.Printf(" Dynamic (ByClientLocale): %s - %s\n", localeName, localeDesc)

		if p.IsMinutesAwayJustDelay() {
			fmt.Println(" MinutesAway: DLY (so delayed there's no prediction)")
			continue
		}
		mins, err := p.GetMinutesAwayAsInt()
		if err != nil {
			fmt.Printf(" MinutesAway: unknown (%v)\n", err)
			continue
		}
		fmt.Printf(" MinutesAway: %d\n", mins)
	}

	routes, err := client.GetRoutes(context.Background())
	if err != nil {
		panic(err)
	}

	fmt.Println("\nAvailable routes (names in Spanish - at least maybe some of them - translations aren't the best):")
	for _, r := range *routes {
		routeNameES := r.RouteName
		fmt.Printf(" Route ID: %s  Name (ES): %s\n", r.RouteId, routeNameES)
	}
}
