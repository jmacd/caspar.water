# Practical Guide: From Your Head to a Working Hydraulic Model

*For the water operator who knows where the pipes are.*

---

## What This Guide Covers

You know your system. You know where the pipes run, roughly how
big they are, where the tank sits, what the pressure gauge reads.
This guide gets that knowledge out of your head and into structured
data that can:

- Display your pipe network on a map
- Run hydraulic simulations (pressure under load)
- Feed your website's interactive pressure explorer
- Serve as the beginning of a proper asset inventory

**Time estimate**: One focused afternoon to get the initial network
digitized. Ongoing refinement as you learn more.

**Cost**: $0. Every tool in this guide is free and open-source.

---

## Part 1: What You Already Have (Digitally)

Before entering anything new, take stock. Your OTel pipeline
already captures these metrics continuously:

### Pumphouse station (wellkit/wellprobe)

| Metric                 | Source        | Unit | What it measures              |
|------------------------|---------------|------|-------------------------------|
| `wellprobe_pressure`   | Modbus probe  | PSI  | Well water level (as head)    |
| `wellprobe_temperature`| Modbus probe  | °C   | Groundwater temperature       |
| `wellkit_temperature`  | BME280 sensor | °F   | Pumphouse air temperature     |
| `wellkit_pressure`     | BME280 sensor | PSI  | Barometric pressure           |
| `wellkit_humidity`     | BME280 sensor | %    | Pumphouse humidity            |

### Gateway station (Sparkplug/MQTT)

| Metric                       | Unit | What it measures              |
|------------------------------|------|-------------------------------|
| `well_depth_value`           |      | Well water depth              |
| `system_pressure_value`      | PSI  | Distribution system pressure  |
| `chlorine_level_value`       | mg/L | Chlorine residual             |
| `concrete_tank_level_value`  |      | Storage tank water level      |

### Pressure display (presskit)

| Metric           | Source       | Unit | What it measures             |
|------------------|-------------|------|------------------------------|
| `water_pressure` | Current loop| PSI  | Dynamic system pressure      |

### Billing data (CSV files)

| File          | Contents                                        |
|---------------|-------------------------------------------------|
| `users.csv`   | Account name, service address, billing address  |
| `business.csv`| Company name, address, contact                  |
| `cycles.csv`  | Billing periods, costs, connections count       |
| `payments.csv`| Payment date, account, amount                   |

**Key point**: `system_pressure_value` is your calibration anchor.
The hydraulic model should predict a pressure at that gauge location
that matches what the sensor actually reads. If it doesn't, adjust
roughness coefficients until it does.

---

## Part 2: What You Need to Enter

The hydraulic model needs information that no sensor captures —
the physical layout of the distribution network.

### Data you need to gather

**For every connection (junction):**

| Field          | How to get it                                   |
|----------------|------------------------------------------------|
| Location       | Stand there with your phone → lat/lon          |
| Elevation      | QGIS extracts this from USGS DEM automatically |
| Static pressure| Read the gauge, or estimate from elevation     |
| Normal demand  | Billing records ÷ billing period → avg GPM     |
| Notes          | Address, account name, anything memorable      |

**For every pipe segment:**

| Field     | How to get it                                      |
|-----------|----------------------------------------------------|
| Route     | Draw it on the satellite image in QGIS             |
| Diameter  | You know this — write it down before you forget    |
| Material  | PVC, poly, galvanized, etc. (determines roughness) |
| Length    | QGIS calculates from the line you draw             |

**For the tank:**

| Field            | You already know this | Already in data? |
|------------------|-----------------------|-------------------|
| Base elevation   | 1225 ft               | fireflow.epanet   |
| Diameter         | 15 ft                 | fireflow.epanet   |
| Max level        | 8 ft                  | fireflow.epanet   |
| Current level    | Yes                   | `concrete_tank_level_value` |

**For pipe branching points (tees, elbows, reducers):**

Same as junctions, but with zero demand. These are points where
the pipe network splits or changes diameter. Mark them if the pipe
changes size or direction significantly.

---

## Part 3: Install QGIS

### Download

Go to https://qgis.org/download/ and install the **Long Term
Release (LTS)** — currently 3.34.x.

| Platform | Install method                                      |
|----------|-----------------------------------------------------|
| **macOS**  | Download .dmg from qgis.org, drag to Applications |
| **Windows**| Download .msi installer, run it                    |
| **Linux**  | `sudo apt install qgis qgis-plugin-grass` (Ubuntu/Debian) |

### First-time setup

1. Launch QGIS
2. Install plugins: `Plugins → Manage and Install Plugins`
   Search and install each of these:
   - **QuickMapServices** — satellite basemap imagery
   - **Gusnet** — EPANET hydraulic simulation inside QGIS
   - **Point Sampling Tool** — extract DEM elevation at points

3. Add a satellite basemap:
   `Web → QuickMapServices → Google → Google Satellite`

   If Google isn't listed: `Web → QuickMapServices → Settings →
   More services tab → Get contributed pack`

4. Navigate to your water system area on the map

---

## Part 4: Create Your GeoPackage

A GeoPackage is a single file that holds all your spatial data.
You'll create one with two layers: nodes and pipes.

### Create the file

1. `Layer → Create Layer → New GeoPackage Layer`
2. **Database**: Browse to `model/caspar_network.gpkg`
   (or wherever you want it)
3. **Table name**: `nodes`
4. **Geometry type**: `Point`
5. **CRS**: `EPSG:4326 - WGS 84`

### Add node attribute fields

Click "Add to Fields List" for each:

| Name             | Type     | Length | Purpose                       |
|------------------|----------|--------|-------------------------------|
| `node_id`        | Text     | 30     | Unique ID (TANK-01, J-01)     |
| `node_type`      | Text     | 15     | JUNCTION, TANK, or RESERVOIR  |
| `elevation_ft`   | Decimal  | 10,2   | Ground elevation in feet      |
| `demand_gpm`     | Decimal  | 10,2   | Base demand in GPM            |
| `static_psi`     | Decimal  | 10,2   | Measured static pressure      |
| `account`        | Text     | 50     | Billing account name          |
| `address`        | Text     | 100    | Service address               |
| `notes`          | Text     | 200    | Any other information         |

Click **OK** to create the layer.

### Create the pipes layer

1. `Layer → Create Layer → New GeoPackage Layer`
2. **Database**: Same `.gpkg` file (it adds a layer, not a new file)
3. **Table name**: `pipes`
4. **Geometry type**: `LineString`
5. **CRS**: `EPSG:4326 - WGS 84`

### Add pipe attribute fields

| Name            | Type     | Length | Purpose                       |
|-----------------|----------|--------|-------------------------------|
| `pipe_id`       | Text     | 30     | Unique ID (P-01, MAIN-01)     |
| `diameter_in`   | Decimal  | 10,2   | Internal diameter in inches   |
| `material`      | Text     | 20     | PVC, poly, galv, steel, etc.  |
| `roughness`     | Decimal  | 10,1   | H-W coefficient (see table)   |
| `install_year`  | Integer  |        | Year installed (if known)     |
| `notes`         | Text     | 200    | Description                   |

Click **OK**.

You now have a GeoPackage at `model/caspar_network.gpkg` with two
empty layers visible in the Layers panel.

---

## Part 5: Enter Your Data

### Enable snapping

**This is critical.** Snapping ensures pipes connect precisely to
nodes — no gaps, no overlaps. Without it, the hydraulic model
breaks.

`Project` menu, then choose the option labeled "Snapping Options" (or press the magnet icon on the toolbar)

- Enable for **All Layers**
- Mode: **Vertex**
- Tolerance: **10 pixels**
- Check **Intersection Snapping**
  (this automatically splits pipes where they cross)

This guarantees your network is topologically connected —
pipe endpoints will jump to nearby junction points when you draw.

### Enter nodes first

1. Select the `nodes` layer in the Layers panel
2. Click the pencil icon (Toggle Editing)
3. Click "Add Point Feature" in the Digitizing toolbar
4. Click on the satellite image where each feature is located
5. Fill in the attribute form that appears:
   - `node_id`: A short unique name (TANK-01, J-01, J-02...)
   - `node_type`: JUNCTION for connections, TANK for the tank
   - `elevation_ft`: Leave blank for now — we'll extract from DEM
   - `demand_gpm`: Normal household use, typically 1-3 GPM
   - `static_psi`: If you've measured it
   - `account`: Match to users.csv account name
   - `address`: Service address
6. Click OK
7. Repeat for every node

**Order of entry:**
```
1. Storage tank(s)
2. Well/reservoir (if separate from tank)
3. Main line branch points (tees, reducers)
4. Service connections (one per customer)
```

### Enter pipes

1. Select the `pipes` layer
2. Toggle Editing on
3. Click "Add Line Feature"
4. Click along the pipe route on the satellite image:
   - Start at one node
   - Click intermediate points to follow the road/path
   - End at the next node (it should snap to the junction)
   - Right-click to finish the line
5. Fill in the form:
   - `pipe_id`: P-01, MAIN-01, SVC-03, etc.
   - `diameter_in`: The pipe diameter
   - `material`: What it's made of
   - `roughness`: Use the table below, or leave blank
6. Repeat for every pipe segment

### Roughness reference

| Material           | Hazen-Williams C | Notes              |
|-------------------|------------------|--------------------|
| PVC (new)          | 150              | Most modern pipe   |
| PVC (20+ years)    | 140              | Slight aging       |
| HDPE / Poly        | 140              | Flexible, rural    |
| Ductile iron (new) | 130              |                    |
| Ductile iron (old) | 100              | Tuberculated       |
| Galvanized steel   | 120              | Service lines      |
| Copper             | 135              | Service lines      |
| Cast iron (old)    | 80–100           | Pre-1960           |

### Save your work

Click the pencil icon again → "Save" when prompted.

**Save early, save often.** The GeoPackage is a single file — back
it up.

---

## Part 6: Get Elevations from DEM

You don't need to survey every junction with a transit. Free
elevation data from USGS is accurate to ~3 meters vertically,
which is close enough for initial modeling.

### Download elevation data

1. Go to https://apps.nationalmap.gov/downloader/
2. Search for your area (Caspar, CA)
3. Select product: **Elevation Products (3DEP)**
4. Select sub-type: **1/3 arc-second DEM**
5. Download the GeoTIFF covering your area

### Load into QGIS

1. `Layer → Add Layer → Add Raster Layer`
2. Browse to the downloaded `.tif` file
3. Click Add — the elevation grid appears under your vector data

### Extract elevations at junctions

1. Open `Processing` menu → `Toolbox`
2. Search for "Sample raster values"
3. Input point layer: your `nodes` layer
4. Raster layer: the DEM you loaded
5. Output column prefix: `elevation_ft`
6. Run it

This populates every junction's elevation from the terrain data.
Check the attribute table — you should see realistic elevations.

### Verify with known pressures

If you know static pressure at a point and the tank elevation:

```
expected_elevation = tank_elevation - (static_psi / 0.433)
```

Compare this to what the DEM says. If they're close (within 10-20
feet), the DEM data is good enough. If they differ significantly,
the measured pressure is more reliable — override the DEM value.

---

## Part 7: Connect to the Hydraulic Model

### Export GeoJSON (for the website)

1. Right-click the `nodes` layer → `Export → Save Features As`
2. Format: GeoJSON
3. File name: `model/caspar_nodes.geojson`
4. CRS: EPSG:4326
5. Click OK
6. Repeat for `pipes` → `model/caspar_pipes.geojson`

Or export both layers at once to a single file if your workflow
prefers it.

### Generate EPANET .inp (for simulation)

A conversion script reads the GeoJSON and writes the .inp file.
This script will be provided in the repository as
`model/geojson_to_inp.js` (or `.go` or `.py`).

What it does:
- Reads nodes → writes [JUNCTIONS], [TANKS], [RESERVOIRS] sections
- Reads pipes → writes [PIPES] section
- Calculates pipe length from LineString coordinates (Haversine)
- Fills roughness from material if not explicitly set
- Sets [OPTIONS] for GPM units and Hazen-Williams headloss
- Writes [COORDINATES] from the GeoJSON point locations

### Or use Gusnet (EPANET inside QGIS)

If you installed the Gusnet plugin, you can skip the export step:

1. Use Gusnet to tag your layers as EPANET components
2. Run the simulation directly inside QGIS
3. View pressure results as colored overlays on the map
4. Export the .inp file from Gusnet for use on the website

This is the more powerful path, but has a steeper learning curve.

---

## Part 8: Validate the Model

### What "validation" means

Run the EPANET model and compare predicted pressure to what you
actually measure. Your `system_pressure_value` metric from the
OTel pipeline is the reference.

### Quick check

1. Load the .inp file in EPANET (desktop app) or epanet-js
2. Run the hydraulic solver
3. Look at pressure at the junction nearest your pressure gauge
4. Compare to `system_pressure_value` at a time of low demand
   (e.g., 3 AM — this approximates static conditions)

### If pressure is too high in the model

- Your elevation estimates may be too low
- Increase junction elevation(s)

### If pressure is too low in the model

- Your elevation estimates may be too high
- Or your roughness values are too aggressive (too much friction)
- Decrease roughness (increase H-W C value) or reduce elevation

### Calibration

Calibration is iterative. Adjust roughness coefficients until the
model matches reality at your measurement points. For a small
system with one pressure gauge, this is trial-and-error. With more
gauges, you can triangulate which pipe segments have higher friction
than assumed.

The duckpond time-series data gives you pressure at different
demand levels (daytime vs. nighttime, weekday vs. weekend). A
calibrated model should match across these conditions, not just
at one point in time.

---

## Part 9: Ongoing Data Management

The GeoPackage is a living document. Here's what triggers updates:

### When infrastructure changes

| Event                        | What to update                    |
|------------------------------|-----------------------------------|
| New service connection       | Add junction + service pipe       |
| Pipe replacement/repair      | Update diameter, material, year   |
| Valve installed/removed      | Add/remove node                   |
| Meter replaced               | Update notes                      |
| New well or pump             | Add reservoir/pump node           |

### When you learn something new

| Discovery                    | What to update                    |
|------------------------------|-----------------------------------|
| Actual pipe diameter found   | Update `diameter_in`              |
| Pipe material confirmed      | Update `material` + `roughness`   |
| Better elevation (survey)    | Override DEM value                |
| Pressure reading at new point| Add `static_psi`, recalibrate     |

### Periodic tasks

| Task                         | Frequency | Purpose                  |
|------------------------------|-----------|--------------------------|
| Export GeoJSON for website   | After changes | Keep web map current  |
| Regenerate .inp              | After changes | Keep simulation current|
| Compare model to real data   | Quarterly | Catch model drift        |
| Back up GeoPackage           | Weekly    | Data protection           |
| QField inspection rounds     | Seasonal  | Valve exercise, leak check|

---

## Part 10: File Summary

After completing this guide, your `model/` directory contains:

```
model/
├── caspar_network.gpkg    # GeoPackage — your canonical GIS data
│   ├── nodes layer        #   Junctions, tanks, reservoirs
│   └── pipes layer        #   All pipe segments
├── caspar_nodes.geojson   # Exported for the website map
├── caspar_pipes.geojson   # Exported for the website map
├── caspar.inp             # Generated EPANET model
├── fireflow.epanet        # Original reference model
├── GUIDEBOOK.md           # GIS format & architecture guide
├── POSITION_PAPER.md      # Project vision document
├── QGIS_DATA_MODELS.md    # Comprehensive data model reference
└── geojson_to_inp.js      # Conversion script (future)
```

The GeoPackage is the source of truth. Everything else is derived
from it. When you change pipe data, re-export GeoJSON and
regenerate the .inp file.

---

## Part 11: Your Measurement Points and the Model

Here's how your existing telemetry connects to EPANET nodes:

| Sensor metric              | EPANET equivalent              | Usage                         |
|---------------------------|-------------------------------|-------------------------------|
| `system_pressure_value`    | Pressure at a junction         | Calibration reference         |
| `concrete_tank_level_value`| Tank InitLevel / level result  | Initial conditions + validation|
| `well_depth_value`         | Reservoir head                 | Source boundary condition     |
| `water_pressure`           | Pressure at presskit location  | Second calibration point      |
| `chlorine_level_value`     | [QUALITY] initial value        | Water quality modeling        |

When you place nodes in QGIS, mark which junction corresponds to
each sensor. Put the sensor metric name in the `notes` field. This
is the bridge between the spatial model and the time-series data.

---

## Part 12: Quick Reference Card

### QGIS quick reference

| Action                | How                            |
|-----------------------|--------------------------------|
| Pan map               | Hold spacebar + drag           |
| Zoom in/out           | Scroll wheel                   |
| Start editing         | Click pencil icon on toolbar   |
| Add point             | Click "Add Point Feature" icon |
| Add line              | Click "Add Line Feature" icon  |
| Finish line           | Right-click                    |
| Undo last vertex      | Backspace while drawing        |
| Save edits            | Ctrl+S (Cmd+S on Mac)          |
| Open attribute table  | Right-click layer → Open Attribute Table |
| Enable snapping      | Magnet icon or Project → Snapping Options |

### Roughness quick lookup

```
PVC:  150    Poly: 140    Galv: 120    Steel-new: 130
Copper: 135  DI-new: 130  DI-old: 100  Cast-iron: 90
```

### Pressure-elevation conversion

```
PSI = elevation_difference × 0.433
elevation_difference = PSI ÷ 0.433

50 PSI ≈ 115 ft of head
35 PSI ≈  81 ft of head
60 PSI ≈ 139 ft of head
```
