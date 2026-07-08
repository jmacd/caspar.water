# QGIS Ecosystem: Data Models, File Formats & Use Cases for Water Systems

A comprehensive reference for understanding the spatial data ecosystem
relevant to small water system operations — geometry types, storage
formats, coordinate systems, the EPANET simulation model, asset
management schemas, field tools, and how they all interconnect.

---

## Table of Contents

1. [Foundational Concepts: How Spatial Data Works](#1-foundational-concepts)
2. [Geometry Types & the Vector Data Model](#2-geometry-types)
3. [Raster Data Model (Elevation, Imagery)](#3-raster-data)
4. [Coordinate Reference Systems (CRS)](#4-coordinate-reference-systems)
5. [File Formats Compared](#5-file-formats)
6. [GeoPackage Deep Dive](#6-geopackage)
7. [GeoJSON Deep Dive](#7-geojson)
8. [EPANET Data Model](#8-epanet-data-model)
9. [Water Utility Data Model (Giswater)](#9-giswater-data-model)
10. [QGIS Plugin Ecosystem for Water](#10-qgis-plugins)
11. [QField: Mobile Data Collection](#11-qfield)
12. [How Everything Connects](#12-how-everything-connects)
13. [Mapping to the Caspar Platform](#13-mapping-to-caspar)

---

## 1. Foundational Concepts

GIS data has two parts: **geometry** (where things are) and
**attributes** (what things are).

```
Feature = Geometry + Attributes
        = WHERE      + WHAT

Example:
  Geometry:   POINT(-123.812, 39.361)
  Attributes: { id: "VALVE-07", type: "gate", diameter: 6, status: "open" }
```

Everything in a GIS is either:
- **Vector data**: Discrete features with precise boundaries
  (points, lines, polygons). Used for pipes, valves, parcels.
- **Raster data**: Continuous grids of cells, each with a value.
  Used for elevation, satellite imagery, terrain.

Both types are georeferenced — they know where they are on the
Earth's surface, via a coordinate reference system.

---

## 2. Geometry Types & the Vector Data Model

### Primitive types

| Type           | Dimensions | Shape              | Water system examples              |
|---------------|-----------|--------------------|------------------------------------|
| **Point**      | 0D        | Single coordinate  | Valve, meter, hydrant, junction    |
| **LineString** | 1D        | Ordered vertices   | Pipe, main, service line           |
| **Polygon**    | 2D        | Closed ring        | Tank footprint, easement, parcel   |

### Multi-types (collections)

| Type                | Contains          | Example                            |
|--------------------|-------------------|------------------------------------|
| **MultiPoint**      | Multiple points   | Cluster of sample locations        |
| **MultiLineString** | Multiple lines    | Pipe with disconnected segments    |
| **MultiPolygon**    | Multiple polygons | Non-contiguous service area        |
| **GeometryCollection** | Mixed types    | Rarely used; avoid if possible     |

### Coordinate dimensions

- **XY**: Longitude and latitude (2D) — most common
- **XYZ**: Adds elevation — useful for pipes with known depth/height
- **XYM**: Adds a "measure" value — used for linear referencing
  (distance along a pipe from a known point)
- **XYZM**: Both elevation and measure

For water networks, **XYZ** is ideal — pipe elevation matters for
hydraulic calculations. But XY with elevation stored as an attribute
works too.

### Topology

Topology describes how features relate spatially:

| Relationship      | Meaning                              | Example                         |
|------------------|--------------------------------------|---------------------------------|
| **Connectivity**  | Features meet at a shared point      | Pipes meet at a junction        |
| **Adjacency**     | Features share a boundary            | Neighboring parcels             |
| **Containment**   | One feature inside another           | Meter inside a parcel           |
| **Intersection**  | Features cross or overlap            | Pipe crossing a road            |

**Why topology matters for water networks**: Every pipe must connect
to exactly two nodes. No dangling endpoints. No gaps. No overlaps.
A topologically valid network is required for hydraulic simulation.
QGIS provides topology-checking tools that catch these errors.

### Spatial queries

Given topology, you can ask spatial questions:

```sql
-- Find all valves within 100 meters of a main break
SELECT * FROM valves
WHERE ST_DWithin(valves.geom, break_point.geom, 100);

-- Find which parcels are served by a specific pipe
SELECT parcels.* FROM parcels, pipes
WHERE ST_Intersects(parcels.geom, ST_Buffer(pipes.geom, 5))
AND pipes.id = 'MAIN-01';
```

These queries work in PostGIS (PostgreSQL), SpatiaLite (SQLite),
and through QGIS's spatial query tools.

---

## 3. Raster Data Model

### What rasters are

A grid of cells (pixels), each holding a numeric value. The grid
has a known origin point, cell size, and CRS.

```
┌────┬────┬────┬────┐
│ 320│ 335│ 350│ 360│   Each cell = elevation in feet
├────┼────┼────┼────┤   Cell size = 10 meters
│ 310│ 325│ 345│ 355│   Origin = (-123.82, 39.37)
├────┼────┼────┼────┤   CRS = EPSG:4326
│ 300│ 315│ 330│ 340│
└────┴────┴────┴────┘
```

### Types relevant to water systems

| Raster type              | Cell value represents    | Source                    |
|-------------------------|--------------------------|---------------------------|
| **DEM** (elevation)      | Ground elevation (ft/m)  | USGS, SRTM, LiDAR        |
| **Satellite imagery**    | RGB color values         | Google, Bing, NAIP, Esri  |
| **Hillshade**            | Shaded relief (derived)  | Computed from DEM         |
| **Slope**                | Steepness (degrees)      | Computed from DEM         |
| **Land use / land cover**| Classification code      | NLCD, OpenStreetMap       |

### DEM (Digital Elevation Model) sources

| Source                    | Resolution | Coverage | Format   | Cost |
|--------------------------|-----------|----------|----------|------|
| USGS 1/3 arc-second      | ~10m      | US       | GeoTIFF  | Free |
| USGS 1 arc-second (NED)  | ~30m      | US       | GeoTIFF  | Free |
| SRTM                     | ~30m      | Global   | GeoTIFF  | Free |
| GMTED2010                | ~250m     | Global   | GeoTIFF  | Free |
| LiDAR (OpenTopography)   | <1m       | Partial  | LAS/LAZ  | Free |

For water systems, the **USGS 1/3 arc-second DEM** is the sweet
spot: free, 10-meter resolution, covers all of the US, and precise
enough to derive junction elevations for EPANET models.

### DEM tiles

DEMs are distributed as rectangular tiles. A single DEM covering
Mendocino County might be 3600×3600 cells. QGIS can:
- Load multiple tiles and mosaic them
- Clip to your area of interest
- Extract point elevations (for pipe junctions)
- Compute slope, aspect, hillshade, contours

---

## 4. Coordinate Reference Systems (CRS)

Every piece of spatial data needs a CRS to mean anything. The CRS
defines how coordinates map to locations on Earth.

### The two families

**Geographic CRS** (unprojected)
- Coordinates are latitude and longitude (degrees)
- Earth modeled as an ellipsoid
- Most common: **WGS 84 (EPSG:4326)** — used by GPS, GeoJSON,
  Google Maps, and virtually all web mapping

**Projected CRS** (flat map)
- Coordinates are X/Y in linear units (meters or feet)
- Introduces distortion (area, angle, or distance — pick two)
- Used for measurement, analysis, and local mapping
- Examples: UTM zones, State Plane

### Key EPSG codes

| Code        | Name             | Units   | Use case                          |
|------------|------------------|---------|-----------------------------------|
| **4326**    | WGS 84           | Degrees | GPS, GeoJSON, web, data exchange  |
| **3857**    | Web Mercator     | Meters  | Google/Bing/OSM tile maps         |
| **26710**   | UTM Zone 10N     | Meters  | Northern California (accurate)    |
| **2226**    | CA State Plane 2 | Feet    | Mendocino County surveys          |

### Why this matters for water systems

- **Data exchange**: Always use EPSG:4326 (WGS 84) for GeoJSON,
  KML, and web maps
- **Pipe length calculation**: Use a projected CRS (UTM or State
  Plane) for accurate distance measurement in feet/meters
- **DEM alignment**: Ensure your DEM and vector data use the same
  CRS, or let QGIS reproject on-the-fly
- **EPANET**: Uses coordinates only for display; actual hydraulic
  calculation uses pipe length values, not geometry

QGIS handles CRS transparently — it reprojects layers on-the-fly
so data in different CRS still aligns visually.

---

## 5. File Formats Compared

### Vector formats

| Format        | Structure      | Type      | CRS support | Size limit | Readable | Spatial index |
|--------------|----------------|-----------|-------------|------------|----------|---------------|
| **GeoPackage** | SQLite DB     | Vector+Raster | Any, multi | None practical | Binary | RTree built-in |
| **GeoJSON**   | JSON text      | Vector    | WGS84 only  | RAM-bound  | Human    | None          |
| **Shapefile**  | Multi-file    | Vector    | Any         | 2 GB       | Binary   | .shx          |
| **KML/KMZ**   | XML (zipped)  | Vector    | WGS84       | RAM-bound  | Verbose  | None          |
| **CSV + WKT** | Text          | Vector    | Declared    | None       | Human    | None          |
| **PostGIS**    | PostgreSQL DB | Vector    | Any, multi  | None       | SQL      | GiST index    |
| **SpatiaLite** | SQLite DB    | Vector    | Any, multi  | None       | SQL      | RTree         |
| **FlatGeobuf** | Binary        | Vector    | Any         | None       | Binary   | Hilbert R-tree|

### Raster formats

| Format       | Structure | Compression | Georef | Bands | Use case              |
|-------------|-----------|------------|--------|-------|-----------------------|
| **GeoTIFF**  | TIFF+tags | LZW/Deflate| Built-in| Multi | DEM, imagery, analysis|
| **COG**      | GeoTIFF   | Yes        | Yes    | Multi | Cloud-optimized tiles |
| **ASC**      | Text grid | None       | Header | 1     | Simple DEMs           |
| **MBTiles**  | SQLite    | PNG/JPEG   | Yes    | RGB   | Basemap tiles         |

### Geometry encodings (within formats)

| Encoding | Type   | Used in                        | Readable |
|---------|--------|--------------------------------|----------|
| **WKT** | Text   | Databases, CSV, display        | Human    |
| **WKB** | Binary | PostGIS, GeoPackage, protocols | Machine  |
| **GeoJSON geometry** | JSON | GeoJSON files, APIs  | Human    |

**WKT examples** (Well-Known Text):
```
POINT(-123.812 39.361)
LINESTRING(-123.812 39.361, -123.815 39.362, -123.818 39.363)
POLYGON((-123.81 39.36, -123.82 39.36, -123.82 39.37, -123.81 39.37, -123.81 39.36))
```

---

## 6. GeoPackage Deep Dive

GeoPackage is a **SQLite database** following the OGC specification.
You can open it with any SQLite client and query it with SQL.

### Internal table structure

```
┌─────────────────────────────────────────────┐
│              your_data.gpkg                  │
│                                              │
│  System tables (OGC mandated):               │
│  ├── gpkg_contents          (catalog)        │
│  ├── gpkg_spatial_ref_sys   (CRS defs)       │
│  ├── gpkg_geometry_columns  (geom metadata)  │
│  ├── gpkg_tile_matrix_set   (raster tiles)   │
│  ├── gpkg_tile_matrix       (zoom levels)    │
│  ├── gpkg_metadata          (metadata docs)  │
│  ├── gpkg_metadata_reference (links)         │
│  └── gpkg_extensions        (extensions)     │
│                                              │
│  Your data tables:                           │
│  ├── nodes    (id, node_type, elev, geom...) │
│  ├── pipes    (id, diameter, material, geom) │
│  └── rtree_nodes_geom  (spatial index)       │
│                                              │
└─────────────────────────────────────────────┘
```

### Key system tables

**gpkg_contents** — catalog of every data table:
```sql
SELECT table_name, data_type, identifier, srs_id,
       min_x, min_y, max_x, max_y
FROM gpkg_contents;
-- Returns:
-- nodes    | features | Water System Nodes | 4326 | -123.82 | 39.35 | ...
-- pipes    | features | Water System Pipes | 4326 | -123.82 | 39.35 | ...
```

**gpkg_spatial_ref_sys** — coordinate reference systems:
```sql
SELECT srs_id, srs_name, organization, definition
FROM gpkg_spatial_ref_sys WHERE srs_id = 4326;
-- 4326 | WGS 84 | EPSG | GEOGCS["WGS 84",DATUM["WGS_1984",...]]
```

**gpkg_geometry_columns** — geometry metadata per table:
```sql
SELECT table_name, column_name, geometry_type_name, srs_id, z, m
FROM gpkg_geometry_columns;
-- nodes | geom | POINT      | 4326 | 0 | 0
-- pipes | geom | LINESTRING | 4326 | 0 | 0
```

### Spatial indexing

GeoPackage uses SQLite's **RTree module** for spatial indexing.
For a table `nodes` with geometry column `geom`, an index table
`rtree_nodes_geom` is created automatically:

```sql
-- Spatial query: find nodes within a bounding box
SELECT * FROM nodes
WHERE nodes.rowid IN (
  SELECT id FROM rtree_nodes_geom
  WHERE minx <= -123.80 AND maxx >= -123.85
  AND miny <= 39.35 AND maxy >= 39.37
);
```

### Why GeoPackage matters

- **It's just SQLite.** You already know SQLite (watertown uses it
  conceptually, your billing uses CSV but could use SQLite). You can
  query a GeoPackage with the `sqlite3` CLI, Python, Go, Rust, or
  any language with SQLite bindings.
- **One file = one project.** Nodes, pipes, raster tiles, metadata,
  indexes — all in one `.gpkg` file. Copy it, back it up, email it.
- **Schema enforcement.** Unlike GeoJSON (anything goes), you define
  columns with types. Diameter must be REAL, not accidentally text.
- **OGC standard.** Not tied to any vendor. QGIS, ArcGIS, GDAL,
  PostGIS, MapServer, GeoServer all read it.

---

## 7. GeoJSON Deep Dive

### Structure

```json
{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "geometry": {
        "type": "Point",
        "coordinates": [-123.812, 39.361]    // [longitude, latitude]
      },
      "properties": {
        "id": "VALVE-07",
        "type": "gate",
        "diameter_in": 6,
        "status": "open"
      }
    }
  ]
}
```

### Rules and constraints

- **CRS**: Always WGS 84 (EPSG:4326) per RFC 7946. No exceptions.
- **Coordinate order**: `[longitude, latitude]` (not lat/lon!)
- **Properties**: Flat key-value pairs. No nested objects in strict
  implementations (though many tools tolerate nesting).
- **No schema**: Properties can vary between features. No type
  enforcement. A "diameter" could be `6` in one feature and `"six"`
  in another. Careful.
- **No spatial index**: Linear scan for spatial queries. Fine for
  hundreds of features, slow for millions.

### Geometry types in GeoJSON

```json
// Point
{ "type": "Point", "coordinates": [-123.812, 39.361] }

// LineString (pipe route with intermediate vertices)
{ "type": "LineString", "coordinates": [
    [-123.812, 39.361],
    [-123.814, 39.362],
    [-123.816, 39.363]
  ]
}

// Polygon (closed area — first and last coord must match)
{ "type": "Polygon", "coordinates": [[
    [-123.81, 39.36], [-123.82, 39.36],
    [-123.82, 39.37], [-123.81, 39.37],
    [-123.81, 39.36]
  ]]
}
```

### Why GeoJSON matters

- **JavaScript native.** `fetch("network.geojson").then(r => r.json())`
  gives you objects you can iterate immediately. No parsing library.
- **Leaflet/Mapbox/D3 native.** All major web mapping libraries
  accept GeoJSON directly.
- **Git-friendly.** Text-based, diffable, mergeable (mostly).
- **Human-readable.** You can open it in a text editor and
  understand what you're looking at.

---

## 8. EPANET Data Model

EPANET models a water distribution network as a directed graph with
hydraulic properties. The .inp file is a text file with bracketed
sections.

### Conceptual model

```
                    ┌─────────┐
                    │RESERVOIR│ (fixed head source)
                    └────┬────┘
                         │
                    ┌────▼────┐
                    │  PUMP   │ (energy input)
                    └────┬────┘
                         │
                    ┌────▼────┐
                    │  TANK   │ (storage, variable level)
                    └────┬────┘
                         │
              ┌──────────▼──────────┐
              │       PIPE          │ (friction, head loss)
              └──────────┬──────────┘
                         │
            ┌────────────▼────────────┐
            │       JUNCTION          │ (demand node, pressure result)
            ├─────────┬───────────────┤
            │ PIPE    │    PIPE       │
            ▼         ▼               ▼
        JUNCTION  JUNCTION        JUNCTION
        (demand)  (demand)        (demand)
```

### Node types (things at locations)

| Entity     | Key attributes                           | Notes                       |
|-----------|------------------------------------------|-----------------------------|
| JUNCTION   | ID, Elevation, BaseDemand, Pattern       | Where water is consumed     |
| RESERVOIR  | ID, Head, Pattern                        | Infinite source at fixed head|
| TANK       | ID, Elevation, InitLevel, MinLevel,      | Finite storage              |
|            | MaxLevel, Diameter, MinVolume, VolCurve  |                             |

### Link types (things between locations)

| Entity  | Key attributes                            | Notes                       |
|--------|-------------------------------------------|-----------------------------|
| PIPE    | ID, Node1, Node2, Length, Diameter,       | Passive conduit             |
|         | Roughness, MinorLoss, Status              |                             |
| PUMP    | ID, Node1, Node2, CurveID/Parameters     | Energy input                |
| VALVE   | ID, Node1, Node2, Diameter, Type, Setting | PRV, PSV, FCV, TCV, GPV    |

### Operational data

| Section      | What it defines                                  |
|-------------|--------------------------------------------------|
| [DEMANDS]    | Multiple demand categories per junction          |
| [PATTERNS]   | Time-varying multipliers (24-hour cycles)        |
| [CURVES]     | Pump head-capacity, tank volume, efficiency      |
| [CONTROLS]   | Simple IF-THEN rules (tank level triggers)       |
| [RULES]      | Complex conditional logic                        |
| [ENERGY]     | Pump energy costs and efficiency                 |
| [EMITTERS]   | Pressure-dependent outflow (sprinklers, leaks)   |

### Simulation parameters

| Section      | What it defines                                  |
|-------------|--------------------------------------------------|
| [OPTIONS]    | Units (GPM/LPS), headloss formula (H-W/D-W/C-M) |
| [TIMES]      | Duration, timestep, report period                |
| [REPORT]     | What results to output                           |

### Water quality (optional)

| Section      | What it defines                                  |
|-------------|--------------------------------------------------|
| [QUALITY]    | Initial concentrations at nodes                  |
| [REACTIONS]  | Decay/growth rates (bulk, wall)                  |
| [SOURCES]    | Injection points (chlorine dosing)               |
| [MIXING]     | Tank mixing model (complete, FIFO, LIFO, 2-comp) |

### Display/GIS (optional)

| Section        | What it defines                                |
|---------------|------------------------------------------------|
| [COORDINATES]  | X,Y position of each node (for display only)  |
| [VERTICES]     | Intermediate points along pipe routes          |
| [LABELS]       | Text labels on the map                         |
| [BACKDROP]     | Background image bounds                        |

### Complete .inp file structure

```
[TITLE]           Free text description
[JUNCTIONS]       ID  Elevation  Demand  Pattern
[RESERVOIRS]      ID  Head  Pattern
[TANKS]           ID  Elev  InitLvl  MinLvl  MaxLvl  Diam  MinVol  VolCurve
[PIPES]           ID  Node1  Node2  Length  Diameter  Roughness  MinorLoss  Status
[PUMPS]           ID  Node1  Node2  Parameters
[VALVES]          ID  Node1  Node2  Diameter  Type  Setting  MinorLoss
[TAGS]            ElementType  ID  Tag
[DEMANDS]         JunctionID  Demand  Pattern  Category
[STATUS]          ID  Status/Setting
[PATTERNS]        ID  Multipliers...
[CURVES]          ID  X-Value  Y-Value
[CONTROLS]        Simple rule statements
[RULES]           Complex rule blocks
[ENERGY]          Global efficiency, price, demand charge
[EMITTERS]        JunctionID  Coefficient
[QUALITY]         NodeID  InitQual
[SOURCES]         NodeID  Type  Quality  Pattern
[REACTIONS]       Type  Pipe/Tank  Coefficient
[MIXING]          TankID  Model
[TIMES]           Duration, timesteps, report settings
[REPORT]          Status, summary, page settings
[OPTIONS]         Units, headloss formula, accuracy, etc.
[COORDINATES]     NodeID  X-Coord  Y-Coord
[VERTICES]        LinkID  X-Coord  Y-Coord
[LABELS]          X-Coord  Y-Coord  Label
[BACKDROP]        Dimensions, units, file, offset
[END]
```

### Key relationships

```
JUNCTION ◄──── DEMAND (1:many — multiple demand categories)
DEMAND ──────► PATTERN (many:1 — demands reference patterns)
PIPE ─────────► JUNCTION × 2 (each pipe connects two nodes)
PUMP ─────────► CURVE (pump performance curve)
TANK ─────────► CURVE (optional volume curve)
```

---

## 9. Giswater Data Model (Full Utility GIS)

Giswater is the most complete open-source data model for water
utility management. It runs on PostgreSQL/PostGIS and integrates
with QGIS via a plugin. Understanding its schema shows what a
"real" utility data model looks like — even if you don't use
Giswater itself.

### Schema organization

```
giswater database
├── ws_*     Water Supply tables
├── ud_*     Urban Drainage tables
└── cm_*     Common/shared tables
```

### Core asset tables (water supply)

**Nodes** (point features — things at locations):
```sql
ws_node (
    node_id     TEXT PRIMARY KEY,
    node_type   TEXT,       -- 'junction', 'tank', 'reservoir',
                            -- 'valve', 'hydrant', 'meter', 'pump'
    elevation   NUMERIC,
    depth       NUMERIC,
    state       INTEGER,    -- 0=obsolete, 1=active, 2=planned
    state_type  TEXT,       -- 'in_service', 'not_in_service'
    the_geom    GEOMETRY(Point, SRID)
)
```

**Arcs** (line features — things between locations):
```sql
ws_arc (
    arc_id      TEXT PRIMARY KEY,
    arc_type    TEXT,       -- 'pipe', 'varc' (virtual arc)
    node_1      TEXT,       -- upstream node
    node_2      TEXT,       -- downstream node
    diameter    NUMERIC,
    material    TEXT,
    length      NUMERIC,
    roughness   NUMERIC,
    state       INTEGER,
    the_geom    GEOMETRY(LineString, SRID)
)
```

### Catalog tables (lookups)

```sql
ws_cat_material   (id, description, roughness, ...)
ws_cat_pipe       (id, material, diameter, thickness, ...)
ws_cat_valve      (id, type, diameter, ...)
ws_cat_node       (id, node_type, description, ...)
```

### Operational tables

```sql
ws_document  (id, doc_type, path, observ, feature_id, ...)
ws_event     (id, event_type, feature_id, tstamp, value, text, ...)
ws_visit     (id, feature_id, startdate, enddate, user, ...)
ws_element   (id, element_type, serial, brand, model, ...)
```

### EPANET integration tables

```sql
ws_inp_junction  (node_id, demand, pattern_id, ...)
ws_inp_pipe      (arc_id, minorloss, status, ...)
ws_inp_tank      (node_id, initlevel, minlevel, maxlevel, ...)
ws_inp_pump      (node_id, curve_id, speed, ...)
ws_inp_pattern   (pattern_id, factor_1..factor_24)
ws_inp_curve     (curve_id, x_value, y_value)
```

### What this teaches us

The Giswater model separates:
1. **Physical reality** (where is the pipe, what's it made of)
2. **Operational state** (is it active, planned, or decommissioned)
3. **Simulation parameters** (roughness, demand patterns for EPANET)
4. **Maintenance history** (events, visits, documents)
5. **Catalog/reference data** (standard materials, pipe specs)

A small system doesn't need all of this. But understanding the
layers helps you decide what to model now and what to add later.

### Minimum viable data model for a small system

Start here:

```
nodes (GeoPackage layer, Point geometry)
├── id              TEXT    -- unique node ID
├── node_type       TEXT    -- JUNCTION, TANK, RESERVOIR
├── elevation_ft    REAL    -- ground elevation
├── base_demand_gpm REAL    -- normal water use
├── static_psi      REAL    -- measured static pressure
├── notes           TEXT    -- address, description
└── geom            POINT   -- location

pipes (GeoPackage layer, LineString geometry)
├── id              TEXT    -- unique pipe ID
├── diameter_in     REAL    -- internal diameter
├── material        TEXT    -- PVC, steel, poly, etc.
├── roughness       REAL    -- H-W coefficient (auto from material if blank)
├── install_year    INTEGER -- year installed (if known)
├── notes           TEXT    -- description
└── geom            LINESTRING -- route
```

This is enough for EPANET simulation, map display, and basic
asset tracking. Add maintenance/event tables when needed.

---

## 10. QGIS Plugin Ecosystem for Water

### Network modeling & simulation

| Plugin       | Backend       | Capabilities                          |
|-------------|---------------|---------------------------------------|
| **Gusnet**   | EPANET + WNTR | Draw network, simulate, visualize results. Exports .inp |
| **QGISRed**  | EPANET        | Digital twin, calibration, scenario analysis |
| **GHydraulics** | EPANET     | Basic simulation, economic diameter calc |
| **QWater**   | EPANET        | Pressure-dependent demand analysis    |
| **Sketcher** | WNTR          | Quick network sketching               |

### Full utility management

| Plugin       | Backend        | Capabilities                         |
|-------------|----------------|--------------------------------------|
| **Giswater** | PostgreSQL/PostGIS | Complete asset mgmt, maintenance, EPANET/SWMM integration |

### Data & terrain

| Plugin                | Capabilities                              |
|----------------------|-------------------------------------------|
| **QuickMapServices**  | Basemaps: Google, Bing, OSM satellite     |
| **SRTM Downloader**   | One-click elevation data download         |
| **Point Sampling Tool**| Extract raster values at point locations  |
| **Profile Tool**       | Elevation profiles along a line          |


### Field & mobile

| Plugin            | Capabilities                               |
|------------------|--------------------------------------------|
| **QField Sync**   | Package project for mobile QField app      |
| **QFieldCloud**   | Multi-device sync, conflict resolution     |
| **Input/Sketcher**| Alternative mobile collectors              |

### Data quality

| Plugin | Capabilities |
|--------|-------------|
| **Topology Checker** | Validate network topology (gaps, overlaps, dangles) |
| **Geometry Checker** | Check geometry validity (self-intersections, duplicates) |
| **Network Analysis** | Graph-based connectivity analysis (tracing, isolation) |
---

## 11. QField: Mobile Data Collection

QField is the official mobile companion to QGIS. It runs on
Android and iOS, providing field data collection with full GIS
capabilities.

### Workflow

```
QGIS (desktop)
    │
    ├── QField Sync plugin: package project for mobile
    │
    ▼
QField (mobile device)
    │
    ├── Offline GPS-located data collection
    ├── Photo attachments
    ├── Form-based attribute entry
    ├── Edit existing features
    │
    ▼
Sync back to QGIS
    │
    ├── Manual: copy .gpkg back via USB/cloud
    └── Automatic: QFieldCloud (conflict resolution)
```

### Water utility field use cases

| Task                     | What QField does                          |
|-------------------------|-------------------------------------------|
| Valve inspection         | Navigate to valve, log status + photo     |
| Hydrant flushing         | Record date, duration, residual chlorine  |
| Leak documentation       | GPS-locate leak, log type and severity    |
| New connection           | Add service line, meter, curb stop        |
| Pipe marking             | Walk pipe route with GPS tracking         |
| Meter reading            | Navigate to meters, log readings          |

### Data format

QField works with GeoPackage natively. The mobile device gets a
copy of the .gpkg file. Edits are made locally (works offline),
then synced back. QFieldCloud handles multi-user conflict resolution.

---

## 12. How Everything Connects

### The complete data model map

```
┌─────────────────────────────────────────────────────────────┐
│                    SPATIAL DATA                              │
│                                                              │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐               │
│  │  Points  │    │  Lines   │    │ Polygons │               │
│  │ (nodes)  │    │ (pipes)  │    │ (areas)  │               │
│  └────┬─────┘    └────┬─────┘    └────┬─────┘               │
│       │               │               │                      │
│       └───────────────┼───────────────┘                      │
│                       │                                      │
│              ┌────────▼─────────┐                            │
│              │   GeoPackage     │  Canonical storage          │
│              │   (.gpkg)        │  SQLite + RTree index       │
│              └────────┬─────────┘                            │
│                       │                                      │
│         ┌─────────────┼──────────────┐                       │
│         │             │              │                        │
│    ┌────▼────┐   ┌────▼────┐   ┌────▼────┐                  │
│    │ GeoJSON │   │  .inp   │   │  QField │                   │
│    │ (web)   │   │(EPANET) │   │(mobile) │                   │
│    └────┬────┘   └────┬────┘   └────┬────┘                   │
│         │             │              │                        │
│         ▼             ▼              ▼                        │
│    Leaflet map   epanet-js     Field data                    │
│    on website    simulation    collection                     │
│                                                              │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                    RASTER DATA                               │
│                                                              │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐               │
│  │   DEM    │    │ Imagery  │    │ Hillshade│               │
│  │(GeoTIFF) │    │(basemap) │    │(derived) │               │
│  └────┬─────┘    └────┬─────┘    └────┬─────┘               │
│       │               │               │                      │
│       ▼               ▼               ▼                      │
│  Elevation        Visual          Terrain                    │
│  extraction       context         analysis                   │
│  at junctions     in QGIS/web     slope/aspect               │
│                                                              │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                  ATTRIBUTE DATA                              │
│                                                              │
│  Per Node:                    Per Pipe:                      │
│  ├── id                       ├── id                         │
│  ├── node_type                ├── diameter_in                │
│  ├── elevation_ft             ├── material                   │
│  ├── base_demand_gpm          ├── roughness                  │
│  ├── static_psi               ├── install_year               │
│  └── notes                    └── notes                      │
│                                                              │
│  Per Event (future):          Per Document (future):         │
│  ├── event_type               ├── doc_type                   │
│  ├── feature_id               ├── feature_id                 │
│  ├── timestamp                ├── file_path                  │
│  ├── value                    └── notes                      │
│  └── notes                                                   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Format conversion paths

```
GeoPackage ──► GeoJSON         ogr2ogr or QGIS export
GeoPackage ──► Shapefile       ogr2ogr or QGIS export
GeoPackage ──► EPANET .inp     Gusnet plugin or custom script
GeoJSON ────► GeoPackage       ogr2ogr or QGIS import
KML ────────► GeoJSON          ogr2ogr, geojson.io
KML ────────► GeoPackage       ogr2ogr or QGIS import
EPANET .inp ► GeoJSON          WNTR (Python) or custom script
Shapefile ──► GeoPackage       ogr2ogr or QGIS import
PostGIS ────► GeoPackage       ogr2ogr or QGIS DB Manager
```

The conversion tool **ogr2ogr** (part of GDAL) handles all of
these. Install with `brew install gdal` (macOS).

---

## 13. Mapping to the Caspar Platform

### Where each component fits

| Platform layer   | GIS role                              | Format           |
|-----------------|---------------------------------------|------------------|
| **supruglue**    | Sensor locations (meters, sensors)    | Attributes (lat/lon in config) |
| **caspar.water** | OTel collector, pressure monitoring   | Metrics (OTel → InfluxDB) |
|                  | Website hosting                       | Hugo static site  |
| **watertown**     | Time-series storage & analysis        | Arrow/Parquet     |
|                  | Site generation (Observable)          | Markdown + data   |
| **QGIS**         | Network authoring & management        | GeoPackage        |
| **Website**      | Interactive hydraulic model           | GeoJSON + .inp    |
| **QField**       | Field inspections (future)            | GeoPackage sync   |

### Data flow across the platform

```
Physical system
    │
    ├── supruglue (PRU) ──► meter readings, pump status
    │                           │
    │                           ▼
    ├── caspar.water (OTel) ──► pressure, pH, flow metrics
    │                           │
    │                           ▼
    ├── watertown ──► time-series storage ──► Observable charts
    │
    │
    ├── QGIS + GeoPackage ──► network model (static/structural)
    │       │
    │       ├──► GeoJSON ──► website map
    │       └──► .inp ──► epanet-js simulation
    │
    └── QField ──► field data ──► sync to GeoPackage
```

### The two kinds of data

1. **Structural/spatial** (changes rarely): pipe routes, diameters,
   materials, junction locations, valve positions. Lives in
   GeoPackage, exported to GeoJSON and .inp.

2. **Operational/temporal** (changes constantly): pressure readings,
   flow rates, tank levels, chlorine residual. Lives in watertown
   (Arrow/Parquet), visualized via Observable.

The hydraulic model bridges both: it uses structural data (pipe
network) to predict operational data (pressure under load). When
real operational data is available (from the OTel pipeline), it
calibrates and validates the structural model.

---

## Appendix: Key Resources

### Standards
- OGC GeoPackage: https://www.geopackage.org/spec/
- GeoJSON RFC 7946: https://tools.ietf.org/html/rfc7946
- EPANET 2.2 User Manual: https://www.epa.gov/water-research/epanet
- OGC Simple Features: https://www.ogc.org/standard/sfa/

### Software
- QGIS: https://qgis.org/download/
- QField: https://qfield.org/
- GDAL/ogr2ogr: https://gdal.org/
- epanet-js: https://github.com/epanet-js/epanet-js-toolkit
- Giswater: https://github.com/Giswater
- Gusnet: https://www.gusnet.org/

### Data
- USGS National Map (DEM): https://apps.nationalmap.gov/downloader/
- OpenTopography (LiDAR): https://opentopography.org/
- EPSG registry: https://epsg.io/
