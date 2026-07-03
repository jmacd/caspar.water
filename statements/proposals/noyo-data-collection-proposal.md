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
to connect directly by cable to the two probes. The hardware will run
open-source software built by Caspar Water Company that is designed to
collect, backup and publish small-scale telemetry data. The proposed
hardware will eliminate subscription costs and produce a public portal
for sharing water quality data on the Noyo Marine Center website.

## Background

The water quality data being collected is sent over the cellular
network to a software service maintained by the manufacturer, carrying
recurring subscription fees. The probes measure disolved oxygen,
salinity, water temperature, and pressure. While three probes were
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

## Detailed design

### Computer

The selected computer will be a [Beagle Play single-board
computer](https://www.beagleboard.org/boards/beagleplay), designed by
the BeagleBoard foundation. It is an open-source hardware design
running the Linux operating system. This model features a number of
wireless connectivity options which could be used to collect

We will add a Sandisk 128GB MAX Endurance microSDXC.

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

Beagle Play $101
Mikroe RS485 Click 3.3V $22
Sandisk 128GB MAX Endurance microSDXC $57
Mean Well MDR-30-5 $20
Mean Well MDR-15-24 $20
Enclosure
Wire, DIN-rail, mounting, terminal blocks, clock battery. $50


