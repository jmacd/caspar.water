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

The aeration process works by removing carbon dioxide from the water through natural off-gassing. It's the reverse of the process causing ocean acidification, because the water has a higher concentration of carbon dioxide than the atmosphere. The addition of O₂ to the water disrupts the following chemical equilibrium:

<div class="science">
<strong>CO₂ + H₂O ⇌ H₂CO₃ ⇌ H⁺ + HCO₃⁻</strong><br>
<strong>O₂ + HCO⁻ ⇌ HCO₃⁻</strong>
</div>

As CO₂ is removed, fewer hydrogen ions (H⁺) are present, effectively raising the water's pH level. Our water is served with pH measuring around 6.8.

In winter months, we serve approximately 800 gallons per day. In summer months, we serve approximately 2,000 gallons per day.

## Telemetry

Five instruments report into our [monitoring](./monitoring.html) pipeline:
four 4–20 mA current loops (well depth, system pressure, chlorine tank
level, concrete tank level) feed a COTS MQTT–Sparkplug device, and an
Atlas Scientific pH probe is read directly by a BeagleBone Black
Industrial in the pumphouse. An OpenTelemetry Collector on the BBB
forwards the merged stream over OTel Arrow to the gateway, which
archives JSON to attached storage and forwards to a cloud InfluxDB
behind a Caddy TLS proxy at `influx.casparwater.us`.

{{ figure src="./img/telemetry-system.svg" caption="Caspar Water telemetry data flow: sensors → pumphouse BBB → gateway → cloud InfluxDB." /}}
