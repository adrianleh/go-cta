// A small website showing CTA bus bunching live: pick a route, see each direction as a strip with every bus on it,
// bunched pairs highlighted, and the wait a rider arriving now can expect versus evenly spaced buses.
//
//	go run ./example/bunching                     serve on :8080 (needs CTA_BUS_API_KEY)
//	go run ./example/bunching -synthetic          serve made-up buses, no key needed
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"adrianlehmann.net/cta"
)

func main() {
	addr := flag.String("addr", ":8080", "address to serve on")
	synthetic := flag.Bool("synthetic", false, "use simulated buses instead of Bus Tracker")
	overviewEvery := flag.Duration("overview-every", OVERVIEW_REFRESH_EVERY, "how often the all-routes page refetches every route while it's being viewed (Bus Tracker has a daily request limit)")
	flag.Parse()

	var client *cta.BusClient
	if !*synthetic {
		var err error
		client, err = cta.NewBusClientFromEnv()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error()+" - set it, or run with -synthetic to try it with simulated buses")
			os.Exit(1)
		}
	}
	source := newLiveSource(client)

	slog.Info("Serving", slog.String("addr", *addr), slog.Bool("synthetic", *synthetic))
	if err := http.ListenAndServe(*addr, newServer(source, *overviewEvery)); err != nil {
		slog.Error("Serving", slog.Any("err", err))
		os.Exit(1)
	}
}
