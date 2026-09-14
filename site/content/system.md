---
title: System Design
weight: 50
layout: page
section: Main
---

## System Design

In modern times, the Caspar Water System has been described as a "chlorinator in the woods". We use a relatively simple process to provide clean and safe drinking water to our community.

- **Source:** Our raw water is sourced from a 188-feet deep well.
- **Disinfection:** The water undergoes chlorination to deactivate harmful bacteria and waterborne pathogens.
- **Aeration:** The water undergoes aeration to raise pH and oxidize iron.
- **Storage:** Treated water is stored in a 10,000-gallon concrete tank.
- **Distribution:** The water main has a linear layout with approximately 1 mile of pipe. While it starts with six-inch pipe, maintenance during our "ghost town" years has left the water main with a mixture of materials and combination of 6", 4", 3", and 2" pipe.
- **Service:** Our water system has 12 service connections, including the Caspar Community Center and the historic Caspar Inn.
- **Pressure:** Our water system delivers water using gravity feed with static pressures between 35psi and 60psi.

In winter months, we serve approximately 800 gallons per day. In summer months, we serve approximately 2,000 gallons per day.

## Telemetry

Our [monitoring](./monitoring.html) pipeline spans three sites and is
built around two OpenTelemetry Collectors.

{{ figure src="./img/telemetry-system.svg" caption="Three sites, two
OpenTelemetry Collectors: sensors merge at the pumphouse, traverse a
radio link to the gateway, and reach the cloud as both an archival
JSON stream and a live InfluxDB feed." /}}
