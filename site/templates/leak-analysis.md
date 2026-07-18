---
title: "Leak Analysis"
layout: data
---

# Leak Analysis

{{ breadcrumb /}}

Leak analysis for water systems that **do not have a high-resolution,
cumulative encoded meter**. Where a totalizing meter would let you read a leak
directly as continuous overnight consumption, here we infer it from the noisy
**system pressure** signal instead.

Each night, while the well pump is off and demand is low, we fit the pressure
trend over the longest quiet stretch. A steady downward slope means water is
leaving the system with nothing drawing it — a leak. The **daily drawdown
rate** below is that fall rate (psi/hour; higher = faster fall), and the
**leak score** maps it to a probability. A single high night is weak evidence;
several consecutive elevated days are a reliable leak signal, while one clean
night is strong evidence the main is sound.

{{ viz renderer="chart" /}}
