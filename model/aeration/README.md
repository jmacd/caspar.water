# Aeration CSTR pH model + energy accounting

Back-of-the-envelope tooling that formalizes the chemistry/engineering
discussion in `../../gemini_says.txt` about the raw-water aeration
(venturi injector + eductor) system feeding the 10,000-gallon storage tank.

* `BLOG-SUMMARY.md` — the short, blog-post-ready explainer for how the
  pH-vs-flow graph was derived.
* `DERIVATION.md` — the full step-by-step chemistry refresher (Henderson-
  Hasselbalch, CSTR mass balance, etc.) behind that short version.

## Part 1 — pH model

Treats the tank as a Continuous-Stirred Tank Reactor (CSTR): raw water
(pH 5.9, 12 mg/L alkalinity as CaCO3) enters continuously, and a 10 GPM
loop recirculates tank water through a venturi/eductor that strips a
temperature-dependent fraction of dissolved CO2 on every pass. Steady
state gives:

```
C_tank   = (Q_raw * C_raw) / (Q_raw + E_T(T) * Q_loop)
pH_final = pK1(T) - log10(C_tank / Alkalinity)
```

`pK1(T)` uses the Plummer & Busenberg (1982) empirical fit for carbonic
acid's first dissociation constant (the same family of equation used by
PHREEQC/WATEQ4F), so cold-water behavior is physically grounded rather
than guessed.

`E_T(T)`, the single-pass stripping efficiency of the venturi/eductor, is
the "fiddle with it" parameter: `E_T(T) = E0 * exp(k * (T_F - Tref_F))`.
By default the script auto-calibrates `E0`/`k` against two known field
points (≈1000 gal/day at 55F → pH 6.7, and at 40F → pH 6.4); pass
`--e0`/`--k`/`--tref-f` to override and see how the whole family of
seasonal curves shifts.

Output: a table and a plot (`*_ph_vs_flow.png`) of steady-state pH vs.
raw-water gallons/day, with one line per tank temperature.

## Part 2 — energy accounting (circulation vs. aeration)

The system measures ~12 kWh/day continuously. This splits that into:

1. **Circulation baseline** — the unavoidable Darcy-Weisbach friction cost
   of just pushing 10 GPM through ~100 ft of 1.5" pipe and ~12
   simplified turns/fittings, converted to electrical watts via an assumed
   pump wire-to-water efficiency — **floored at the smallest practical
   potable-safe circulator's real electrical draw** (default 60W, e.g. a
   small bronze/stainless NSF-61 unit like a Taco 006 or Grundfos
   UP15-10). The pure hydraulics here are tiny (~2W, ~5W electrical), but
   no potable-water pump is built to draw single-digit watts, so the floor
   is what you'd actually pay even for circulation alone.
2. **Aeration surcharge** — measured total minus the circulation baseline.
   This is the extra energy the venturi/eductor spends creating the
   pressure differential needed to entrain and shear air into bubbles.

With the default assumptions, circulation (using a realistic small
potable pump) is ~1.4 kWh/day (~$131/yr); the remaining ~10.6 kWh/day
(~88% of the total, ~$964/yr) is the aeration surcharge attributable to
the venturi/eductor's pressure differential. This surcharge is the number
that's fair to compare against a passive calcite neutralizer's media
cost, since a calcite system would likely still need *some* circulation
pump (or you'd accept losing that mixing benefit) — the circulation cost
is roughly a wash either way.

Output: a printed cost table and a plot (`*_energy_breakdown.png`).

## Usage

```bash
python3 -m venv /tmp/venv-aeration && /tmp/venv-aeration/bin/pip install matplotlib numpy
/tmp/venv-aeration/bin/python aeration_model.py                  # both parts, default assumptions
/tmp/venv-aeration/bin/python aeration_model.py --e0 0.3 --k 0.05 # try a different efficiency curve
/tmp/venv-aeration/bin/python aeration_model.py --no-plots        # numbers only, no matplotlib needed
/tmp/venv-aeration/bin/python aeration_model.py --part 2 \
    --pump-efficiency 0.45 --n-turns 10 --k-per-turn 0.75         # tweak the hydraulics assumptions
```

Run `--help` for the full list of tunable parameters (alkalinity, raw pH,
loop GPM, pipe size/length/turns, pump efficiency, electricity cost,
calcite comparison cost, etc.).

## Caveats

* `E_T(T)`'s functional form (single exponential in temperature) is a
  modeling choice, not a measurement — it's a knob to match observed
  pH, not a substitute for actually characterizing the venturi/eductor.
* The energy split assumes a single lumped "wire-to-water" pump
  efficiency (default 35%) and simplified minor-loss coefficients (K=0.5
  per turn) for the "12 turns" — both are rough and worth tightening if
  you have pump curve data or better fitting counts/types. The
  circulation baseline is floored at `--min-pump-watts` (default 60W)
  since the theoretical hydraulic requirement is unrealistically small —
  there's no potable-safe pump built to draw a handful of watts.
* Alkalinity is assumed to be ~ equal to `[HCO3-]` in this pH range, which
  is standard practice but breaks down outside roughly pH 4.5–8.3.
