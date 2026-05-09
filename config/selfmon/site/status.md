---
title: "Status"
layout: data
---

# Pond Status

Per-pond `systemctl status`-style snapshot, rendered server-side from
the systemd journal data ingested by this pond and enriched with the
per-pond perf series.  Each card is colour-coded:

- **Green** -- last successful run is within `2 *` the timer's interval.
- **Yellow** -- last successful run is overdue.
- **Red** -- the most recent terminal event was a failure, or the
  timer is inactive.
- **Grey** -- not enough data to classify (perf-series lookup failed,
  no successful run yet, or the timer interval is unknown).

Timestamps are displayed in your local timezone with a relative
"X minutes ago" label that ticks every 30 s without a page reload.

{{ pond_status_grid /}}
