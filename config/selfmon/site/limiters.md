---
title: "Limiters"
layout: logs
---

# Watershop Pond Limiters

Current sliding-window and burst positions for every limiter configured on a
watershop-resident pond. Collection reads bounded control state locally; it
does not contact MinIO or scan retained limiter-usage history.

`charged` governs admission. `observed` is independently measured physical
traffic. They should agree during ordinary governed work; `observed` may exceed
`charged` when `POND_IGNORE_LIMITS` deliberately lets traffic through.
`observed_window_complete=false` means the pond was upgraded less than one
limiter window ago, so older charged buckets have no corresponding independent
measurement yet and the totals are not comparable.

{{ viz renderer="logs" /}}
