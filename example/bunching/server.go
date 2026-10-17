package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"
)

//go:embed page.html
var pageHTML string

var page = template.Must(template.New("page").Parse(pageHTML))

//go:embed map.js
var mapJS string

//go:embed overview_map.js
var overviewMapJS string

type pageData struct {
	Title     string
	Synthetic bool
	Live      []routeView // route page
	Map       bool        // show the map instead of the strips
	Overview  *overviewView
	Method    template.HTML
}

type routeView struct {
	Route      string
	Title      string
	Updated    string
	Error      string
	Unplaced   int
	Directions []directionView
}

type directionView struct {
	Direction string
	InService int
	Convoys   int
	Bunched   string // e.g. "4 bunched into 2 clusters"
	HasWait   bool
	WaitNow   string
	WaitEven  string
	WaitWorst string
	Strip     template.HTML
}

type overviewView struct {
	Loading    bool // no snapshot yet
	Refreshing bool
	Done       int // routes fetched so far while refreshing
	Total      int
	Sort       string
	Rows       []overviewRow
}

type overviewRow struct {
	Route    string
	Name     string
	Buses    int
	Bunched  int
	Wait     string
	WaitEven string
	Worst    string
	Factor   string
}

func newServer(source *liveSource, overviewEvery time.Duration) *http.ServeMux {
	mux := http.NewServeMux()
	ov := newOverviewSource(source, overviewEvery)
	overviewPage := func(w http.ResponseWriter, r *http.Request) {
		render(w, pageData{
			Title:     "CTA bunching",
			Synthetic: source.client == nil,
			Overview:  newOverviewView(ov.get(), r.URL.Query().Get("sort")),
		})
	}
	mux.HandleFunc("GET /{$}", overviewPage)
	mux.HandleFunc("GET /overview", overviewPage)
	mux.HandleFunc("GET /overview.json", func(w http.ResponseWriter, r *http.Request) {
		st := ov.get()
		convoys := []networkConvoy{}
		if st.Snap != nil {
			convoys = st.Snap.Convoys
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(map[string]any{
			"convoys": convoys, "loading": st.Snap == nil, "refreshing": st.Refreshing, "done": st.Done, "total": st.Total,
		})
		if err != nil {
			slog.Error("Encoding overview data", slog.Any("err", err))
		}
	})
	mux.HandleFunc("GET /overview-map.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		_, _ = w.Write([]byte(overviewMapJS))
	})
	mux.HandleFunc("GET /route/{routes}", func(w http.ResponseWriter, r *http.Request) {
		routes := parseRouteList(r.PathValue("routes"))
		data := pageData{Synthetic: source.client == nil, Method: METHOD_HTML}
		for _, rt := range routes {
			live, err := source.getLive(r.Context(), rt)
			data.Live = append(data.Live, newRouteView(rt, live, err))
		}
		data.Title = "Bunching on #" + strings.Join(routes, ", #")
		render(w, data)
	})
	mux.HandleFunc("GET /route/{route}/map", func(w http.ResponseWriter, r *http.Request) {
		rt := r.PathValue("route")
		live, err := source.getLive(r.Context(), rt)
		render(w, pageData{
			Title:     "Bunching on #" + rt + ", map",
			Synthetic: source.client == nil,
			Method:    METHOD_HTML,
			Map:       true,
			Live:      []routeView{newRouteView(rt, live, err)},
		})
	})
	mux.HandleFunc("GET /route/{route}/map.json", func(w http.ResponseWriter, r *http.Request) {
		live, _ := source.getLive(r.Context(), r.PathValue("route"))
		if live == nil {
			http.Error(w, "Couldn't reach Bus Tracker", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mapData(live)); err != nil {
			slog.Error("Encoding map data", slog.Any("err", err))
		}
	})
	mux.HandleFunc("GET /map.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		_, _ = w.Write([]byte(mapJS))
	})
	return mux
}

func render(w http.ResponseWriter, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.Execute(w, data); err != nil {
		slog.Error("Rendering page", slog.Any("err", err))
	}
}

func newRouteView(rt string, live *liveRoute, err error) routeView {
	v := routeView{Route: rt, Title: "#" + rt}
	if err != nil {
		// Details go to the server log only - visitors don't need them
		v.Error = "Couldn't reach Bus Tracker just now."
		if live != nil {
			v.Error += " Showing the last data we got."
		}
	}
	if live == nil {
		return v
	}
	v.Title = routeTitle(live)
	v.Updated = live.FetchedAt.In(chicago).Format("3:04:05 PM")
	v.Unplaced = live.Unplaced
	for _, d := range live.Directions {
		v.Directions = append(v.Directions, directionView{
			Direction: d.Direction,
			InService: d.InService,
			Convoys:   len(d.Convoys),
			Bunched:   bunchedSummary(d),
			HasWait:   !math.IsNaN(d.ExpectedWait),
			WaitNow:   formatMinutes(d.ExpectedWait),
			WaitEven:  formatMinutes(d.EvenSpacingWait),
			WaitWorst: formatGap(d.WorstGap),
			Strip:     template.HTML(stripSVG(d)), // built from escaped strings in svg.go
		})
	}
	return v
}

func newOverviewView(st overviewStatus, sortBy string) *overviewView {
	if sortBy == "" {
		sortBy = DEFAULT_OVERVIEW_SORT
	}
	snap := st.Snap
	v := &overviewView{Loading: snap == nil, Refreshing: st.Refreshing, Done: st.Done, Total: st.Total, Sort: sortBy}
	if snap == nil {
		return v
	}
	rows := slices.Clone(snap.Routes)
	sortSummaries(rows, sortBy)
	for _, s := range rows {
		row := overviewRow{Route: s.Route, Name: s.Name, Buses: s.Buses, Bunched: s.BunchedBuses, Wait: "-", WaitEven: "-", Worst: "-", Factor: "-"}
		if !math.IsNaN(s.Wait) {
			row.Wait, row.WaitEven = formatMinutes(s.Wait), formatMinutes(s.WaitEven)
		}
		if f := s.worstFactor(); !math.IsNaN(f) {
			row.Factor = fmt.Sprintf("%.1f×", f)
		}
		if !math.IsNaN(s.Worst) {
			row.Worst = formatMinutes(s.Worst)
		}
		v.Rows = append(v.Rows, row)
	}
	return v
}

const METHOD_HTML = template.HTML(`<p>Buses are ordered by distance along their direction's route pattern.
Buses within 1000 ft of either end are treated as laying over (hollow dots) and left out.</p>
<p><strong>Gap</strong> between a bus and the one behind it: from Bus Tracker predictions, the time the bus behind is predicted at
the first upcoming stop both have a prediction for, minus the time the bus ahead is predicted there.
If they share no predicted stop (big gaps can be longer than the prediction horizon), the gap is <strong>estimated</strong>
from the distance and the average speed the buses report (shown dashed and marked ~).</p>
<p><strong>Bunched</strong>: a gap under 2 minutes, or under 25% of the median gap, whichever is larger.
Buses in a row with bunched gaps between them form one <strong>bunched cluster</strong>: three buses in a row are one cluster, not two pairs.</p>
<p><strong>Worst case</strong>: the longest gap right now - how long you wait if you just missed the bus at the start of it.</p>
<p><strong>Expected wait if you arrive now</strong>: E[H²] / (2·E[H]) over the current gaps H. You're more likely to show up during a long gap,
so uneven spacing makes the average wait longer. <strong>If evenly spaced</strong>: E[H] / 2, the same buses spread out evenly.</p>`)
