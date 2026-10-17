// Logs CTA bus positions and measures bunching from the log.
//
//	bunchinglog log      poll Bus Tracker and append vehicle positions to a JSONL file
//	bunchinglog analyze  compute headways, rider waits and bunching from that file
//	bunchinglog synth    write a synthetic dataset (clearly flagged) to try the analyzer without an API key
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"adrianlehmann.net/cta"
	"adrianlehmann.net/cta/example/bunchinglib"
)

const (
	DEFAULT_OBSERVATIONS = "bunching-observations.jsonl"
	DEFAULT_PATTERNS     = "bunching-patterns.json"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: bunchinglog <log|analyze|synth> [flags]\nRun 'bunchinglog <command> -h' for the flags of a command.\n")
	os.Exit(2)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "log":
		err = logCmd(os.Args[2:])
	case "analyze":
		err = analyzeCmd(os.Args[2:])
	case "synth":
		err = synthCmd(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		slog.Error("bunchinglog failed", slog.Any("err", err))
		os.Exit(1)
	}
}

func logCmd(args []string) error {
	fs := flag.NewFlagSet("log", flag.ExitOnError)
	routes := fs.String("routes", "6", "comma-separated bus routes to log")
	interval := fs.Duration("interval", 60*time.Second, "polling interval")
	out := fs.String("out", DEFAULT_OBSERVATIONS, "observations file (appended to)")
	patterns := fs.String("patterns", DEFAULT_PATTERNS, "pattern cache file")
	_ = fs.Parse(args)

	var routeList []string
	for _, r := range strings.Split(*routes, ",") {
		if r = strings.TrimSpace(r); r != "" {
			routeList = append(routeList, r)
		}
	}
	if len(routeList) == 0 {
		return fmt.Errorf("no routes given")
	}
	if *interval < 10*time.Second {
		return fmt.Errorf("interval %s is too short, the API has a daily request limit", *interval)
	}

	client, err := cta.NewBusClientFromEnv()
	if err != nil {
		return err
	}
	client = client.WithHTTPClient(&http.Client{Timeout: REQUEST_TIMEOUT})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runLogger(ctx, client, loggerConfig{
		routes:       routeList,
		interval:     *interval,
		outPath:      *out,
		patternsPath: *patterns,
	})
}

func analyzeCmd(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	cfg := analyzerConfig{}
	fs.StringVar(&cfg.inPath, "in", DEFAULT_OBSERVATIONS, "observations file")
	fs.StringVar(&cfg.patternsPath, "patterns", DEFAULT_PATTERNS, "pattern cache file written by the logger")
	fs.StringVar(&cfg.reportPath, "report", "bunching-report.md", "markdown report to write (empty to skip)")
	fs.StringVar(&cfg.chartPath, "chart", "bunching-headways.svg", "SVG headway chart to write (empty to skip)")
	fs.IntVar(&cfg.stops, "stops", 5, "number of reference stops per direction")
	fs.Float64Var(&cfg.bunchMinutes, "bunch-minutes", bunchinglib.BunchMinutes, "a headway under this many minutes counts as bunched")
	fs.Float64Var(&cfg.bunchFraction, "bunch-fraction", bunchinglib.BunchFraction, "relative bunching: headway under this fraction of the median")
	fs.IntVar(&cfg.minHeadways, "min-headways", 30, "minimum headways for a group to make the quick read")
	fs.DurationVar(&cfg.maxGap, "max-gap", 5*time.Minute, "drop headways spanning a data gap longer than this")
	fs.DurationVar(&cfg.maxInterp, "max-interp", 10*time.Minute, "longest gap between two reports of a bus to interpolate across")
	_ = fs.Parse(args)

	obs, err := readObservations(cfg.inPath)
	if err != nil {
		return err
	}
	patterns, err := loadPatterns(cfg.patternsPath)
	if err != nil {
		return err
	}
	a, err := analyze(obs, patterns, cfg)
	if err != nil {
		return err
	}

	fmt.Print(a.summaryText(cfg))

	if cfg.chartPath != "" {
		if err := os.WriteFile(cfg.chartPath, []byte(headwayChartSVG(a, cfg)), 0o644); err != nil {
			return fmt.Errorf("writing chart: %w", err)
		}
	}
	if cfg.reportPath != "" {
		if err := os.WriteFile(cfg.reportPath, []byte(a.markdown(cfg, cfg.chartPath)), 0o644); err != nil {
			return fmt.Errorf("writing report: %w", err)
		}
		fmt.Printf("\nWrote %s\n", cfg.reportPath)
	}
	return nil
}

func synthCmd(args []string) error {
	fs := flag.NewFlagSet("synth", flag.ExitOnError)
	cfg := synthConfig{}
	fs.StringVar(&cfg.outPath, "out", "synthetic-observations.jsonl", "observations file to write")
	fs.StringVar(&cfg.patternsPath, "patterns", "synthetic-patterns.json", "pattern file to write")
	fs.IntVar(&cfg.days, "days", 7, "days to simulate")
	fs.Uint64Var(&cfg.seed, "seed", 6, "random seed")
	_ = fs.Parse(args)
	return runSynth(cfg)
}
