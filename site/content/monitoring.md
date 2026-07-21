---
title: Monitoring
weight: 60
layout: page
section: Main
---

## Monitoring

Owner/operator Joshua MacDonald is a software engineer with professional experience in telemetry systems, hence our monitoring system uses "cloud-native" software practices. We monitor five instruments:

- **[Well depth](/data/well-depth.html):** measures the height of the water column relative to the bottom of the well.
- **[Chlorine tank level](/data/chlorine-level.html):** lets us observe that the chlorine pump is operational.
- **[Water tank level](/data/tank-level.html):** tells us how much treated water is in storage.
- **[System pressure](/data/system-pressure.html):** lets us observe dynamic pressure and see that the aeration pump is running.
- **[pH level](/data/ph.html):** An in-tank probe measures the pH of the water, lets us see that our aeration process is effective.

Operators access our [Influxdb](https://influx.casparwater.us) instance with live monitoring data collected through several OpenTelemetry Collectors.

We have high-resolution well depth measurements dating back to August 2022, with which we can see the history of leaks, leak repairs, faucets left running, and other kinds of fine detail about our impact on the aquifer. See the [Well Depth History](/well-depth-history.html) page for an annotated 4-year timeline.

We also publish pump-cycle analyses showing how the well pump draws down the aquifer and how it recovers afterward: a [Drawdown by month](/analysis/drawdown-by-month.html) chart and a [Horner recovery by month](/analysis/horner-by-month.html) plot, each aggregated into per-month median and P10-P90 bands.
