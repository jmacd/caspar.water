---
title: Software
weight: 70
layout: page
section: Main
---

## Software

#### Open-source

Caspar Water System thanks the authors of the many pieces of computer software/system that we depend on, including:

- [Debian Linux](https://www.debian.org/)
- [Apache Arrow](https://arrow.apache.org)
- [DataFusion](https://datafusion.apache.org)
- [Parquet](https://parquet.apache.org)
- [Beagleboard](https://www.beagleboard.org/)
- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
- [DuckDB](https://duckdb.org/)
- [Observable Plot](https://observablehq.com/plot/)

And many good libraries:

- [mochi-mqtt/server](https://github.com/mochi-mqtt/server)
- [simonvetter/modbus](https://github.com/simonvetter/modbus)
- [n0-computer/bao-tree](https://github.com/n0-computer/bao-tree)

Our source code is available under an Apache-2 license at
[jmacd/caspar.water](https://github.com/jmacd/caspar.water),
including:

- Custom OpenTelemetry collector build including receivers (modbus,
  current-loop, mqtt/sparkplug, bme280, atlas pH), exporters
  (influxdb, LCD displays), etc.
- Billing program.
- Terraform definitions for cloud and station computer infrastructure
  (station, gateway, cloud).

#### Watertown

[Watertown](https://github.com/jmacd/watertown) is a "local-first" Rust
software system and site generator that manages timeseries and tabular
data from a variety of sources, based on DataFusion for query,
Deltalake for transactions, and Parquet for columnar storage.

Watertown is being used to publish water monitoring data collected by
the [Noyo Harbor Blue Economy](https://noyooceancollective.org/)
project in a volunteer collaboration, see [our demo
site](https://casparwater.us/noyo-harbor).

Watertown is being used to publish this site, including our
[high-resolution water monitoring data](./data/well-depth.html).

#### Supruglue

[Supruglue](https://github.com/jmacd/supruglue) is a C++ programming
environment for the Beaglebone/Texas Instruments *am335x* PRU
real-time chip aimed at being low-tech.

Mmmm hmmm. A proof-of-concept industrial real-time pulse counter.
