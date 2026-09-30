---
title: Reverse Acidification
weight: 60
layout: blog
section: Blog
date: "2024-05-01"
image: "/img/ph-adjustment.svg"
---

{{ figure src="/img/ph-adjustment.svg" /}}

Our pH-adjustment process recirculates water from our service main
back into storage, on the way passing through a **venturi injector**
that draws air into the pipe. As it returns to the tank, the air/water
mixture is distributed through an **eductor**, which induces
circulation in the tank.

This type of system works for water with low pH as the result of
carbonic acid, H₂CO₃, which dissolves in solution into a bicarbonate
ion HCO₃⁻ and a hydrogen ion H⁺.

```
CO₂ + H₂O ⇌ H₂CO₃ ⇌ H⁺ + HCO₃⁻
```

Adding air into the water allows CO₂ to escape in the direction of
equillibrium, leaving fewer hydrogen ions: higher pH, less
acidity. This is the same chemistry driving ocean acidification,
running in reverse.

(Our water is also high in soluble iron Fe²⁺. This treatment causes
red iron oxide Fe₂O₃ to precipitate onto the floor of the tank.)

## pH

In chemistry, `[X]` denotes the concentration of compound `X` in
solution. For example, `[H⁺]` is the concentration of hydrogen ions
and `[HCO₃⁻]` is the concentration of bicarbonate (i.e., hydrogen
carbonate) ions.  In 1908, American chemist Lawrence Joseph Henderson
derived an equation relating the concentration of hydrogen ions in a
buffer solution.

```
H⁺ + A⁻ ⇌ HA
```

He was studying how carbon dioxide regulates oxygen in blood. The
Danish chemish Karl Albert Hasselbach, also studying blood chemistry,
rearranged this into its modern form by taking the negative-logarithm
of hydrogen concentration known as pH.

```
pH = -log₁₀[H+]
```

The Henderson-Hasselbach equation can be simplified for normal pH
values (i.e., not extreme); the concentration `[H₂O]` is effectively
constant, and we can combine the effects of CO₂ gas and aqueous H₂CO₃
into as single form denoted `[H₂CO₃*]`.  The acid disassociation
constant `pKa` expresses the thermodynamic equillibrium for carbonic
acid, the midpoint of its buffering range.

```
pH = pKa + log₁₀( [HCO₃⁻] / [H₂CO₃*] )
```

This tells us that for a carbonic acid solution, pH is defined by a
constant function of temperature plus a logarithmic function of 

## Alkalinity

Alkalinity is the ability of a solution to neutralize acid. In the
context of drinking water, alkalinity is expressed in concentration
units equivalent to `[CaCO₃]`. Calcium carbonate is the standard
alkalinity measure in freshwater system both for being most common
(e.g., from rain over limestone) and because its molecular weight
rounds neatly to 100 g/mol. 

Drinking water systems generally know or sample raw water alkalinity
measured as CaCO₃ in mg/L. The Caspar system's raw water measures 12
mg/L which is relatively "soft", meaning little in the way of calcium
or magnesium.

For typical drinking water pH (see the "Bjerrum plot"), the dominant
carbonate species is bicarbonate ion (HCO₃⁻); it means for drinking
water pH, one unit of alkalinity equals one mole of bicarbonate ion.
From this, we can compute:

12 mg/L alkalinity 

indicates 

12 mol/L 



Alkalinity (as CaCO3, mg/L)  ≈  [HCO3-]






How effective is this treatment? Our raw water starts at around pH 5.9
with alkalinity of 12 mg/L (as CaCO₃). Each 1000 gallons of this water
contains about 1kG of CO₂ as it enters the tank, of which 100g
off-gasses as CO₂. Water enters the service main with pH in the range
of 6.5-6.7 depending on ambient temperature.

How efficient is this treatment? It's not. The process uses about
12kWh of electricity per day and although there is nothing disposable
in the process, it costs significantly more than a calcium carbonate
media filter.
