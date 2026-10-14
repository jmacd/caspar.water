---
title: Pond Status
weight: 100
layout: page
hidden: true
---

## Pond Status

Raw monitoring status for every pond in this deployment. This page is not
linked from navigation; it exists for operators to check all ponds at a
glance without SSHing into Watershop. Each card below is rendered from that
pond's authoritative `status.json`, published atomically with each site
build (the browser never reaches Watershop directly).

<link rel="stylesheet" href="assets/monitor-status.css">

### Water

<div class="monitor-summary" data-monitor data-title="Water monitoring" data-status-url="pond-status/water/status.json">
  <p data-loading>Loading water pond status...</p>
</div>

### Septic

<div class="monitor-summary" data-monitor data-title="Septic monitoring" data-status-url="pond-status/septic/status.json">
  <p data-loading>Loading septic pond status...</p>
</div>

### Noyo Harbor

<div class="monitor-summary" data-monitor data-title="Noyo monitoring" data-status-url="pond-status/noyo/status.json">
  <p data-loading>Loading Noyo pond status...</p>
</div>

### Site

<div class="monitor-summary" data-monitor data-title="Site monitoring" data-status-url="pond-status/site/status.json">
  <p data-loading>Loading site pond status...</p>
</div>

<script type="module" src="assets/monitor-status.js"></script>
