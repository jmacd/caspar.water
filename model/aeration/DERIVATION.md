# From carbonic acid equilibrium to the pH-vs-flow graph

This is a from-scratch derivation of the two chemistry results used in this
project: (1) how many grams of CO2 a given pH shift represents, and (2) how
that turns into the steady-state pH-vs-throughput graph
(`aeration_model_ph_vs_flow.png`). College-level general/analytical
chemistry is assumed (acid/base equilibria, log scales, basic mass
balance) — no water-treatment background required.

## 1. The carbonic acid system

When atmospheric/soil CO2 dissolves in water it forms carbonic acid, which
is a weak diprotic acid:

```
CO2 + H2O  ⇌  H2CO3  ⇌  H+ + HCO3-  ⇌  H+ + CO3^2-
                (Ka1)          (Ka2)
```

In practice, dissolved CO2(aq) and true H2CO3 are lumped together as
"H2CO3*" because true carbonic acid is a tiny fraction of the dissolved
gas. In the pH range this water sits in (5.5–7.5), essentially none of the
carbon is present as carbonate (CO3^2-) — the second dissociation only
matters above pH ~9. So for our purposes there are really just two forms
that matter:

```
H2CO3*  ⇌  H+ + HCO3-        Ka1 = [H+][HCO3-] / [H2CO3*]
```

(Square brackets, `[X]`, are standard chemistry notation for "molar
concentration of X," i.e. moles per liter.)

### Deriving Henderson-Hasselbalch, step by step

It isn't obvious on sight how the equilibrium expression above turns into
"pH = pK1 + log(...)," so here is every step.

**Step 1 — recall two definitions.** Just as pH is defined as the negative
log10 of hydrogen ion concentration, pK1 is defined the same way from the
equilibrium constant:

```
pH  = -log10[H+]
pK1 = -log10(Ka1)
```

**Step 2 — take -log10 of both sides of the equilibrium expression.**
Starting from `Ka1 = [H+][HCO3-] / [H2CO3*]`:

```
-log10(Ka1) = -log10( [H+] · [HCO3-]/[H2CO3*] )
```

**Step 3 — split the log of a product into a sum of logs.** Using the
log rule `log(a·b) = log(a) + log(b)`, the right-hand side becomes:

```
-log10(Ka1) = -log10[H+]  -  log10( [HCO3-]/[H2CO3*] )
```

**Step 4 — substitute in the definitions from Step 1.** The left side
`-log10(Ka1)` is `pK1`, and `-log10[H+]` on the right is `pH`:

```
pK1 = pH  -  log10( [HCO3-]/[H2CO3*] )
```

**Step 5 — rearrange to solve for pH** (move the log term to the other
side by adding it to both sides):

```
pH = pK1 + log10( [HCO3-] / [H2CO3*] )
```

That last line is the **Henderson-Hasselbalch equation**. This single
equation is the workhorse for everything below: it relates a measurable,
human-friendly quantity (pH) to the underlying ratio of the two dissolved
carbon species.

**Physical intuition:** pK1 is just a fixed property of carbonic acid
(≈6.35 at 25°C — see §4 below for its temperature dependence). It is the
pH at which `[HCO3-] = [H2CO3*]` exactly, because then
`log10(1) = 0` and the equation collapses to `pH = pK1`. It's the
"midpoint" pH of this acid's buffering range, exactly analogous to how
acetic acid's pKa (~4.76) is the pH where acetate and acetic acid are
50/50. Away from that midpoint, pH deviates from pK1 by precisely
`log10([HCO3-]/[H2CO3*])` — i.e., the ratio of the two species tells you
how far pH sits above or below pK1.

## 2. Alkalinity stands in for [HCO3-]

Total alkalinity is defined as the water's total acid-neutralizing
capacity — operationally, the equivalents of strong acid needed to titrate
the water down to about pH 4.5. In the 4.5–8.3 pH window, essentially all
of that capacity comes from bicarbonate, so:

```
Alkalinity (as CaCO3, mg/L)  ≈  [HCO3-]  (once converted to the same units)
```

The conversion uses CaCO3's equivalent weight of 50 mg/meq (its molar mass,
100.1 g/mol, divided by 2 because CaCO3 contributes 2 equivalents of
alkalinity per mole):

```
[HCO3-] (mmol/L)  =  Alkalinity (mg/L as CaCO3) / 50
```

For this system, Alkalinity = 12 mg/L as CaCO3, so [HCO3-] ≈ 0.24 mmol/L,
essentially year-round and independent of pH — **this is the key fact that
makes the whole model tractable: aeration strips CO2 gas but does not
touch alkalinity.** Removing CO2 doesn't add or remove any strong acid or
base equivalents; it just re-partitions the same fixed pool of dissolved
inorganic carbon between the H2CO3* and HCO3- forms. That means we can
treat [HCO3-] (≈ alkalinity) as a constant and let the Henderson-Hasselbalch
equation tell us how [H2CO3*] must be changing as pH moves.

## 3. Deriving grams of CO2 released for a given pH shift

Rearranging Henderson-Hasselbalch for the CO2 concentration:

```
[H2CO3*] = [HCO3-] · 10^(pK1 - pH) ≈ Alk_mM · 10^(pK1 - pH)
```

So at the raw pH (5.9) and the final pH (say 6.7), the dissolved CO2
concentrations are:

```
C_raw   = Alk_mM · 10^(pK1 - pH_raw)
C_final = Alk_mM · 10^(pK1 - pH_final)
```

Since alkalinity doesn't change, the difference between these two is
exactly the CO2 mass that had to leave the water as gas:

```
ΔC (mmol/L) = Alk_mM · [ 10^(pK1 - pH_raw) - 10^(pK1 - pH_final) ]
```

To turn a concentration difference into a mass for a fixed batch, multiply
by the volume and CO2's molar mass (44.01 g/mol):

```
mass (mg) = ΔC (mmol/L) · 44.01 (mg/mmol) · V (L)
```

For 1000 gallons (V = 3785.4 L), substituting Alk_mM = Alk_mgL / 50:

```
mass (g) = Alk_mgL · Δratio · (44.01 · 3785.4 / 50) / 1000
         = Alk_mgL · Δratio · 3.33
```

where `Δratio = 10^(pK1-pH_raw) - 10^(pK1-pH_final)`. That reproduces the
"3.33 constant" rule of thumb from the original conversation, now with a
transparent derivation: it's just unit-converting the Henderson-Hasselbalch
difference for a 1000-gallon batch at room temperature (pK1 = 6.35).

**Worked example** (12 mg/L alkalinity, 55°F/12.8°C, pH 5.9 → 6.7):
`pK1(12.8°C) ≈ 6.44` (see §4 below), so
`Δratio = 10^(6.44-5.9) - 10^(6.44-6.7) = 3.47 - 0.55 = 2.92`, and
`mass = 12 · 2.92 · 3.33 ≈ 117 g` per 1000 gallons — matching the ~116 g
figure discussed earlier.

## 4. Why temperature matters: pK1 is not really constant

`pK1` (the negative log of carbonic acid's first dissociation constant) is
usually quoted as "6.35," but that's only true at 25°C. Like any
equilibrium constant, it's tied to the reaction's standard Gibbs free
energy, which is temperature-dependent (the van 't Hoff relationship). For
the CO2/carbonic acid system in freshwater, this project uses the
well-established **Plummer & Busenberg (1982)** empirical fit (the same
one used in geochemical modeling software like PHREEQC):

```
log(K1) = -356.3094 - 0.06091964·T + 21834.37/T + 126.8339·log10(T) - 1684915/T²
pK1 = -log(K1)                      (T in Kelvin)
```

This reproduces pK1 ≈ 6.35 at 25°C, 6.44 at 12.8°C (55°F), and 6.52 at
4.4°C (40°F) — i.e., **colder water has a higher pK1**, meaning at a given
pH, more of the total dissolved carbon sits as CO2 gas rather than
bicarbonate. That's why the same pH shift (5.9→6.7) requires stripping
more grams of CO2 in November than in May: the starting gas load is
larger in cold water, even though the alkalinity is identical.

## 5. Making it continuous: the CSTR mass balance

Sections 1–4 describe a single, fixed batch of water. But the actual tank
is continuously fed with raw water and continuously stirred/treated, so we
model it as a **Continuous-Stirred Tank Reactor (CSTR)**: a control volume
that is perfectly mixed, so the concentration leaving equals the
concentration everywhere inside it.

Two flows carry CO2 mass across the tank's boundary at steady state
(when concentrations have stopped changing day to day):

- **In:** raw water enters at rate `Q_raw` (gal/day) carrying CO2 at
  concentration `C_raw` (computed from Alk and pK1(T) as in §3).
- **Out (as liquid):** treated water leaves the tank (to serve demand) at
  the same rate `Q_raw`, carrying the tank's own concentration `C_tank`,
  because the tank is well-mixed.
- **Out (as gas):** the recirculation loop pulls tank water through the
  venturi/eductor at a much higher rate `Q_loop` and strips a fraction
  `E_T` of its CO2 to atmosphere on each pass, then returns the water (now
  lower in CO2) to the tank. This is a sink for CO2 mass, not a sink for
  water volume — none of the loop's water actually leaves the tank system.

At steady state, mass in = mass out:

```
Q_raw · C_raw  =  Q_raw · C_tank  +  E_T · Q_loop · C_tank
```

Solving for the tank's steady-state CO2 concentration:

```
C_tank = (Q_raw · C_raw) / (Q_raw + E_T · Q_loop)
```

And converting back to pH with Henderson-Hasselbalch (since alkalinity is
still ≈ constant, exactly as in §2-3):

```
pH_final = pK1(T) - log10( C_tank / Alk_mM )
```

This is the central equation of the model. Notice the intuition it
encodes: if `E_T · Q_loop >> Q_raw` (the loop processes far more water per
day than the raw demand), `C_tank` collapses toward zero and pH climbs
close to the maximum achievable; if `Q_raw` dominates, fresh acidic water
overwhelms the loop's stripping capacity and `C_tank` approaches `C_raw`
(pH stays close to the raw 5.9).

## 6. The one thing we can't derive: single-pass efficiency E_T

`E_T(T)`, the fraction of dissolved CO2 the venturi/eductor loop strips
out on a single pass, depends on bubble size, contact time, hydraulic
shear, gas solubility (Henry's Law), and diffusivity — a genuine mass
transfer coefficient (`K_La`) problem that would require lab
characterization of this specific injector/eductor to derive from first
principles.

Instead, this project **calibrates** E_T against two known field
observations (≈1000 gal/day, 55°F → pH 6.7; ≈1000 gal/day, 40°F → pH 6.4)
by solving the steady-state equation backwards:

```
E_T = Q_raw · (C_raw - C_tank) / (C_tank · Q_loop)
```

giving `E_T(55°F) = 0.369` and `E_T(40°F) = 0.150`. These two points are
then fit with a simple exponential (chosen because it's the natural shape
for a temperature-driven rate process — Henry's Law solubility and
diffusion coefficients both vary roughly exponentially with temperature):

```
E_T(T) = E0 · exp( k · (T - Tref) )
```

Anchoring at `Tref = 55°F` makes `E0` just equal to `E_T(55°F) = 0.369`,
and matching the second point gives:

```
k = ln( E_T(55°F) / E_T(40°F) ) / (55 - 40) = ln(0.369/0.150) / 15 ≈ 0.060 /°F
```

**Important caveat:** this is a two-point curve fit, not an independently
derived physical law. It reproduces the two known data points exactly by
construction; its predictive value elsewhere on the curve depends entirely
on whether the exponential-in-temperature assumption is actually a good
model for this particular injector/eductor's behavior. A third field data
point (e.g., a midsummer or midwinter pH reading) would be a real test of
the model, not just another point to fit.

## 7. Building the graph

With `pK1(T)`, `Alk_mM`, `pH_raw = 5.9`, `Q_loop = 14,400 gal/day` (10 GPM
continuous), and the calibrated `E_T(T)` curve in hand, the plot is just a
double loop:

```
for each temperature T in {35, 40, 45, ..., 70} °F:
    for each raw flow Q_raw in {500, 750, 1000, ..., 3000} gal/day:
        pk1     = pK1(T)
        C_raw   = Alk_mM · 10^(pk1 - 5.9)
        E_T     = E0 · exp(k · (T - 55))
        C_tank  = (Q_raw · C_raw) / (Q_raw + E_T · Q_loop)
        pH      = pk1 - log10(C_tank / Alk_mM)
    plot the (Q_raw, pH) points for this T as one line
```

Each temperature becomes one curve; the x-axis is raw water throughput
(gal/day) and the y-axis is the steady-state tank pH. The curves slope
downward (more throughput → less relative treatment → lower pH) and shift
upward with temperature (warmer water strips more easily), which is
exactly the seasonal behavior observed in the field.

## Summary of what's rigorous vs. what's fit

| Piece | Status |
|---|---|
| Henderson-Hasselbalch relating pH/HCO3-/CO2 | Standard chemistry, exact |
| Alkalinity ≈ [HCO3-] in this pH range | Standard approximation, very good here |
| pK1(T) (Plummer-Busenberg) | Established empirical geochemistry, not fit by us |
| CSTR steady-state mass balance | Standard chemical engineering, exact given its assumptions (perfect mixing, steady state) |
| E_T(T) exponential curve | **Calibrated to 2 data points** — a modeling choice, treat predictions elsewhere on the curve with caution |
