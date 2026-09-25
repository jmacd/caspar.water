---
title: Monitoring
weight: 60
layout: page
section: Main
---

## Monitoring

Owner/operator Joshua MacDonald is a software engineer with professional experience in telemetry systems, hence our monitoring system uses "cloud-native" software practices. We collect five instruments and currently evaluate two operational checks:

The current water checks are evaluated from committed pond data. Each check
links to the most relevant existing graph; future graphs will show the exact
derived quantity used by the check.

<link rel="stylesheet" href="assets/monitor-status.css">

<div class="monitor-summary" data-water-monitors data-status-url="pond-status/water/status.json">
  <p data-loading>Loading water monitoring status...</p>
</div>

<script type="module" src="assets/monitor-status.js"></script>

Operators access our [Influxdb](https://influx.casparwater.us) instance with live monitoring data collected through several OpenTelemetry Collectors.

We have high-resolution well depth measurements dating back to August 2022, with which we can see the history of leaks, leak repairs, faucets left running, and other kinds of fine detail about our impact on the aquifer. See the [Well Depth History](/well-depth-history.html) page for an annotated 4-year timeline.

We also publish pump-cycle analyses showing how the well pump draws down the aquifer and how it recovers afterward: a [Drawdown by month](/analysis/drawdown-by-month.html) chart and a [Horner recovery by month](/analysis/horner-by-month.html) plot, each aggregated into per-month median and P10-P90 bands.
