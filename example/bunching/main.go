// A small website showing CTA bus bunching live: pick a route, see each direction as a strip with every bus on it,
// bunched pairs highlighted, and the wait a rider arriving now can expect versus evenly spaced buses.
//
//	go run ./example/bunching                     serve on :8080 (needs CTA_BUS_API_KEY)
//	go run ./example/bunching -synthetic          serve made-up buses, no key needed
//	go run ./example/bunching -snapshot 6         write snapshot-6.svg and snapshot-6.md and exit
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"adrianlehmann.net/cta"
)

func main() {
	addr := flag.String("addr", ":8080", "address to serve on")
	synthetic := flag.Bool("synthetic", false, "use simulated buses instead of Bus Tracker")
	snapshot := flag.String("snapshot", "", "write a snapshot of this route and exit instead of serving")
	outDir := flag.String("out", ".", "directory for -snapshot files")
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

	if *snapshot != "" {
		if err := writeSnapshot(source, *snapshot, *outDir); err != nil {
			slog.Error("Writing snapshot", slog.Any("err", err))
			os.Exit(1)
		}
		return
	}

	slog.Info("Serving", slog.String("addr", *addr), slog.Bool("synthetic", *synthetic))
	if err := http.ListenAndServe(*addr, newServer(source)); err != nil {
		slog.Error("Serving", slog.Any("err", err))
		os.Exit(1)
	}
}

func writeSnapshot(source *liveSource, rt string, outDir string) error {
	live, err := source.getLive(context.Background(), rt)
	if err != nil {
		return err
	}
	svgName := "snapshot-" + rt + ".svg"
	if err := os.WriteFile(filepath.Join(outDir, svgName), []byte(snapshotSVG(live)), 0o644); err != nil {
		return err
	}
	mdPath := filepath.Join(outDir, "snapshot-"+rt+".md")
	if err := os.WriteFile(mdPath, []byte(snapshotMarkdown(live, svgName)), 0o644); err != nil {
		return err
	}
	fmt.Println(snapshotCaption(live))
	for _, d := range live.Directions {
		fmt.Printf("%s: %s\n", d.Direction, headline(d))
	}
	fmt.Println("Wrote", filepath.Join(outDir, svgName), "and", mdPath)
	return nil
}
