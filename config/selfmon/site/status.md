---
title: "Pond Status"
layout: data
---

# Pond Status

[← Back to charts](./index.html)

Per-pond `systemctl status`-style snapshot, rendered server-side from
the systemd journal data ingested by this pond.  Each card shows the
last-seen journal entry, last "Started" line, last exit/failure line,
peak resident-set size of the most recent run (parsed from the pond
CLI's exit log), and a tail of recent MESSAGE lines.

This page refreshes whenever sitegen runs (every 60s in normal
operation).

{{ pond_status_grid /}}
