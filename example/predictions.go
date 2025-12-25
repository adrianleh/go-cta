//go:build predictions
// +build predictions

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

	StateAndWashingtonStopID := "1425"

	stopIDs := []string{StateAndWashingtonStopID} // example stop id; change as needed

	preds, err := client.GetPredictionsByStopIds(context.Background(), stopIDs)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Predictions for stop(s) %v:\n", stopIDs)
	for _, p := range *preds {
		if p.IsMinutesAwayJustDelay() {
			fmt.Printf("Vehicle: %s  Route: %s  StopName: %s  MinutesAway: SO DELAYED CTA DOESN'T EVEN KNOW. GOOD LUCK!  Type: %s\n",
				p.VehicleId, p.Route, p.StopName, p.Type)
			continue
		}
		mins, err := p.GetMinutesAwayAsInt()
		if err != nil {
			fmt.Printf("Vehicle: %s  Route: %s  StopName: %s  MinutesAway: Unknown: %v  Type: %s\n",
				p.VehicleId, p.Route, p.StopName, err, p.Type)
			continue
		}
		fmt.Printf("Vehicle: %s  Route: %s  StopName: %s  MinutesAway: %d  Type: %s\n",
			p.VehicleId, p.Route, p.StopName, mins, p.Type)
	}
}
