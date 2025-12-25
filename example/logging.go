//go:build logging
// +build logging

package main

import (
	"context"
	"log/slog"

	"adrianlehmann.net/cta"
)

func main() {
	client, err := cta.NewBusClientFromEnv()
	if err != nil {
		panic(err)
	}
	client = client.WithLogger(slog.Default())

	unixTime, err := client.GetUnixTime(context.Background())

	if err != nil {
		panic(err)
	}

	println("CTA API Unix Time:", unixTime)
}
