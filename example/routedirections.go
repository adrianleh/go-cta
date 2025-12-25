//go:build routedirections
// +build routedirections

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

	dirs, err := client.GetRouteDirections(context.Background(), "146")

	if err != nil {
		panic(err)
	}
	for _, d := range *dirs {
		println("Direction ID:", d.Id, "Name:", d.LocalizedName)
	}
}
