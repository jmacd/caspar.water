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
constant function of temperature plus a logarithm of a ratio.

## Alkalinity

Alkalinity is the ability of a solution to neutralize acid, it's a
function of negative charge imbalance.  For a simple carbonate system
with drinking water pH (see the "Bjerrum plot"), dominant carbonate
species is bicarbonate ion (HCO₃⁻) and alkalinity is approximately the
difference `[HCO3-] - [H+]`.

In the context of drinking water, alkalinity is expressed in units
equivalent to `[CaCO₃]`. Calcium carbonate concentration is the
standard measure for alkalinity in freshwater because it is most
common (e.g., from rain on limestone) and because its molecular weight
rounds neatly to 100 g/mol.

Drinking water systems generally know or sample their raw water
alkalinity. Since calcium neutralizes two positive ions per unit of
mass, we can translate from alkalinity (mg/L) into units of charge per
liter. Our raw water has alkalinity 12 mg/L,

```
Alkalinity = [HCO3-] = 12 mg/L · 2 / 100 g/mol = 0.24mmol/L
```

To a good approximation, we known `[HCO3-]` is 0.24mmol/L. More
importantly, alkalinity does not change as C₂O offgasses, because C₂O
is charge-neutral. pH rises because there is less H₂CO₃* with the same
balance of charge.

## Mass

The Henderson-Hasselback equation can be rearranged with `[HCO₃⁻]`
fixed by alkalinity to solve for `[H₂CO₃*]`.

```
Alkalinity = [H₂CO₃*] · 10^(pH - pKa)
```

The quantity `[H₂CO₃*]` becomes a function of the measured pH and
temperature.

```
[H₂CO₃*] = Alkalinity · 10^(pKa - pH)
```

From this we can derive a change of mass from a change of pH.

## Temperature

Using temperature, we can estimate the acid disassociation constant
`pK` for carbonic acid. There is a well-established Plummer &
Busenberg formula, a emperical fit with 5 terms:

```
pK = -log(a + b·T + c/T + d·log10(T) - e/T²)
```

As water circulates through the injector, CO₂ is extracted in
temperature-dependent process. We will assume by the Arrhenius
equation that the rate of this process `E(T)` is an exponential
function of temperature that we can fit from observed data.

```
E(T) = E_0 · exp( k · (T - T_0) )
```

## Reaction

The setup is modeled as a well-mixed tank where raw water enters with
`pH_raw`, is treated continuously, and leaves with `pH_finished`. Our
raw water enters the tank with pH 5.9 and exits with higher pH. As
constants in this system, we have:

- Alkalinity
- Treatment
- Raw pH

The control variables are:

- Temperature
- Flow

With an analytical model for the injector-eductor system (e.g., a mass
transfer coefficient), we could predict finished pH from the input
variables.

Using these equations and conservation of mass, we can estimate how
many grams of CO₂ off-gas in a typical day as the change in acid
concentration times the molecular mass of CO₂ times the volume of
water. This works out to about 100g on a typical day.

With several real measurements of temperature, flow, and finished pH,
we can fit parameters for the extraction process `E(T)`. Finished pH
can then be modeled as a function of temperature and flow, here is the
predicted pH of our system.

{{ figure src="/img/ph-model.svg" /}}

## Cost

How effective is this treatment? Our raw water starts at around pH 5.9
with alkalinity of 12 mg/L (as CaCO₃). Each 1000 gallons of this water
contains about 1kG of CO₂ as it enters the tank, of which 100g
off-gasses as CO₂. Water enters the service main with pH in the range
of 6.3-6.7 depending on temperature and flow.

How efficient is this treatment? It's not. The process uses about
12kWh of electricity per day and although there is nothing disposable
in the process, it costs around 3x the cost of operating a calcium
carbonate media filter.
