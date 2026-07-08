# Hydraulic Model & GIS Guidebook for Small Water Systems

Free software toolkit for water system operators to build interactive
hydraulic models of their distribution networks — from GIS data capture
through EPANET simulation to browser-based pressure visualization.

Part of an open-source platform for small water system operations:

| Repository | Purpose | Stack |
|------------|---------|-------|
| [caspar.water](https://github.com/jmacd/caspar.water) | Telemetry, monitoring, billing, website | Go, OpenTelemetry, MQTT/Sparkplug |
| [watertown](https://github.com/jmacd/watertown) | Time-series data lake, SQL transforms, site generation | Rust, Arrow, Delta Lake, Observable |
| [supruglue](https://github.com/jmacd/supruglue) | Real-time hardware I/O (meter reading, pump control) | C, TI PRU, BeagleBone |

Built and tested on the Caspar Water System in Mendocino County, CA,
but designed for any small water utility. Apache-2.0 license.

---

## 1. Who This Is For

You operate a small water system — maybe 10 connections, maybe 500.
You know where your pipes are. You probably know the diameters. You
may have pressure readings at a few points. You want to:

- **See what happens to pressure** when customers run hoses, fill
  pools, or fight fires
- **Show your customers** why leaving hoses running affects their
  neighbors
- **Make infrastructure decisions** about where to upsize pipe
- **Put an interactive model on your website** so people can explore
  the system themselves

This guide gets you from "I know where my pipes are" to a working
browser-based hydraulic simulator with no commercial software required.

---

## 2. Architecture Overview

### How hydraulic modeling fits into the platform

The platform already handles real-time sensor data (supruglue →
caspar.water OTel collector), time-series storage and SQL transforms
(watertown), and browser-based visualization with WASM (watertown
sitegen → Observable Framework + DuckDB WASM). The hydraulic model
adds one new capability: **simulation** — answering "what if?"
questions about the distribution network.

```
                    ┌─────────────────────────────────────────────┐
  supruglue         │ Real-time: meters, pumps, sensors (PRU/BBB) │
                    └──────────────────┬──────────────────────────┘
                                       │
                    ┌──────────────────▼──────────────────────────┐
  caspar.water      │ Telemetry: OTel collector, MQTT, InfluxDB   │
                    │ Operations: billing, operator tools          │
                    └──────────────────┬──────────────────────────┘
                                       │
                    ┌──────────────────▼──────────────────────────┐
  watertown          │ Data lake: Arrow/Parquet, SQL transforms     │
                    │ Site generation: Observable Framework pages  │
                    └──────────────────┬──────────────────────────┘
                                       │
         ┌─────────────────────────────▼───────────────────────┐
         │                   Website                            │
         │                                                      │
         │  ┌── Monitoring ──┐  ┌── Hydraulic Model ──────────┐ │
         │  │ DuckDB WASM    │  │ Leaflet map (GeoJSON)       │ │
         │  │ Time-series    │  │ epanet-js (WASM)            │ │
         │  │ charts         │  │ Interactive pressure sim    │ │
         │  └────────────────┘  └─────────────────────────────┘ │
         └─────────────────────────────────────────────────────┘
```

### Data flow for the hydraulic model

```
QGIS + GeoPackage  (operator's working data)
    │
    ├──► GeoJSON ──► website map + watertown sitegen
    │
    └──► .inp    ──► epanet-js WASM simulation (in browser)
                 ──► QGIS Gusnet plugin (desktop validation)
```

Real-time pressure data from the OTel collector can also feed
model calibration: compare simulated pressure to measured pressure
and adjust roughness coefficients until they match.

### Quick-start alternative (no QGIS)

For operators who want to start fast and learn QGIS later:

```
Google My Maps ──► KML export ──► GeoJSON ──► EPANET .inp
```

Both paths converge on GeoJSON as the web format and EPANET .inp
as the simulation format. QGIS just makes the middle steps better.

---

## 3. GIS Format Strategy

### GeoPackage (.gpkg) — Canonical data store

- **What**: SQLite-based OGC standard; one file holds all your layers
- **Why**: Schema enforcement (diameter must be a number, node_type
  must be JUNCTION/TANK/RESERVOIR), spatial indexing, scales from 10
  to 10,000 features, multiple layers in one file
- **Where**: Lives in the operator's QGIS project; the authoritative
  record of the distribution network
- **Trade-off**: Binary file, not git-diffable. That's OK — the
  GeoPackage is the operator's working data, not a developer artifact

### GeoJSON — Web exchange format

- **What**: JSON-based, human-readable, directly loadable in any
  browser
- **Why**: JavaScript can `fetch()` it and render it on a map.
  Readable in any text editor. Easy to inspect and debug.
- **Where**: Exported from QGIS (or converted from KML). Committed
  to the repo. Consumed by the website.
- **Limitation**: No schema enforcement, WGS84 only, gets unwieldy
  above ~10MB. Fine for distribution networks.

### KML — Intermediate/on-ramp format

- **What**: XML-based, exported by Google My Maps
- **Why**: Easy way to get started without installing anything.
  Operators can sketch their network on Google satellite imagery
  in minutes.
- **Where**: Stepping stone → convert to GeoJSON or import into QGIS
- **Limitation**: Verbose, no structured attributes, not good for
  long-term storage

### EPANET .inp — Simulation format

- **What**: Text-based input format for the EPANET hydraulic solver
- **Why**: Industry standard since 1993. Every hydraulic modeling tool
  reads it. epanet-js reads it in the browser.
- **Where**: Generated from GeoJSON/GeoPackage by a conversion script,
  or exported directly from QGIS via the Gusnet plugin

### Format flow summary

```
Operator's data (QGIS + GeoPackage)
    │
    ├──► GeoJSON  ──► website map display
    │
    └──► .inp     ──► epanet-js hydraulic simulation (in browser)
                  ──► desktop EPANET / Gusnet (for validation)
```

---

## 4. QGIS — The Recommended Workflow

QGIS is a free, open-source desktop GIS. It runs on Windows, Mac,
and Linux. For water network modeling, it replaces several expensive
commercial tools.

### Install

- Download: https://qgis.org/download/
- Current LTS: QGIS 3.34+

### Why QGIS over Google My Maps

| Capability                    | Google My Maps | QGIS          |
|-------------------------------|---------------|---------------|
| Quick sketch of network       | ✓ Easy        | ✓ Doable      |
| Structured attribute entry    | Awkward       | ✓ With forms  |
| Elevation from terrain data   | Manual        | ✓ Automatic   |
| Snap pipes to junctions       | No            | ✓ Precise     |
| Validate network topology     | No            | ✓ Yes         |
| Run EPANET simulations        | No            | ✓ Gusnet      |
| Export to GeoJSON              | Via KML       | ✓ Direct      |
| Export to GeoPackage           | No            | ✓ Native      |
| Color-code by diameter/PSI    | No            | ✓ Graduated   |
| Print maps for reports        | Screenshot    | ✓ Layouts     |
| Works offline                 | No            | ✓ Yes         |
| Learning curve                | Minutes       | Hours         |

Google My Maps is a fine on-ramp. But QGIS is where you'll want to
be for ongoing network management.

### Core capabilities for water operators

**Sketching with snapping**
Draw pipes that snap precisely to junction points. No dangling
endpoints, no gaps in your network topology. QGIS enforces
connectivity that the hydraulic model requires.

**Attribute tables with forms**
Structured data entry: pipe diameter is a number field, material is
a dropdown (PVC, steel, poly, etc.), node type is constrained to
JUNCTION/TANK/RESERVOIR. Reduces data entry errors.

**DEM elevation lookup**
Load free USGS elevation data and automatically extract ground
elevation at every junction. No surveyor needed for initial modeling.
(See Section 5 for elevation data sources.)

**Layer styling**
Color-code pipes by diameter (thin blue for 1", thick red for 6").
Show pressure results as graduated colors at junctions. See your
network's characteristics at a glance.

**Coordinate reference systems**
QGIS handles projection math. Your data stays in lat/lon (WGS84)
for web compatibility but you can measure and display in feet or
meters.

### Water network plugins

**Gusnet** (recommended) — https://www.gusnet.org/
- Integrates EPANET + WNTR (Water Network Tool for Resilience)
  directly inside QGIS
- Draw your network on the map, assign properties, run simulations
  without leaving QGIS
- Visualize pressure, flow, and velocity as colored overlays
- Export to .inp files for use with epanet-js on the website
- Active development, good documentation, multilingual
- Install: Plugins → Manage and Install → search "Gusnet"

**QGISRed** — https://qgisred.upv.es/en/
- EPANET integration focused on digital twin workflows
- Mature UI for network editing
- Strong for calibration and scenario analysis

**GHydraulics** — http://epanet.de/ghydraulics/
- Simpler, older plugin for basic EPANET integration
- Economic pipe diameter calculations

### Setting up a project

1. Install QGIS and the QuickMapServices plugin
2. Add a basemap: `Web → QuickMapServices → Google Satellite`
3. Create a GeoPackage with two layers:

   **Layer: nodes** (Point geometry)
   | Field           | Type    | Description                   |
   |-----------------|---------|-------------------------------|
   | id              | Text    | Unique node ID                |
   | node_type       | Text    | JUNCTION, TANK, or RESERVOIR  |
   | elevation_ft    | Real    | Ground elevation (feet)       |
   | base_demand_gpm | Real    | Normal demand (GPM)           |
   | static_psi      | Real    | Known static pressure (PSI)   |
   | notes           | Text    | Address, description          |

   **Layer: pipes** (LineString geometry)
   | Field        | Type    | Description                      |
   |-------------|---------|----------------------------------|
   | id           | Text    | Unique pipe ID                   |
   | diameter_in  | Real    | Pipe diameter (inches)           |
   | material     | Text    | PVC, steel, poly, etc.           |
   | roughness    | Real    | Hazen-Williams C (auto if blank) |
   | notes        | Text    | Description                      |

4. Digitize your network on top of satellite imagery
5. Enable snapping: `Project → Snapping Options → All Layers`
6. Use Gusnet to validate with a simulation
7. Export layers as GeoJSON for the website

---

## 5. Elevation Data

Elevation is critical for hydraulic modeling — pressure is directly
determined by the height difference between the water source and
the delivery point (roughly 0.433 PSI per foot of elevation).

### Free sources

**USGS National Map** (US systems)
- https://apps.nationalmap.gov/downloader/
- Product: "1/3 arc-second DEM" (~10m resolution)
- Format: GeoTIFF
- Coverage: All 50 states
- Load in QGIS, then: `Processing → Toolbox → Sample raster values`
  to extract elevation at each junction

**OpenTopography** (global, higher resolution available)
- https://opentopography.org/
- Lidar-derived DEMs where available (sub-meter resolution)

**SRTM** (global, 30m resolution)
- Available via QGIS plugin: `SRTM Downloader`

### Elevation from pressure readings

If you know static pressure at a point and the tank water surface
elevation, you can back-calculate:

```
junction_elevation = tank_elevation - (static_psi / 0.433)
```

Example: Tank at 1225 ft, static pressure reads 50 PSI:
```
junction_elevation = 1225 - (50 / 0.433) = 1225 - 115.5 = 1109.5 ft
```

This is often more accurate than DEM data for points near buildings.

---

## 6. Converting Data to EPANET Format

### From QGIS (recommended)

The Gusnet plugin exports .inp files directly. This is the simplest
path: draw your network, assign properties, export.

### From GeoJSON (for the website pipeline)

A conversion script reads GeoJSON features and writes .inp sections.
The script handles:

- Pipe length calculation from LineString coordinates (Haversine)
- Roughness defaults by material
- `from_node` / `to_node` inference from LineString endpoints
  (matching to nearest Point features)

### Roughness defaults by material

| Material       | Hazen-Williams C | Notes                  |
|---------------|-----------------|------------------------|
| PVC            | 150             | Smooth, modern         |
| HDPE / Poly    | 140             | Flexible, common rural |
| Ductile iron   | 130             | New                    |
| Cast iron      | 100             | Older systems          |
| Galvanized     | 120             | Service lines          |
| Copper         | 135             | Service lines          |
| Steel (new)    | 130             |                        |
| Steel (old)    | 100             | Tuberculated           |

### GeoJSON schema expected by the converter

```json
{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "geometry": { "type": "Point", "coordinates": [-123.812, 39.361] },
      "properties": {
        "id": "TANK-01",
        "node_type": "TANK",
        "elevation_ft": 1225,
        "tank_diameter_ft": 15,
        "tank_max_level_ft": 8,
        "tank_min_level_ft": 0,
        "tank_init_level_ft": 8
      }
    },
    {
      "type": "Feature",
      "geometry": { "type": "Point", "coordinates": [-123.815, 39.362] },
      "properties": {
        "id": "J-01",
        "node_type": "JUNCTION",
        "elevation_ft": 350,
        "base_demand_gpm": 0,
        "notes": "Residential connection"
      }
    },
    {
      "type": "Feature",
      "geometry": {
        "type": "LineString",
        "coordinates": [[-123.812, 39.361], [-123.814, 39.3615], [-123.815, 39.362]]
      },
      "properties": {
        "id": "P-01",
        "diameter_in": 6,
        "material": "PVC"
      }
    }
  ]
}
```

Note: pipes do not need explicit `from_node` / `to_node` fields.
The converter matches LineString endpoints to the nearest Point
features automatically. Trace pipes along their actual route for
accurate length calculation.

---

## 7. EPANET Model Concepts for Operators

### What EPANET solves

Given a network of pipes, tanks, pumps, and valves with known
elevations and demands, EPANET calculates:

- **Pressure** at every junction (PSI)
- **Flow** in every pipe (GPM)
- **Velocity** in every pipe (ft/s)
- **Head loss** across every pipe
- **Tank levels** over time

### Nodes

| Type       | What it represents                    |
|-----------|---------------------------------------|
| JUNCTION   | A connection point, service tap, tee  |
| TANK       | Storage tank with variable level      |
| RESERVOIR  | Fixed-head source (well, spring)      |

### Links

| Type   | What it represents                       |
|--------|------------------------------------------|
| Pipe   | A pipe segment between two nodes         |
| Pump   | A pump (requires a head-capacity curve)  |
| Valve  | PRV, PSV, or other control valve         |

### Demands

Demands represent water use at junctions, in GPM. Typical values:

| Use case                 | Demand (GPM) |
|-------------------------|-------------|
| Single fixture           | 1–2         |
| Household (typical)      | 2–5         |
| Garden hose (½")         | 4–6         |
| Garden hose (¾")         | 8–12        |
| Sprinkler system         | 5–15        |
| Fire hydrant             | 500–1500    |

### Pressure-dependent demand (PDA)

By default EPANET delivers demanded flow regardless of pressure.
For realistic modeling of low-pressure scenarios (like the garden
hose problem), enable pressure-dependent demand:

```javascript
model.setDemandModel(DemandModel.PDA, pmin, preq, pexp);
```

- `pmin`: Minimum pressure for any flow (e.g., 5 PSI)
- `preq`: Required pressure for full demand (e.g., 30 PSI)
- `pexp`: Pressure exponent (typically 0.5)

This makes demand decrease realistically as pressure drops.

---

## 8. Website Integration with epanet-js

### What is epanet-js?

The OWA-EPANET 2.2 hydraulic solver (written in C) compiled to
WebAssembly. Runs entirely in the browser — no server needed.
Any operator's .inp model can be loaded and simulated client-side.

- Package: `epanet-js` on npm
- Engine: `@model-create/epanet-engine` (WASM binary)
- License: MIT
- Repository: https://github.com/epanet-js/epanet-js-toolkit

### How it fits with the existing platform

The platform already runs WASM in the browser (DuckDB WASM via
watertown's Observable Framework sites). epanet-js is another WASM
module alongside it. Watertown's sitegen can generate the page that
hosts both:

- **DuckDB WASM** queries real pressure history from Parquet files
- **epanet-js WASM** simulates hypothetical pressure scenarios
- **Same page** shows "here's what pressure actually was" next to
  "here's what happens if three hoses turn on"

The OTel collector already records `water_pressure` metrics.
Watertown already reduces and exports that data as Parquet.
The simulation model gives that data predictive context.

### Basic usage

```javascript
import { Project, Workspace } from "epanet-js";

const ws = new Workspace();
await ws.loadModule();
const model = new Project(ws);

// Load any system's .inp file
ws.writeFile("system.inp", inpFileContent);
model.open("system.inp", "report.rpt", "output.bin");

// Solve
model.solveH();
model.saveH();

// Read pressure at any junction
const idx = model.getNodeIndex("J-01");
const psi = model.getNodeValue(idx, NodeProperty.Pressure);

model.close();
```

### Interactive pressure explorer

The web page combines a map with the hydraulic solver:

1. **Load**: Fetch the operator's `network.geojson` and `system.inp`
2. **Display**: Render the network on a Leaflet/Mapbox map
3. **Interact**: User clicks a junction → "Turn on garden hose here"
4. **Simulate**: Add demand (+8 GPM), re-solve hydraulics
   (milliseconds for small networks)
5. **Visualize**: Update pressure colors at all junctions
6. **Explore**: Turn on multiple hoses, see cumulative pressure drop

This is a static page — no backend server. The WASM solver runs
entirely in the visitor's browser. The operator just needs to host
two files (GeoJSON + .inp) alongside the web page.

### Calibration with real data

If the system has pressure sensors reporting to the OTel collector,
the model can be calibrated against reality:

1. Query watertown for historical pressure at monitored junctions
2. Run the EPANET model with known demand patterns
3. Compare simulated vs. measured pressure
4. Adjust pipe roughness coefficients until they converge
5. A calibrated model gives credible predictions

This closes the loop between the monitoring platform (what is
happening) and the simulation model (what would happen if).

---

## 9. Quick-Start Path (Google My Maps)

For operators who want to get started immediately without QGIS:

1. Go to https://www.google.com/maps/d/
2. Create two layers (Nodes and Pipes)
3. Drop pins at every tank, connection, tee, and branch point
   - Name each pin with an ID
   - Put elevation/pressure/type in the description
4. Draw lines along each pipe segment
   - Name each line
   - Put diameter and material in the description
5. Export as KML
6. Convert to GeoJSON:
   - Web: drag into https://geojson.io and save
   - CLI: `ogr2ogr -f GeoJSON network.geojson export.kml`
   - Python: `geopandas.read_file("export.kml").to_file("network.geojson")`
7. Add structured properties to the GeoJSON (the description text
   from Google Maps will need to be parsed into proper fields)

This gets data captured quickly. Migrate to QGIS when you want
better control over the model.

---

## 10. Project File Layout

```
model/
├── GUIDEBOOK.md           # This document
├── fireflow.epanet        # Reference EPANET model
├── network_data.csv       # Data entry template
├── network.geojson        # GIS data for website (generated)
├── system.inp             # EPANET model (generated)
└── geojson_to_inp.js      # Conversion script
```

For QGIS users, the `.qgz` project file and `.gpkg` GeoPackage
live alongside these but may not be committed to version control
(binary files).

---

## 11. Reference: Pressure and Elevation Math

### Static pressure from elevation

```
pressure_psi = (source_elevation - junction_elevation) × 0.433
```

0.433 PSI per foot of water column (at 60°F).

### Examples

| Source (ft) | Junction (ft) | Head (ft) | Static PSI |
|------------|---------------|-----------|------------|
| 1225       | 1100          | 125       | 54.1       |
| 1225       | 1000          | 225       | 97.4       |
| 1225       | 1150          | 75        | 32.5       |
| 500        | 350           | 150       | 64.9       |

### Dynamic pressure loss

Static pressure is reduced by friction losses in pipes. Losses
depend on flow rate, pipe diameter, pipe length, and roughness.
Small-diameter pipes lose pressure much faster than large ones —
which is exactly why the garden hose scenario is interesting.

The Hazen-Williams formula (used by EPANET):
```
h_f = 10.67 × L × Q^1.852 / (C^1.852 × D^4.87)
```
Where h_f = head loss (ft), L = length (ft), Q = flow (ft³/s),
C = roughness coefficient, D = diameter (ft).

You don't need to compute this — EPANET does it. But understanding
it helps explain to customers why pipe diameter matters.
