# Practical Guide: From Your Head to a Working Hydraulic Model

*CSV in. Scripts transform. Map and simulation out.*

---

## Philosophy

You know your pipes. You'd rather type data into a spreadsheet
than click around a GIS application. The workflow here is:

1. **Edit CSV files** describing nodes, pipes, and attributes
2. **Run scripts** that convert CSV → GeoJSON, CSV → EPANET .inp
3. **Run EPANET** offline to study the hydraulic model
4. **Serve a web map** with OpenStreetMap tiles showing the asset
   catalog — every pipe, valve, connection with its attributes
5. **Optionally** run epanet-js in the browser for interactive
   simulation

The pipeline consumes CSV files. How you produce them is your
choice:

- **Text editor or spreadsheet**: Type coordinates, diameters,
  and connections directly. Good when you already know everything.
- **QGIS**: Digitize the network on a map, fill in attribute
  tables, export as CSV. A natural workflow for operators who
  prefer pointing and clicking on a map to typing coordinates.
- **Google My Maps → KML → CSV**: Drop pins, draw lines, export
  KML, convert to CSV with ogr2ogr or a script.
- **Field GPS app**: Collect points in the field, export CSV.

All roads lead to the same three CSV files. The build pipeline
doesn't care how they were made. Any tool in the ecosystem —
ogr2ogr, QGIS, PostGIS — can also consume what we produce,
because we write standard formats (GeoJSON, GeoPackage, .inp).

This follows the duckpond pattern: source data in simple files,
factory methods produce derived outputs, everything is reproducible
from the source.

```
CSV files (you edit these)
    │
    ├── nodes.csv     (junctions, tanks, reservoirs)
    ├── pipes.csv     (pipe segments with diameter, material)
    └── valves.csv    (gate valves, PRVs, etc.)
         │
         ▼
    build script (Go, Python, or shell)
         │
         ├──► network.geojson    → web map (Leaflet + OSM)
         ├──► system.inp         → EPANET simulation
         ├──► network.gpkg       → for anyone who wants QGIS
         └──► assets.json        → asset catalog for the website
```

---

## Part 1: The CSV Files You Edit

### nodes.csv

Each row is a point in the network — a connection, tank, branch
point, valve, or well.

```csv
id,type,lat,lon,elevation_ft,demand_gpm,static_psi,account,address,notes
TANK-01,TANK,39.36152,-123.81234,1225,0,,,,Storage tank 15ft diameter
WELL-01,RESERVOIR,39.36200,-123.81300,1230,0,,,,"Well #1, 8 GPM"
TEE-A,JUNCTION,39.36100,-123.81400,1180,0,,,,Main line tee
J-01,JUNCTION,39.35950,-123.81500,1110,2,50,Comm_Ctr,"15051 Caspar Rd",Community center
J-02,JUNCTION,39.35900,-123.81550,1100,1.5,52,45100_CFRW,"45100 Caspar Frontage Rd W",Residential
J-03,JUNCTION,39.35850,-123.81600,1095,1.5,53,45200_CFRW,"45200 Caspar Frontage Rd W",Residential
```

**Fields:**

| Field          | Required | Description                                |
|----------------|----------|--------------------------------------------|
| `id`           | Yes      | Unique name. Your choice of convention.    |
| `type`         | Yes      | JUNCTION, TANK, or RESERVOIR               |
| `lat`          | Yes      | Latitude (decimal degrees, WGS84)          |
| `lon`          | Yes      | Longitude (decimal degrees, WGS84)         |
| `elevation_ft` | Yes*     | Feet above sea level. *See Part 3 for auto-fill. |
| `demand_gpm`   | No       | Base demand in GPM. 0 for tees/tanks.      |
| `static_psi`   | No       | Measured static pressure, if known.        |
| `account`      | No       | Billing account name (match users.csv).    |
| `address`      | No       | Service address.                           |
| `notes`        | No       | Anything else. Sensor metric names go here.|

**For tanks, add these columns (ignored for other types):**

| Field              | Description                            |
|--------------------|----------------------------------------|
| `tank_diameter_ft` | Tank diameter in feet                  |
| `tank_max_level_ft`| Maximum water level above base         |
| `tank_min_level_ft`| Minimum water level above base         |
| `tank_init_level_ft`| Initial water level for simulation    |

### pipes.csv

Each row is a pipe segment connecting two nodes.

```csv
id,from_node,to_node,diameter_in,material,roughness,install_year,notes
MAIN-01,TANK-01,TEE-A,6,PVC,150,1985,Main line from tank
MAIN-02,TEE-A,J-01,6,PVC,150,1985,Main to community center
SVC-01,TEE-A,J-02,1.5,poly,140,2010,Service line
SVC-02,J-01,J-03,1,galv,120,1975,Old service line
```

**Fields:**

| Field          | Required | Description                                |
|----------------|----------|--------------------------------------------|
| `id`           | Yes      | Unique pipe name.                          |
| `from_node`    | Yes      | Upstream node ID (must exist in nodes.csv).|
| `to_node`      | Yes      | Downstream node ID.                        |
| `diameter_in`  | Yes      | Internal diameter in inches.               |
| `material`     | Yes      | PVC, poly, galv, steel, copper, DI, CI.    |
| `roughness`    | No       | Hazen-Williams C. Auto-filled from material if blank. |
| `install_year` | No       | Year installed.                            |
| `notes`        | No       | Description.                               |

**Pipe length** is calculated automatically from the lat/lon
coordinates of from_node and to_node (Haversine formula). You
don't enter it. If you need a pipe that follows a curved route
(not a straight line between nodes), add intermediate junction
nodes with zero demand.

### valves.csv (optional)

```csv
id,from_node,to_node,type,diameter_in,setting,notes
PRV-01,TEE-A,J-04,PRV,6,50,Pressure reducing valve set to 50 PSI
GV-01,MAIN-01,,GATE,6,,Gate valve on main line
```

**Valve types:** PRV (pressure reducing), PSV (pressure sustaining),
FCV (flow control), TCV (throttle control), GATE (isolation — not
modeled hydraulically, just an asset record).

### Roughness defaults

If you leave `roughness` blank, the build script fills it from
`material`:

```
PVC:  150    poly: 140    galv: 120    steel: 130
copper: 135  DI:   130    CI:    100
```

---

## Part 2: What You Need to Know

To fill in nodes.csv and pipes.csv, you need:

**Easy to get:**
- Lat/lon for each point: stand there, open your phone's compass
  app or Google Maps, note the coordinates
- Pipe diameters: you know these
- Pipe materials: you know these
- Tank dimensions: you know these
- Which node connects to which: you know the topology

**Available from existing data:**
- Account names and addresses: already in users.csv
- System pressure: `system_pressure_value` from OTel
- Tank level: `concrete_tank_level_value` from OTel
- Well depth: `well_depth_value` from OTel

**Can be derived automatically:**
- Elevation: from USGS DEM data (script does this — see Part 3)
- Pipe length: from coordinates (script does this)
- Roughness: from material (script does this)

**The hard part is connectivity.** Drawing the network on paper
first helps. Which pipes connect to which junctions? Where are
the tees? Where does the main line branch? Sketch it on paper,
assign IDs, then type the CSV.

---

## Part 3: Elevation Lookup

You can fill `elevation_ft` by hand if you know it. But for
junctions where you only have coordinates, a script can query
the USGS Elevation Point Query Service:

```bash
# Query elevation for a single point
curl -s "https://epqs.nationalmap.gov/v1/json?x=-123.81234&y=39.36152&units=Feet&wkid=4326" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['value'])"
```

A build script can iterate through nodes.csv, query each point
that lacks elevation, and fill it in. The USGS service is free,
no API key needed, accurate to ~3 meters vertically.

Or, if you know static pressure at a point and the tank elevation:

```
elevation = tank_elevation - (static_psi / 0.433)
```

This is often more accurate than the DEM. If you have `static_psi`
in your CSV, the build script can derive elevation from it.

---

## Part 4: The Build Script

The build script reads your CSVs and produces all derived outputs.
It can be written in Go (fits your stack), Python (quick to
prototype), or as a Makefile calling ogr2ogr and small utilities.

### What it does

```
nodes.csv + pipes.csv + valves.csv
    │
    ├── Validate: every from_node/to_node exists in nodes.csv
    ├── Fill defaults: roughness from material, elevation from DEM
    ├── Calculate: pipe lengths from coordinates (Haversine)
    │
    ├──► network.geojson     GeoJSON FeatureCollection
    │      Points for nodes, LineStrings for pipes
    │      All attributes as properties
    │
    ├──► system.inp          EPANET input file
    │      [JUNCTIONS], [TANKS], [RESERVOIRS], [PIPES],
    │      [VALVES], [COORDINATES], [OPTIONS], etc.
    │
    ├──► network.gpkg        GeoPackage (via ogr2ogr)
    │      For anyone who wants to open it in QGIS
    │
    └──► assets.json         Asset catalog for the website
           Structured data for the web map popups
```

### GeoJSON output format

```json
{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "geometry": { "type": "Point", "coordinates": [-123.81234, 39.36152] },
      "properties": {
        "id": "TANK-01", "type": "TANK",
        "elevation_ft": 1225,
        "tank_diameter_ft": 15, "tank_max_level_ft": 8
      }
    },
    {
      "type": "Feature",
      "geometry": {
        "type": "LineString",
        "coordinates": [[-123.81234, 39.36152], [-123.81400, 39.36100]]
      },
      "properties": {
        "id": "MAIN-01", "from_node": "TANK-01", "to_node": "TEE-A",
        "diameter_in": 6, "material": "PVC", "roughness": 150,
        "length_ft": 523.7
      }
    }
  ]
}
```

### EPANET .inp output

The script writes standard EPANET sections from the CSV data.
The existing `fireflow.epanet` file shows the format. The build
script generates a complete version with all your nodes and pipes.

### GeoPackage output (optional)

If you have GDAL installed (`brew install gdal` on Mac):

```bash
ogr2ogr -f GPKG network.gpkg network.geojson
```

One command. Now anyone with QGIS can open your data. You never
touch QGIS yourself.

### Makefile integration

```makefile
# In model/Makefile
GEOJSON := network.geojson
INP := system.inp
GPKG := network.gpkg

all: $(GEOJSON) $(INP) $(GPKG)

$(GEOJSON) $(INP): nodes.csv pipes.csv valves.csv
	go run ./cmd/buildmodel -nodes nodes.csv -pipes pipes.csv \
	    -valves valves.csv -geojson $(GEOJSON) -inp $(INP)

$(GPKG): $(GEOJSON)
	ogr2ogr -f GPKG $@ $<

clean:
	rm -f $(GEOJSON) $(INP) $(GPKG)
```

Edit your CSVs, run `make`, get everything. Reproducible.

---

## Part 5: Running EPANET

### Option A: Command-line (offline)

The EPANET command-line tool runs a simulation and writes a report:

```bash
# Install EPANET CLI (or build from source: github.com/OpenWaterAnalytics/EPANET)
epanet system.inp system.rpt system.out
```

The report file (`system.rpt`) contains pressure at every junction,
flow in every pipe, velocity, head loss — everything.

### Option B: Python with WNTR

```bash
pip install wntr
```

```python
import wntr

wn = wntr.network.WaterNetworkModel('system.inp')
sim = wntr.sim.EpanetSimulator(wn)
results = sim.run_sim()

# Pressure at every junction over time
print(results.node['pressure'])

# Pressure at a specific junction at hour 0
print(results.node['pressure'].loc[0, 'J-01'])
```

WNTR (Water Network Tool for Resilience) is an EPA-funded Python
library. It reads .inp files, runs EPANET, and gives you results
as pandas DataFrames. Excellent for scripting scenarios.

### Option C: Browser with epanet-js

```javascript
import { Project, Workspace } from "epanet-js";

const ws = new Workspace();
await ws.loadModule();
const model = new Project(ws);

ws.writeFile("system.inp", inpContent);
model.open("system.inp", "report.rpt", "output.bin");
model.solveH();
model.saveH();

const idx = model.getNodeIndex("J-01");
const psi = model.getNodeValue(idx, NodeProperty.Pressure);
```

This runs entirely in the browser via WASM. No server. But for
studying the model, WNTR on the command line is more productive.

### Worst-case scenario: everyone runs a spigot

With WNTR, this is a few lines:

```python
import wntr

wn = wntr.network.WaterNetworkModel('system.inp')

# Add 8 GPM demand to every junction (garden hose)
for name, node in wn.junctions():
    node.demand_timeseries_list[0].base_value += 8 / 15850.3  # GPM to m3/s

sim = wntr.sim.EpanetSimulator(wn)
results = sim.run_sim()

# Show pressure at all junctions
pressure = results.node['pressure'].loc[0]  # hour 0
for junc_name, psi in pressure.items():
    print(f"{junc_name}: {psi:.1f} PSI")
```

Run it with 1 hose per connection, 2 hoses, fire flow at a
hydrant — whatever scenarios you want. Script it, don't click it.

---

## Part 6: The Web Map

A static web page with Leaflet and OpenStreetMap tiles. It loads
your GeoJSON, draws the network, and shows asset info on click.

### What it looks like

- **Base layer**: OpenStreetMap tiles (free, no API key needed)
- **Pipe layer**: Lines colored by diameter or material
- **Node layer**: Circles colored by type (junction/tank/well)
- **Click any feature**: Popup with all attributes from the CSV
- **Optional simulation overlay**: Pressure colors from EPANET

### Minimal implementation

```html
<!DOCTYPE html>
<html>
<head>
  <link rel="stylesheet" href="https://unpkg.com/leaflet/dist/leaflet.css"/>
  <script src="https://unpkg.com/leaflet/dist/leaflet.js"></script>
  <style>
    #map { height: 100vh; width: 100%; }
  </style>
</head>
<body>
  <div id="map"></div>
  <script>
    const map = L.map('map').setView([39.361, -123.813], 15);

    // OpenStreetMap tiles — free, no API key
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '&copy; OpenStreetMap contributors'
    }).addTo(map);

    // Load your network
    fetch('network.geojson')
      .then(r => r.json())
      .then(data => {
        L.geoJSON(data, {
          style: feature => {
            if (feature.geometry.type === 'LineString') {
              const d = feature.properties.diameter_in;
              return {
                weight: Math.max(2, d),
                color: d >= 6 ? '#2196F3' : d >= 2 ? '#4CAF50' : '#FF9800'
              };
            }
          },
          pointToLayer: (feature, latlng) => {
            const t = feature.properties.type;
            const color = t === 'TANK' ? '#1565C0'
                        : t === 'RESERVOIR' ? '#0D47A1'
                        : '#4CAF50';
            return L.circleMarker(latlng, {
              radius: t === 'JUNCTION' ? 5 : 8,
              fillColor: color,
              fillOpacity: 0.8,
              weight: 1, color: '#333'
            });
          },
          onEachFeature: (feature, layer) => {
            const p = feature.properties;
            let html = `<b>${p.id}</b> (${p.type})<br>`;
            for (const [k, v] of Object.entries(p)) {
              if (k !== 'id' && k !== 'type' && v)
                html += `${k}: ${v}<br>`;
            }
            layer.bindPopup(html);
          }
        }).addTo(map);
      });
  </script>
</body>
</html>
```

This is ~40 lines. It loads your GeoJSON, draws pipes colored by
diameter, draws nodes colored by type, and shows a popup with every
attribute when you click anything. No build step, no framework, no
dependencies beyond Leaflet and OSM tiles.

### Free map tile options

| Provider          | API key | Cost | Quality            |
|-------------------|---------|------|--------------------|
| **OpenStreetMap**  | No      | Free | Street map         |
| **Stamen Terrain** | No     | Free | Terrain + labels   |
| **ESRI Satellite** | No*    | Free | Satellite imagery  |
| **Mapbox**         | Yes    | Free tier | Satellite + custom|

*ESRI provides a free tile URL for non-commercial use:*
```
https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}
```

Toggle between street and satellite view with Leaflet's layer
control.

---

## Part 7: Asset Catalog

The web map is your asset catalog. Every feature has all its
attributes accessible in the popup. But you can also generate a
standalone asset inventory page from the same CSV data:

### What to track per asset

**Pipes:**
- ID, from/to, diameter, material, roughness
- Install year, expected life, replacement priority
- Condition notes, last inspection date

**Valves:**
- ID, type, diameter, location
- Last exercised date, operational status
- Notes (stuck, leaking, needs replacement)

**Meters:**
- Account, location, meter make/model/serial
- Install date, last read date
- AMR compatible (yes/no), protocol (Itron ERT, etc.)

**Hydrants (if any):**
- ID, location, make/model
- Last flow test, static/residual pressure
- GPM available at 20 PSI residual

You can add columns to your CSVs as needed. The build script
passes them through to GeoJSON properties and the web map shows
them automatically.

---

## Part 8: Connecting to Your Telemetry

### Sensor placement in the model

When you create nodes.csv, note which junctions have sensors:

```csv
id,type,lat,lon,elevation_ft,demand_gpm,static_psi,account,address,notes
PRESS-PT,JUNCTION,39.360,-123.814,1105,0,48,,,system_pressure_value sensor location
```

The `notes` field records the OTel metric name. This is how you
know which model junction to compare against real data.

### Calibration (scripted)

```python
import wntr

# Load model
wn = wntr.network.WaterNetworkModel('system.inp')
sim = wntr.sim.EpanetSimulator(wn)
results = sim.run_sim()

# Model says:
model_psi = results.node['pressure'].loc[0, 'PRESS-PT']

# OTel says (query from duckpond or InfluxDB):
measured_psi = 48.0  # from system_pressure_value at low-demand time

print(f"Model: {model_psi:.1f} PSI")
print(f"Measured: {measured_psi:.1f} PSI")
print(f"Error: {abs(model_psi - measured_psi):.1f} PSI")
```

Adjust roughness values in pipes.csv until the error is small.
Roughness is the main tuning knob.

---

## Part 9: Your Existing Data, Mapped

| Your metric                  | Model equivalent               | Use                          |
|------------------------------|-------------------------------|------------------------------|
| `system_pressure_value`      | Pressure at a junction         | Calibration anchor           |
| `water_pressure`             | Pressure at presskit location  | Second calibration point     |
| `concrete_tank_level_value`  | Tank level (InitLevel)         | Initial conditions           |
| `well_depth_value`           | Reservoir head                 | Source boundary condition    |
| `chlorine_level_value`       | [QUALITY] initial value        | Water quality model (future) |
| `wellprobe_pressure`         | Well water level as head       | Source characterization      |
| users.csv accounts           | Junction demand attribution    | Demand assignment            |

---

## Part 10: Ongoing Management

### When to update the CSVs

| Event                        | Edit                              |
|------------------------------|-----------------------------------|
| New service connection       | Add row to nodes.csv + pipes.csv  |
| Pipe replacement/repair      | Update diameter, material, year   |
| Valve installed/removed      | Add/remove row in valves.csv      |
| Meter replaced               | Update notes                      |
| New well or pump             | Add RESERVOIR row to nodes.csv    |
| Better diameter info         | Update `diameter_in` in pipes.csv |
| Pressure reading at new point| Add `static_psi`, rebuild, recalibrate |

### After any edit

```bash
cd model && make   # rebuilds geojson, inp, gpkg
```

### Periodic tasks

| Task                       | Frequency   | Purpose                  |
|----------------------------|-------------|--------------------------|
| Rebuild and deploy web map | After edits | Keep map current         |
| Run calibration script     | Quarterly   | Catch model drift        |
| Back up CSVs               | Weekly      | Data protection (git)    |
| Run worst-case scenario    | Annually    | System capacity check    |

The CSVs are plain text. They live in git. Every change is tracked.

---

## Part 11: File Layout

```
model/
├── nodes.csv              # YOU EDIT THIS — network nodes
├── pipes.csv              # YOU EDIT THIS — pipe segments
├── valves.csv             # YOU EDIT THIS — valves (optional)
│
├── Makefile               # Run 'make' to build everything
├── cmd/buildmodel/        # Build script (Go)
│
├── network.geojson        # GENERATED — web map + QGIS import
├── system.inp             # GENERATED — EPANET simulation
├── network.gpkg           # GENERATED — GeoPackage for QGIS users
├── assets.json            # GENERATED — structured asset data
│
├── scenarios/             # WNTR/EPANET scenario scripts
│   ├── worst_case.py      #   Everyone runs a hose
│   ├── fire_flow.py       #   Hydrant flow test
│   └── pipe_break.py      #   Main line failure
│
├── web/                   # Static web map
│   ├── index.html         #   Leaflet + OSM + GeoJSON
│   └── style.css
│
├── fireflow.epanet        # Original reference model
├── GUIDEBOOK.md           # Architecture & format guide
├── POSITION_PAPER.md      # Project vision
└── QGIS_DATA_MODELS.md    # Data model reference
```

**You edit 3 CSV files. `make` produces everything else.**

---

## Part 12: Quick Reference

### Pressure & elevation

```
PSI = elevation_difference × 0.433
elevation_difference = PSI ÷ 0.433

50 PSI ≈ 115 ft of head
35 PSI ≈  81 ft of head
60 PSI ≈ 139 ft of head
```

### Roughness by material

```
PVC:  150    poly: 140    galv: 120    steel: 130
copper: 135  DI:   130    CI:    100
```

### Typical demands

```
Single fixture:      1-2 GPM
Household average:   2-5 GPM
Garden hose (1/2"):  4-6 GPM
Garden hose (3/4"):  8-12 GPM
Fire hydrant:        500-1500 GPM
```

### USGS elevation query

```bash
curl -s "https://epqs.nationalmap.gov/v1/json?x=LONGITUDE&y=LATITUDE&units=Feet&wkid=4326"
```
