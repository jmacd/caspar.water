---
title: Site activity
layout: data
---

# Site activity

Daily reported page views over the last 30 UTC calendar days, colored by
section of the site. The total height is that day's count, not a running total.

{{ viz renderer="stacked-area" x="timestamp" y="views" color="section" days="30" y_title="Reported page views" order="Blog,Static content,Monitoring,Noyo,Other / unclassified" /}}

These are reported page views, not unique visitors or verified human visits.
Today is partial. Missing days do not establish zero traffic, and observed
counts do not establish uninterrupted collection.

Sections reflect the current publisher inventory. Unrecognized or retired
pages count under Other / unclassified without publishing their paths.
