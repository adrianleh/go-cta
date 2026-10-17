# Bus bunching, live

A small website built on go-cta. Pick any CTA bus route and see each direction as a strip (or on a map) with every bus
on it, the gaps between them, bunched clusters highlighted, and what that spacing does to your wait:

> N buses, X bunched into Y clusters. Average wait ≈ Y min (Z if even), longest W.

One Go server (standard library only), one HTML file plus a small `map.js`, no build step. Works on a phone.

## Running it

The key comes from `CTA_BUS_API_KEY`, same as the other examples.

```sh
export CTA_BUS_API_KEY=...
go run ./example/bunching                 # http://localhost:8080
go run ./example/bunching -addr :9000     # another port
go run ./example/bunching -synthetic      # simulated buses, no key needed
```

- `/` is the all-routes overview, see below. It has a search box that filters the table by route number or name.
- `/route/6` is the live view for #6, refreshed every 30 seconds. `/route/6,J14` shows several routes on one page (up to 4).
- `/route/6/map` is the same route on a map (Leaflet and OpenStreetMap tiles, loaded from a CDN). Tap a bus for its gaps.
- `/overview` lists every route with a bus in service (both directions pooled): buses, buses bunched (the buses in a bunched cluster, which is 2 or more buses in a row), average wait, wait if evenly spaced and longest wait and longest ÷ average, sortable with `?sort=route|buses|bunched|wait|even|worst|factor`. A map above it shows every convoy as one line through its buses, labeled with its route number (and ×N from 3 buses up). It's a snapshot that refetches all routes (a couple hundred Bus Tracker requests, about a minute) only while someone has the page open and it's older than `-overview-every` (default 10m), so mind your daily request limit.

Bus Tracker is only called for routes someone is viewing. Vehicle positions and predictions are cached for 30 seconds,
route patterns for 6 hours and the route list for a day, so many viewers cost the same as one. A route that's open
somewhere costs 1 vehicle request plus 1 prediction request per 10 buses every 30 seconds - about 11,500 requests a day
for a route with 30 buses, so keep an eye on the daily limit if several routes stay open all day.

## How it's computed

- **Order.** Every bus is placed along its direction's longest pattern by distance along the pattern: the API's `pdist`
  for buses on that pattern, projection of the GPS position onto it for buses on other variants (short turns etc.).
  Buses within 1000 ft of either end are treated as laying over (hollow dots) and left out of the numbers.
- **Gap in minutes.** From Bus Tracker predictions. For a bus A and the bus B behind it, take the first of A's upcoming
  stops that B also has a prediction for (usually A's next stop). The gap is B's predicted time there minus A's.
  That accounts for traffic and slow stretches, which distance alone can't.
- **Estimated gaps.** If A and B share no predicted stop (a big gap can be longer than the prediction horizon), or the
  predictions contradict the order on the strip, the gap falls back to distance divided by one speed for the direction:
  the average of the speeds the buses report right now (10 mph if fewer than 3 report a usable speed).
  Estimated gaps are drawn dashed and marked ~.
- **Bunched.** A gap under 2 minutes, or under 25% of the median gap in that direction, whichever is larger.
  Buses in a row with bunched gaps between them form one **bunched cluster**: three buses in a row are one cluster (shown with
  one "3 bunched" label on the strip and one line through them on the map), not two pairs.
  The relative part matters on infrequent routes.
- **Expected wait for a rider arriving now.** E[H²] / (2·E[H]) over the current gaps H. A rider is more likely to show
  up during a long gap than a short one, so uneven spacing makes the average wait longer even with the same number of buses.
  **If evenly spaced** is E[H] / 2. The open-ended gaps ahead of the first bus and behind the last are not included.
- **Worst case.** The longest current gap: how long you wait if you just missed the bus at the start of it.

Limitations: predictions are only as good as Bus Tracker's, estimated gaps use a rough speed, gaps are
"time for the bus behind to get where the bus ahead is going next", not measured headways at a stop, and buses on a
detour pattern far from the main pattern get projected onto it anyway.

## Logging and analyzing over time

`example/bunchinglog` logs vehicle positions to a JSONL file and computes observed headways at a few reference stops,
per route, direction and time of day (weekday AM peak, midday, PM peak, evening, overnight, weekend).

```sh
go run ./example/bunchinglog log -routes 6,55 -interval 60s     # appends to bunching-observations.jsonl, Ctrl-C to stop
go run ./example/bunchinglog analyze                            # prints the quick read, writes bunching-report.md and bunching-headways.svg
go run ./example/bunchinglog synth                              # a synthetic week to try the analyzer with
```

The logger keeps going on API errors and rate limits (logs them and backs off up to 15 minutes) and can be restarted
any time - it only ever appends. Headways that span a stretch with no data (logger down, no service) are dropped.
The wait, bunched and cluster definitions are the same code as the site (`cta.Headways` and `example/bunchinglib`). The difference is that
headways are real passing times at a stop, interpolated between position reports, with the median taken over a route, direction and
time of day. `-bunch-minutes` and `-bunch-fraction` change the bunched rule.

Example quick read (**synthetic** - `bunchinglog synth`, not CTA data):

```
*** SYNTHETIC DATA - simulated buses, not a real finding ***

- #6 Northbound, weekday PM peak (3-7pm): average observed wait 4.9 min, versus 4.1 min if the same buses were evenly spaced; 4% of headways were bunched (under 2.1 min), in 29 clusters. (703 headways over 5 days, median headway 8.3 min.)
- #6 Southbound, weekday PM peak (3-7pm): average observed wait 4.9 min, versus 4.1 min if the same buses were evenly spaced; 5% of headways were bunched (under 2.1 min), in 35 clusters. (710 headways over 5 days, median headway 8.2 min.)
...
```

## Not done yet

- **L trains.** Train Tracker positions have no distance along a pattern, so trains would need projecting onto each
  line's station sequence. `buildLiveDirection` only needs positions along a line, so a train source can feed the same view.
- **Schedule comparison.** "If evenly spaced" compares against the same buses spread out, not against the schedule.
  Scheduled headways could come from the CTA GTFS feed.
