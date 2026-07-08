# An Open-Source Operating System for Small Water Utilities

*Position paper — Joshua MacDonald, Caspar Water System*
*March 2026*

---

## The Problem

Small water systems serve millions of Americans, yet the software
tools available to operate them are either expensive, proprietary,
and designed for large utilities — or they don't exist at all.

A system with 12 connections and a chlorinator in the woods has the
same fundamental obligations as a system with 12,000: monitor water
quality, maintain pressure, read meters, send bills, file regulatory
reports, plan for infrastructure, and keep records. The commercial
vendors who serve this market charge per-meter, per-year license
fees for closed systems that lock operators into upgrade cycles and
vendor dependency. The alternative is spreadsheets, paper maps, and
institutional memory that walks out the door when an operator
retires.

There is no open-source project that covers the full scope of
operating a small water system. The pieces exist in isolation —
SCADA systems that don't do billing, billing systems that don't
know where the pipes are, GIS tools that can't read a meter, meter
reading tools that can't generate a regulatory report. Nobody has
assembled them into a coherent whole.

This paper describes a project to do that.

---

## What Exists Today

### Open-source SCADA

Several open-source SCADA platforms are in production use at water
utilities worldwide:

- **ScadaBR** — Java-based, used by the Brazilian utility CASAN for
  flow monitoring and leak detection. The most proven in water.
- **Scada-LTS** — A modernized fork with REST APIs and Docker
  deployment. Designed for water and wastewater.
- **Rapid SCADA** — .NET-based, unlimited data points, plugin
  ecosystem. AGPLv3.
- **OpenSCADA** — C++/Linux, highly modular, European community.

These are data acquisition and visualization layers — they show an
operator what a pump is doing right now. They do not manage assets,
read meters, generate bills, file reports, model hydraulics, or
maintain a GIS. They are one layer of a seven-layer problem.

### Open-source GIS

QGIS is a mature, well-funded open-source GIS with 22 million
launches per month. Its plugin ecosystem includes several tools
purpose-built for water utilities:

- **Gusnet** — EPANET hydraulic simulation inside QGIS
- **Giswater** — Full water utility asset management on
  PostgreSQL/PostGIS
- **QField** — Mobile companion for field data collection

QGIS and its plugins solve the spatial data problem — where are
the pipes, what are they made of, how do they connect. But they
don't collect telemetry, read meters, or generate bills.

### Open-source meter reading

**rtlamr** is a Go program that uses a $25 RTL-SDR dongle to
decode 900 MHz meter transmissions from Itron ERT and Neptune R900
meters — the same protocol used by commercial drive-by AMR systems
costing tens of thousands of dollars. It works today, it's written
in Go, and it outputs structured data that can be piped to MQTT or
any message broker.

Sensus FlexNet meters use a proprietary protocol with no open
decoder. Meter selection matters.

### Regulatory reporting

California's Division of Drinking Water requires an Electronic
Annual Report (eAR) covering system details, connections, sources,
water quality, customer charges, conservation, and more. The eAR
is submitted through a web portal with manual data entry or bulk
Excel upload. No API exists. No open-source tool generates the
required data in the required format.

Every state has analogous requirements. None have open tooling.

### Billing

No serious open-source water utility billing system exists. Generic
invoicing tools (Odoo, InvoicePlane) can be adapted but don't
understand metered water billing, tiered rate structures, or the
specific reporting a water utility needs.

### Hydraulic modeling

EPANET, developed by the EPA, is the industry standard hydraulic
solver. It's public domain C code, over 30 years old, still
definitive. **epanet-js** compiles it to WebAssembly for browser
use, enabling interactive hydraulic simulation with no server
infrastructure. A customer can see what happens to their neighbor's
pressure when they leave a garden hose running.

---

## What We Have Built

The Caspar Water project is an Apache-2.0 licensed platform that
has been operating a real water system in Mendocino County,
California since 2022. It currently spans three repositories:

**supruglue** — Real-time hardware I/O on the TI PRU (BeagleBone
Black). Cooperative threading, microsecond timing, Modbus, I²C.
Reads water meters via the UI1203 protocol. Controls dosing pumps
via PWM. The industrial control layer that replaces a PLC — built
from a $55 single-board computer.

**caspar.water** — Telemetry and operations, built on OpenTelemetry.
A custom OTel Collector with receivers for Modbus, current-loop
sensors, Atlas Scientific pH, BME280 temperature/humidity, and MQTT
Sparkplug-B. Exporters for InfluxDB, serial LCD displays, and LED
matrices. A billing system that processes customer accounts and
generates invoices. A Hugo-based public website. Terraform
definitions for cloud and station infrastructure.

**watertown** — A "very small data lake" in Rust. ACID-transactional
time-series storage on Apache Arrow and Delta Lake. SQL-based data
transformations. A static site generator that produces Observable
Framework pages with DuckDB WASM — interactive data visualization
that runs entirely in the browser with no backend server.

Together these cover real-time I/O, telemetry collection, data
storage, analysis, visualization, billing, and public-facing web
presence. They are in daily production use.

---

## What Remains

The full vision — a system that a neighboring water utility could
adopt wholesale — requires filling specific gaps:

### Layer 1: Drive-by meter reading

Integrate rtlamr with the OTel collector. A truck with a Raspberry
Pi and an RTL-SDR dongle drives the service area. Meter readings
flow through MQTT to the collector, into watertown, and out to
billing. The hardware cost is under $100. The supruglue UI1203
reader handles wired meter interfaces; rtlamr handles wireless.
Together they cover both deployment models.

### Layer 4: GIS and asset management

Adopt QGIS with GeoPackage as the canonical spatial data store for
pipe networks, valve locations, meter positions, and service
boundaries. Export GeoJSON for the website, EPANET .inp files for
hydraulic simulation. Use QField for mobile field data collection —
valve inspections, leak documentation, meter installation. The data
model starts simple (nodes, pipes, attributes) and grows toward the
Giswater schema as operational needs demand.

### Layer 5: Hydraulic modeling

Load the operator's EPANET model in the browser via epanet-js.
Overlay it on a Leaflet map fed by the same GeoJSON. Let customers
and board members toggle demands at junctions and see pressure
response in real time. Calibrate the model against actual pressure
readings flowing through the OTel pipeline. The simulation becomes
a planning tool — what happens if we upsize this pipe, add this
connection, lose this well.

### Layer 6: Metered billing from AMR data

Connect the meter reading pipeline to the existing billing engine.
Tiered rate structures, minimum charges, overage calculations.
Generate PDF invoices and payment records. This is bookkeeping, not
computer science — but it's bookkeeping that every system needs and
no open tool provides.

### Layer 7: Regulatory reporting

Generate the California eAR from operational data already in the
system — connections, sources, water quality results, customer
charges, conservation metrics. The data exists in watertown; the
task is formatting it for the state's submission portal. Each state
has its own requirements, but the underlying data model is broadly
similar. Start with California, abstract later.

---

## Architecture Principles

**Use open standards, not open-source reimplementations.**
EPANET is the hydraulic standard. GeoPackage is the OGC spatial
standard. Arrow and Parquet are the columnar data standards.
OpenTelemetry is the telemetry standard. We don't rewrite these —
we connect them.

**Run in the browser, not on a server.**
DuckDB WASM queries Parquet files client-side. epanet-js runs
hydraulic simulations client-side. The website is static files on
a CDN. An operator with a $5/month hosting bill gets the same
analytical capabilities as a utility district with an IT department.

**Start with one system, design for all.**
Caspar has 12 connections. The Caspar Community Services District,
if formed, might serve 200. The architecture must not assume scale
in either direction. A GeoPackage with 15 nodes and a GeoPackage
with 5,000 nodes are the same format and the same tools.

**Hardware should be cheap and replaceable.**
BeagleBone Black: $55. RTL-SDR dongle: $25. pH sensor: $200.
Pressure transducer: $50. The entire field instrumentation package
for a small system costs less than one year's license fee for
commercial SCADA software.

**The operator should not need to be a programmer.**
QGIS has a learning curve, but it is a GUI application with
documentation and training resources. The OTel collector is
configured with YAML. The website is generated from data. The
billing system reads CSV. Nowhere in the operational workflow
should an operator need to write code.

---

## The Competitive Landscape

No one else is doing this. The reasons are structural:

**Commercial vendors** make money from per-meter recurring fees.
An open-source alternative destroys their business model. They have
no incentive to build one.

**Open-source SCADA projects** are built by industrial automation
engineers who think in terms of PLCs, HMI screens, and OPC servers.
Water utility operations — billing, regulatory compliance, asset
management, hydraulic planning — are outside their domain.

**GIS vendors** (Esri) sell expensive enterprise platforms. QGIS is
excellent but is a general-purpose tool. The water-specific plugins
(Giswater, Gusnet) are built by civil engineers, not systems
programmers. They don't integrate with telemetry or billing.

**Water engineering consultants** build bespoke systems for
individual clients. The work is not reusable and not open.

The gap is at the integration layer. Every component exists. No one
has incentive to assemble them — except an operator who needs them
and knows how to build software.

---

## Why This Matters

There are approximately 50,000 community water systems in the
United States. The vast majority are small — serving fewer than
3,300 people. Many are operated by part-time staff or volunteers.
They face the same regulatory requirements as large systems, with
a fraction of the resources.

The cost of commercial SCADA, billing, and GIS software — typically
$10,000 to $50,000 per year for a small system — is a meaningful
fraction of their operating budget. Many simply go without, relying
on manual processes that increase the risk of compliance failures,
undetected leaks, and deferred maintenance.

A free, open-source platform that covers the full operational scope
— from pulse counters in the pump house to regulatory filings —
would materially reduce the cost and complexity of operating safe
drinking water infrastructure. It would also make the knowledge
embedded in that software available to every system, rather than
locked inside vendor implementations that disappear when contracts
end.

Water is infrastructure. Infrastructure should be open.

---

## Current Status and Next Steps

The platform is in daily production at the Caspar Water System.
The telemetry pipeline, data lake, billing system, and public
website are operational. The hydraulic modeling and GIS components
are in active development.

The immediate roadmap:

1. Build the Caspar pipe network in QGIS with GeoPackage storage
2. Generate the EPANET model from the GIS data
3. Deploy the interactive pressure explorer on the website
4. Integrate rtlamr for drive-by meter reading
5. Connect meter data to the billing pipeline
6. Generate California eAR from operational data
7. Document everything so the next system can adopt it

The code is at github.com/jmacd/caspar.water,
github.com/jmacd/watertown, and github.com/jmacd/supruglue.
Apache-2.0 license. Contributions welcome.
