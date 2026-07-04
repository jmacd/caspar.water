# Proposal: Onsite Water-Quality Data Collection for Noyo Marine Center

> **Prepared for:** Noyo Marine Center — Noyo Harbor water-quality monitoring project
> **Prepared by:** Joshua MacDonald — volunteer; Principal Software Engineer, Microsoft; owner/operator, Caspar Water Company
> **Date:** 2026-06-28
> **Status:** Hardware purchase proposal

## Summary

Noyo Marine Center currently collects water-quality data from two
multi-parameter probes located at its Field Station dock. These probes
were funded through Noyo Harbor Blue Economy project grants, and they
are being operated by the Noyo Marine Center to monitor conditions
for onsite aquatic life.

This proposal asks Noyo Marine Center to purchase a low-cost computer
to connect directly by cable to the two probes. The proposed hardware
will eliminate subscription costs and produce a public portal for
sharing water quality data on the Noyo Marine Center website.

## Background

The water quality data being collected is sent over the cellular
network to a software service maintained by the manufacturer, carrying
recurring subscription fees. The probes measure disolved oxygen,
salinity, water temperature, and tide level. While three probes were
originally purchased for the study, one was removed from service.

## Engineering

Joshua is a Principal Software Engineer at Microsoft with a specialty
in open-source telemetry systems. He is a member of the OpenTelemetry
technical committee, an industry association under the Linux-foundation
responsible for software telemetry protocols.

Joshua became the owner/operator of Caspar Water Company in 2021 and
began building a small-scale, low-cost "operating system" for use by
small water systems. He has been volunteering on this effort since the
Noyo Harbor Blue Economy project formed with Jami Miller.

## Open-Source Software

Open-source software is software whose underlying source code is
licensed for public use, allowing anyone to view, modify, share, and
redistribute their work. Caspar Water Company is developing
open-source software to support data collection in small-scale
industrial and environmental monitoring applications.

The software, named Duckpond, provides a complete platform for
collecting, archiving and publishing telemetry data to a public
portal. Duckpond designed to operate at low cost with flexible options
for off-site storage.

Open-source software is popular with users for avoiding vendor
lock-in. Becuse open-source software is available to the public, Noyo
Marine Center will be able to build and operate the software itself,
should it become necessary for any reason.

## Detailed design

### Computer

The selected computer will be a [Beagle Play single-board
computer](https://www.beagleboard.org/boards/beagleplay), designed by
the BeagleBoard foundation. It is an open-source hardware design
running the Linux operating system. This model features a number of
wireless connectivity options which could be used to collect

We will add a 128GB microSD card for local storage.

### Communications

The In-Situ AT500 probes are designed with support for multiple
communictation protocols. The simplest and least expensive choice for
Noyo Marine Center is the Modbus protocol using RS-485 communication.

The RS-485 protocol will be added to the computer using a [Mikroe
RS485 Click 3.3V add-on board](https://www.mikroe.com/rs485-33v-click).

### Power

The computer and accessories will be powered through a standard 120V
outlet using two AC:DC power supplies, Mean Well HDR-15-24 for the
probes and HDR-30-5 for the computer.

The system will draw up to 25W of power, approximately 0.6kW daily,
220 kW or approximately $100 yearly to operate.

### Cabling

In-Situ cables are expensive and sold by the foot. It appears
that Noyo Marine Center has long-enough cable

https://in-situ.com/us/rugged-cable-splitter




## Bill of materials

Approximate single-unit prices, USD, excluding tax and shipping. Three
small orders: the compute parts, the power supplies, and one
AutomationDirect order that covers the enclosure and all panel wiring.

**Compute** — *SparkFun / DigiKey / Mouser*

| Part | Part number | Qty | Price |
|------|-------------|----:|------:|
| BeaglePlay single-board computer | BeaglePlay | 1 | $101 |
| Mikroe RS485 Click, 3.3 V | MIKROE-986 | 1 | $22 |
| SanDisk 128GB MAX Endurance microSDXC | SDSQQVR-128G | 1 | $57 |

**Power** — *DigiKey / Mouser*

| Part | Part number | Qty | Price |
|------|-------------|----:|------:|
| Mean Well 5 V DIN supply (computer) | HDR-30-5 | 1 | $20 |
| Mean Well 24 V DIN supply (probes) | HDR-15-24 | 1 | $20 |

**Enclosure and panel wiring** — *all from [AutomationDirect](https://www.automationdirect.com) in one order*

| Part | Part number | Qty | Price |
|------|-------------|----:|------:|
| Steel enclosure, NEMA 1 indoor, ~10×8×4 in, screw cover[^enc] | B100804 | 1 | $30 |
| Steel back panel (DIN backing plate) | SPB1008 | 1 | $12 |
| DIN rail, 35 mm steel, 1 m (cut to fit) | DN-R35S1 | 1 | $8 |
| DIN-rail end brackets | DN-EB35 | 2 | $2 |
| Feed-through terminal blocks, 12 AWG | DN-T12 | 6 | $6 |
| Ground terminal block (green/yellow) | DN-G12 | 2 | $4 |
| Enclosure grounding lug kit (any 10–14 AWG panel-bond lug) | — | 1 | $6 |
| DIN-rail mount for the BeaglePlay (C45 snap clips + M3 nylon standoffs, from Amazon) | — | 1 | $8 |
| Fasteners (M4 screws for rail, panel studs are included) | — | 1 | $4 |

**Approximate total: ~$300**

[^enc]: This is an indoor NEMA 1 enclosure that protects the hardware in a
sheltered location. Exterior or wash-down placement would require a weather-rated
enclosure (e.g. a NEMA 4X polycarbonate box), at higher cost.

