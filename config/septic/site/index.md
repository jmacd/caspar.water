---
title: "Home"
layout: default
---

# Septic Station Monitor

## About

This site monitors an Orenco septic system via Modbus registers,
collected by an [OpenTelemetry collector](https://opentelemetry.io/)
running on a BeaglePlay ARM64 board (`septicplaystation.local`),
exported as OtelJSON, and ingested into
[DuckPond](https://github.com/jmacd/caspar.water).

## Data Groups

| Group | Metrics | Description |
|-------|---------|-------------|
| [**Pump Amps**](/data/pumps.html) | RT Pump 1–2, DT Pump 3–4 | Motor current draw |
| [**Cycle Times**](/data/cycle-times.html) | RT Pump 1–2 CT, DT Pump 3–4 CT | Cumulative pump cycle counts |
| [**Pump Modes**](/data/pump-modes.html) | RT PumpMode, DT PumpMode | Operating mode registers |
| [**Flow Totals**](/data/flow-totals.html) | RT/DT TotalCount, TotalFlow, TotalTime | Cumulative flow counters |
| [**Dose Zones**](/data/dose-zones.html) | Zone 1–3 CountTday, TimeTday | Daily zone valve activity |
| [**Environment**](/data/environment.html) | Temperature, Pressure, Humidity | BME280 sensor on BeaglePlay |

Use the sidebar to explore the data at different time resolutions.
