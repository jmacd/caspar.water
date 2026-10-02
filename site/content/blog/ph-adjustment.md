---
title: Reverse Acidification
weight: 60
layout: blog
section: Blog
date: "2024-05-01"
image: "/img/ph-adjustment.svg"
---

{{ figure src="/img/ph-adjustment.svg" /}}

Our pH-adjustment process recirculates water from the service main
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

## pH

In chemistry, `[X]` denotes the concentration of compound `X` in
solution. For example, `[H⁺]` is the concentration of hydrogen ions
and `[HCO₃⁻]` is the concentration of bicarbonate ions.  In 1908,
American chemist Lawrence Joseph Henderson derived an equation
relating the concentration of hydrogen ions in a buffer solution.

```
H⁺ + A⁻ ⇌ HA
```

He was studying how carbon dioxide regulates oxygen in blood. The
Danish chemish Karl Albert Hasselbach, also studying blood chemistry,
rearranged this into its modern form by taking the negative-logarithm
of hydrogen concentration known as pH.

<div class="equation">
pH = -log<sub>10</sub>[H⁺]
</div>

The Henderson-Hasselbach equation can be simplified for normal pH
values (i.e., not extreme); the concentration `[H₂O]` is effectively
constant, and we can combine the effects of CO₂ gas and aqueous H₂CO₃
into as single form denoted `[H₂CO₃*]`.  The acid disassociation
constant <code>pK<sub>a</sub></code> expresses the thermodynamic equillibrium for carbonic
acid, the midpoint of its buffering range.

<div class="equation">
pH = pK<sub>a</sub> + log<sub>10</sub>( [HCO₃⁻] / [H₂CO₃*] )
</div>

This tells us that for a carbonic acid solution, pH is defined by a
constant function of temperature plus a logarithm of a base-to-acid
ratio.

## Alkalinity

Alkalinity is the ability of a solution to neutralize acid, it's a
function of negative charge imbalance.  For a simple carbonate system
with drinking water pH (see the "Bjerrum plot"), the dominant carbonate
species is bicarbonate ion (HCO₃⁻) and alkalinity is approximately the
difference `[HCO₃⁻] - [H⁺]`.

In the context of drinking water, alkalinity is expressed in units
equivalent to `[CaCO₃]`. Calcium carbonate concentration is the
standard measure for alkalinity in freshwater because it is the most
common (e.g., from rain on limestone) and because its molecular weight
rounds neatly to 100 g/mol.

Drinking water systems generally know or sample their raw water
alkalinity. Since calcium neutralizes two positive ions per unit of
mass, we can translate from alkalinity (mg/L) into units of charge per
liter. Our raw water has alkalinity 12 mg/L,

<div class="equation">
Alkalinity = [HCO₃⁻] = 12 mg/L · 2 / 100 g/mol = 0.24 mmol/L
</div>

To a good approximation, we known `[HCO₃⁻]` is 0.24 mmol/L. More
importantly, alkalinity does not change as CO₂ offgasses, because CO₂
is charge-neutral. pH rises because there is less H₂CO₃* with the same
balance of charge.

## Mass

The Henderson-Hasselback equation can be rearranged with `[HCO₃⁻]`
fixed by alkalinity to solve for `[H₂CO₃*]`.

<div class="equation">
Alkalinity = [H₂CO₃*] · 10<sup>(pH - pK<sub>a</sub>)</sup>
</div>

The quantity `[H₂CO₃*]` becomes a function of the measured pH and
temperature.

<div class="equation">
[H₂CO₃*] = Alkalinity · 10<sup>(pK<sub>a</sub> - pH)</sup>
</div>

From this we can derive a change of mass from a change of pH.

## Temperature

Using temperature, we can estimate the acid disassociation constant
`pK` for carbonic acid. There is a well-established Plummer &
Busenberg formula, a emperical fit with 5 terms:

<div class="equation">
pK = -log<sub>10</sub>(k<sub>1</sub> + k<sub>2</sub>·T + k<sub>3</sub>/T + k<sub>4</sub>·log<sub>10</sub>(T) - k<sub>5</sub>/T<sup>2</sup>)
</div>

As water circulates through the injector, CO₂ is extracted in
temperature-dependent process. We will assume by the Arrhenius
equation that the rate of this process `E(T)` is an exponential
function of temperature that we can fit from observed data.

<div class="equation">
E(T) = E<sub>0</sub> · exp( k · (T - T<sub>0</sub>) )
</div>

## Reaction

The setup is modeled as a well-mixed tank where raw water enters with
<code>pH<sub>raw</sub></code>, is treated continuously, and leaves with <code>pH<sub>finished</sub></code>. Our
raw water enters the tank with pH 5.9 and exits with higher pH. As
constants in this system, we have:

- Alkalinity
- Treatment
- Raw pH

The ambient variables are:

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

{{ figure src="/img/ph-model.png" /}}

## Cost

How efficient is this treatment? It's not. The process uses about
12kWh of electricity per day and while there is nothing disposable
in the process, it costs around 3x the cost of operating a calcium
carbonate media filter.


