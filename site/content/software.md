---
title: Software
weight: 70
layout: page
section: Main
---

## Software

#### Open-source

Caspar Water System thanks the authors of the many pieces of computer software/system that we depend on, including:

- [Debian Linux 🐧](https://www.debian.org/)
- [Beagleboard](https://www.beagleboard.org/)
- [InfluxDB](https://www.influxdata.com/lp/influxdb-database)
- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
- [DuckDB](https://duckdb.org/)
- [Observable Framework](https://observablehq.com/framework/)
- Many Rust and Golang libraries, especially [mochi-mqtt/server](https://github.com/mochi-mqtt/server), and [simonvetter/modbus](https://github.com/simonvetter/modbus), and the [Rust Apache Arrow libraries](https://github.com/apache/arrow-rs).

Our source code is available under an Apache-2 license at [jmacd/caspar.water](https://github.com/jmacd/caspar.water), including:

- Custom OpenTelemetry collector build including receivers (modbus, current-loop, mqtt/sparkplug, bme280, atlas pH), exporters (influxdb, LCD displays), etc.
- Billing program (thanks [johnfercher/maroto](https://github.com/johnfercher/maroto) for the PDF generator library).
- Terraform definitions for cloud and station computer infrastructure (station, gateway, cloud).

#### Duckpond

[Duckpond](https://github.com/jmacd/duckpond) is a "local-first" Rust software system for managing timeseries from a variety of sources (e.g., random CSV files), based on DuckDB and Parquet files. This manages a file system of timeseries data and exports to Observable Framework.

🚧 Duckpond is being used to publish water monitoring data collected by the [Noyo Harbor Blue Economy](https://noyooceancollective.org/) project in a volunteer collaboration. Includes a vendor-specific [HydroVu](https://www.hydrovu.com) client library.

🚧 Duckpond is being used to publish our [high-resolution water monitoring data](./well_depth/index.html).

#### Supruglue

[Supruglue](https://github.com/jmacd/supruglue) is a C++ programming environment for the Beaglebone/Texas Instruments *am335x* PRU real-time chip aimed at being low-tech.

Mmmm. Proof of concept industrial real-time timer switch (that logs to the cloud), pulse counter, UI-1203 ("Sensus protocol") reader. I would prefer to write such a thing in Rust today, and we're not certain that Texas Instruments will continue producing this chip!
