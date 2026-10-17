// The network map: every convoy (buses bunched with the one ahead) on every route: a red line through its buses and the route number on it.
// Click a pair to open its route. Polls /overview.json: every 2 seconds while the server is fetching routes (to show
// progress), every 30 seconds otherwise (the server only refetches Bus Tracker when its snapshot is stale).
const mapEl = document.getElementById("map");
const wrapEl = document.getElementById("map-wrap");
const statusEl = document.getElementById("status");
const statusText = document.getElementById("status-text");
const bunched = getComputedStyle(document.documentElement).getPropertyValue("--bunched").trim();

const map = L.map(mapEl).setView([41.88, -87.63], 11);
L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
  maxZoom: 19,
  attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
}).addTo(map);
const layer = L.layerGroup().addTo(map);
let fitted = false;
let drawn = ""; // the convoys on the map, so an unchanged poll doesn't close a popup the rider has open
let busy = true; // the server is fetching routes, as far as we know

function escapeHtml(s) {
  return s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
}

function labelIcon(c) {
  const size = c.buses > 2 ? ` ×${c.buses}` : "";
  return L.divIcon({
    html: `<span class="route-pill">${escapeHtml(c.route)}${size}</span>`,
    className: "route-label",
    iconSize: null, // sized by the pill, anchored at its center by the CSS
  });
}

function popupHtml(c) {
  const rt = encodeURIComponent(c.route);
  const what = c.buses > 2 ? `${c.buses} buses in a row` : "2 buses bunched";
  return (
    `<b>#${escapeHtml(c.route)} ${escapeHtml(c.name)}</b><br>${escapeHtml(c.direction)}<br>${what}, gaps ${escapeHtml(c.gaps)} min<br>` +
    `<a href="/route/${rt}/map"><b>Open this route's map →</b></a><br><a href="/route/${rt}">Strip view</a>`
  );
}

// The pill sits on the middle bus of the convoy (between the two for an even count)
function middle(pos) {
  const m = (pos.length - 1) / 2;
  const [a, b] = [pos[Math.floor(m)], pos[Math.ceil(m)]];
  return [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2];
}

function draw(convoys) {
  layer.clearLayers();
  const points = [];
  for (const c of convoys) {
    const popup = popupHtml(c);
    L.polyline(c.pos, { color: bunched, weight: 8, opacity: 0.8 }).bindPopup(popup).addTo(layer);
    L.marker(middle(c.pos), { icon: labelIcon(c) }).bindPopup(popup).addTo(layer);
    points.push(...c.pos);
  }
  // Only zoom to the convoys on the first draw, so a refresh doesn't undo the rider's panning
  if (!fitted && points.length) {
    map.fitBounds(points, { padding: [24, 24] });
    fitted = true;
  }
}

function showStatus(data) {
  statusEl.hidden = !(data.loading || data.refreshing);
  statusEl.className = `status ${data.loading ? "big" : "small"}`;
  wrapEl.classList.toggle("loading", data.loading);
  statusText.textContent = data.total ? `Loaded ${data.done}/${data.total} routes` : "";
}

// Swap in the server-rendered table, which the server filled in while we weren't looking
async function swapTable() {
  const res = await fetch(location.href);
  const fresh = new DOMParser().parseFromString(await res.text(), "text/html").getElementById("live");
  if (fresh) {
    document.getElementById("live").replaceWith(fresh);
    applyFilter(); // keep the table filtered by whatever is typed in the search box
  }
}

async function refresh() {
  let delay = 30000;
  try {
    const res = await fetch("/overview.json");
    if (res.ok) {
      const data = await res.json();
      const convoys = JSON.stringify(data.convoys);
      if (convoys !== drawn) {
        draw(data.convoys);
        drawn = convoys;
      }
      showStatus(data);
      const nowBusy = data.loading || data.refreshing;
      if (busy && !nowBusy) await swapTable();
      busy = nowBusy;
      if (nowBusy) delay = 2000;
    }
  } catch (e) {
    // Offline for a moment - keep showing what we have
  }
  setTimeout(refresh, delay);
}

refresh();
