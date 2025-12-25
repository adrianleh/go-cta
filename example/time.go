//go:build time
// +build time

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

	unixTime, err := client.GetUnixTime(context.Background())

	if err != nil {
		panic(err)
	}

	friendlyTime, err := client.GetTime(context.Background())
	if err != nil {
		panic(err)
	}

	println("CTA API Time:", friendlyTime.String())
	println("CTA API Unix Time:", unixTime)
}
