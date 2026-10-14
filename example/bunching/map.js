// The map view: each direction's pattern, buses at their GPS positions, bunched pairs joined by a red line.
// Redraws from /route/<route>/map.json every 30 seconds.
const mapEl = document.getElementById("map");
const style = getComputedStyle(document.documentElement);
const colors = {
  bus: style.getPropertyValue("--gap").trim(),
  bunched: style.getPropertyValue("--bunched").trim(),
  muted: style.getPropertyValue("--muted").trim(),
};

const map = L.map(mapEl).setView([41.88, -87.63], 12); // downtown until the route loads
L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
  maxZoom: 19,
  attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
}).addTo(map);
const layer = L.layerGroup().addTo(map);
let fitted = false;
let openVid = null; // keep a tapped bus's popup open across refreshes
map.on("popupclose", () => (openVid = null));

function popupText(direction, bus) {
  const lines = [`<b>Bus ${bus.vid}</b>`, direction];
  if (bus.terminal) lines.push("At a terminal");
  if (bus.gapAhead) lines.push(`${bus.gapAhead} min behind the bus ahead${bus.bunched ? " (bunched)" : ""}`);
  if (bus.gapBehind) lines.push(`${bus.gapBehind} min ahead of the bus behind`);
  if ((bus.gapAhead + bus.gapBehind).includes("~")) lines.push("~ estimated from distance");
  return lines.join("<br>");
}

function draw(directions) {
  const keep = openVid;
  layer.clearLayers();
  openVid = keep;
  const bounds = [];
  for (const d of directions) {
    if (d.path) {
      L.polyline(d.path, { color: colors.muted, weight: 4, opacity: 0.5 }).addTo(layer);
      bounds.push(...d.path);
    }
    for (const pair of d.bunchedPairs) {
      L.polyline(pair, { color: colors.bunched, weight: 8, opacity: 0.8 }).addTo(layer);
    }
    for (const bus of d.buses) {
      const color = bus.bunched ? colors.bunched : bus.terminal ? colors.muted : colors.bus;
      const marker = L.circleMarker(bus.pos, {
        radius: 8,
        color: "#fff",
        weight: 2,
        fillColor: color,
        fillOpacity: bus.terminal ? 0.3 : 1,
      }).bindPopup(popupText(d.direction, bus)).addTo(layer);
      marker.on("click", () => (openVid = bus.vid));
      if (bus.vid === openVid) marker.openPopup();
    }
  }
  // Only zoom to the route on the first draw, so a refresh doesn't undo the rider's panning
  if (!fitted && bounds.length) {
    map.fitBounds(bounds, { padding: [16, 16] });
    fitted = true;
  }
}

async function refresh() {
  try {
    const res = await fetch(`/route/${encodeURIComponent(mapEl.dataset.route)}/map.json`);
    if (res.ok) draw(await res.json());
  } catch (e) {
    // Offline for a moment - keep showing what we have
  }
}

refresh();
setInterval(refresh, 30000);
