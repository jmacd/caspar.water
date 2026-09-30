#!/usr/bin/env python3
"""
aeration_model.py

A back-of-the-envelope model for the raw-water CO2-stripping (aeration)
system described in gemini_says.txt.

Part 1 - Chemistry (CSTR pH model)
-----------------------------------
The 10,000 gallon storage tank is treated as a Continuous-Stirred Tank
Reactor (CSTR). Raw water enters at a fixed alkalinity and pH (5.9), and a
3/4 HP pump continuously recirculates tank water at Q_loop gal/day through a
venturi injector + small-orifice eductor, stripping a temperature-dependent
fraction (E_T) of dissolved CO2 on every pass. At steady state:

    C_tank = (Q_raw * C_raw) / (Q_raw + E_T(T) * Q_loop)
    pH_final = pK1(T) - log10(C_tank / Alkalinity)

where C_raw = Alkalinity * 10**(pK1(T) - pH_raw), Alkalinity is expressed in
mM as HCO3- (bicarbonate is ~ equal to total alkalinity in this pH range),
and pK1(T) is the first dissociation constant of carbonic acid, which is
itself a function of water temperature (colder water holds CO2 more tightly
-> higher pK1 -> harder to strip -> lower final pH for the same treatment).

E_T(T), the single-pass stripping efficiency of the venturi/eductor loop, is
not something we can derive from first principles without a lot of detailed
diffuser/hydraulic data (see the "bubble size" and "venturi vs eductor"
discussion in gemini_says.txt). Instead we model it with two knobs:

    E_T(T) = E0 * exp(k * (T_F - Tref_F))

E0 is the "efficiency parameter" the user asked to fiddle with (baseline
single-pass stripping efficiency at the reference temperature Tref_F). k
controls how strongly efficiency falls off in cold water. The script's
defaults are pre-calibrated (see calibrate_efficiency()) against two known
field observations:

    * ~1000 gal/day, ~55F tank -> pH climbs to ~6.7
    * ~1000 gal/day, ~40F tank -> pH only reaches ~6.4

You can override E0/k/Tref on the command line to see how sensitive the
seasonal curves are to that assumption, which is the point of the exercise:
find an (E0, k) pair that makes the plotted lines fall in the observed
range, then treat that as your system's characterized performance.

Part 2 - Energy accounting (circulation vs. aeration)
------------------------------------------------------
The measured system draws ~12 kWh/day continuously. Two very different
things are consuming that energy:

  1. Circulation: just moving 10 GPM through ~100 ft of 1.5" pipe and
     ~12 fittings/turns has an unavoidable friction cost, independent of
     whether any air is being injected at all. We compute this with a
     standard Darcy-Weisbach + minor-loss calculation and divide by an
     assumed wire-to-water pump efficiency to get an electrical baseline.

  2. Aeration surcharge: the venturi needs a large pressure differential to
     pull in air, and the small eductor orifice adds backpressure to shear
     that air into fine bubbles. This is energy that would NOT be spent if
     the loop were just circulating water with no air injection. We
     estimate it as (measured total power) - (circulation baseline).

This split lets you compare the aeration surcharge (the part you could
eliminate by switching to a passive calcite neutralizer) against the
calcite media cost, while acknowledging that the circulation baseline may
still be worth paying for (it prevents tank staleness/stratification,
which calcite alone would not provide).

Usage examples
--------------
    python3 aeration_model.py                      # run both parts, save plots
    python3 aeration_model.py --e0 0.3 --k 0.05     # try a different efficiency curve
    python3 aeration_model.py --no-plots            # just print numbers
"""

import argparse
import math
import sys

try:
    import numpy as np
except ImportError:
    np = None


# ---------------------------------------------------------------------------
# Part 1: Chemistry / CSTR pH model
# ---------------------------------------------------------------------------

def f_to_c(t_f: float) -> float:
    return (t_f - 32.0) * 5.0 / 9.0


def pK1_carbonic(t_c: float) -> float:
    """First dissociation constant of carbonic acid in fresh water, as a
    function of temperature (deg C). Plummer & Busenberg (1982) empirical
    fit, widely used in geochemical modeling (e.g. PHREEQC/WATEQ4F).
    Reproduces ~6.35 @ 25C, ~6.44 @ 12.8C (55F), ~6.52 @ 4.4C (40F).
    """
    t_k = t_c + 273.15
    log_k1 = (
        -356.3094
        - 0.06091964 * t_k
        + 21834.37 / t_k
        + 126.8339 * math.log10(t_k)
        - 1684915.0 / (t_k ** 2)
    )
    return -log_k1


def alkalinity_mM(alk_mg_l_as_caco3: float) -> float:
    """Convert alkalinity in mg/L as CaCO3 to mM as HCO3- (bicarbonate is
    ~ all of the alkalinity in the pH 5.5-7.5 range we care about). CaCO3
    equivalent weight is 50 g/eq (100 g/mol / 2 eq/mol)."""
    return alk_mg_l_as_caco3 / 50.0


def raw_co2_mM(alk_mM: float, pk1: float, ph_raw: float) -> float:
    """Dissolved CO2 concentration (mM) implied by Henderson-Hasselbalch,
    given alkalinity (~ [HCO3-]) and pK1 at this temperature."""
    return alk_mM * 10 ** (pk1 - ph_raw)


def single_pass_efficiency(t_f: float, e0: float, k: float, tref_f: float) -> float:
    """Temperature-dependent single-pass CO2 stripping efficiency of the
    venturi/eductor loop, clamped to [0, 1]."""
    e_t = e0 * math.exp(k * (t_f - tref_f))
    return max(0.0, min(1.0, e_t))


def steady_state_pH(
    q_raw_gpd: float,
    t_f: float,
    q_loop_gpd: float,
    alk_mg_l: float,
    ph_raw: float,
    e0: float,
    k: float,
    tref_f: float,
):
    """Solve the CSTR steady state for final tank pH, and return
    intermediate values useful for reporting."""
    t_c = f_to_c(t_f)
    pk1 = pK1_carbonic(t_c)
    alk_mM = alkalinity_mM(alk_mg_l)
    c_raw = raw_co2_mM(alk_mM, pk1, ph_raw)
    e_t = single_pass_efficiency(t_f, e0, k, tref_f)

    c_tank = (q_raw_gpd * c_raw) / (q_raw_gpd + e_t * q_loop_gpd)
    ph_final = pk1 - math.log10(c_tank / alk_mM)

    return {
        "pk1": pk1,
        "alk_mM": alk_mM,
        "c_raw_mM": c_raw,
        "e_t": e_t,
        "c_tank_mM": c_tank,
        "ph_final": ph_final,
    }


def calibrate_efficiency(alk_mg_l: float, ph_raw: float, q_loop_gpd: float):
    """Back out the (E0, k) pair implied by two field observations:
    ~1000 gal/day @ 55F -> pH 6.7, and ~1000 gal/day @ 40F -> pH 6.4.
    This just reproduces the arithmetic done by hand in gemini_says.txt so
    the script's defaults line up with the known-good field data."""

    def required_e_t(t_f, q_raw, target_ph):
        t_c = f_to_c(t_f)
        pk1 = pK1_carbonic(t_c)
        alk_mM = alkalinity_mM(alk_mg_l)
        c_raw = raw_co2_mM(alk_mM, pk1, ph_raw)
        c_tank = alk_mM * 10 ** (pk1 - target_ph)
        return (q_raw * (c_raw - c_tank)) / (c_tank * q_loop_gpd)

    e_55 = required_e_t(55.0, 1000.0, 6.7)
    e_40 = required_e_t(40.0, 1000.0, 6.4)

    k = math.log(e_55 / e_40) / (55.0 - 40.0)
    tref_f = 55.0
    e0 = e_55  # efficiency at the reference temperature
    return e0, k, tref_f, e_55, e_40


def run_part1(args):
    print("=" * 78)
    print("PART 1: CSTR pH model (aeration chemistry)")
    print("=" * 78)

    if args.e0 is None or args.k is None:
        e0, k, tref_f, e_55, e_40 = calibrate_efficiency(
            args.alkalinity, args.raw_ph, args.loop_gpm * 1440.0
        )
        print(
            f"Auto-calibrated single-pass efficiency curve from field data:\n"
            f"  E_T(55F) = {e_55:.3f}, E_T(40F) = {e_40:.3f}\n"
            f"  -> E0={e0:.4f} @ Tref={tref_f:.0f}F, k={k:.4f} per degF\n"
        )
        if args.e0 is not None:
            e0 = args.e0
        if args.k is not None:
            k = args.k
    else:
        e0, k, tref_f = args.e0, args.k, args.tref_f
        print(f"Using user-specified efficiency curve: E0={e0}, k={k}, Tref={tref_f}F\n")

    q_loop_gpd = args.loop_gpm * 1440.0
    print(f"Fixed loop recirculation rate: {args.loop_gpm} GPM -> {q_loop_gpd:,.0f} gal/day")
    print(f"Alkalinity: {args.alkalinity} mg/L as CaCO3, Raw pH: {args.raw_ph}\n")

    temps_f = args.temps
    flows = list(range(int(args.flow_min), int(args.flow_max) + 1, int(args.flow_step)))

    header = f"{'Q_raw (gpd)':>12}" + "".join(f"{'pH @'+str(t)+'F':>12}" for t in temps_f)
    print(header)
    results = {t: [] for t in temps_f}
    for q in flows:
        row = f"{q:>12,}"
        for t in temps_f:
            r = steady_state_pH(
                q, t, q_loop_gpd, args.alkalinity, args.raw_ph, e0, k, tref_f
            )
            results[t].append(r["ph_final"])
            row += f"{r['ph_final']:>12.2f}"
        print(row)
    print()

    # sanity check against the two calibration anchors
    for t_f, target in [(55.0, 6.7), (40.0, 6.4)]:
        r = steady_state_pH(1000.0, t_f, q_loop_gpd, args.alkalinity, args.raw_ph, e0, k, tref_f)
        print(
            f"Check: 1000 gal/day @ {t_f:.0f}F -> pH {r['ph_final']:.2f} "
            f"(field target ~{target})"
        )
    print()

    if not args.no_plots:
        try:
            import matplotlib
            matplotlib.use("Agg")
            import matplotlib.pyplot as plt
        except ImportError:
            print("[part1] matplotlib not installed; skipping plot (--no-plots to silence)")
            return e0, k, tref_f

        plt.figure(figsize=(8, 5.5))
        for t in temps_f:
            plt.plot(flows, results[t], marker="o", markersize=3, label=f"{t:.0f}F")
        plt.xlabel("Raw water throughput (gallons/day)")
        plt.ylabel("Steady-state tank pH")
        plt.title(
            f"CSTR pH model — 10 GPM loop, {args.alkalinity} mg/L alk, "
            f"E0={e0:.3f}, k={k:.4f}/F"
        )
        plt.grid(True, alpha=0.3)
        plt.legend(title="Tank temp")
        plt.tight_layout()
        out = args.plot_prefix + "_ph_vs_flow.png"
        plt.savefig(out, dpi=150)
        print(f"[part1] wrote {out}")

    return e0, k, tref_f


# ---------------------------------------------------------------------------
# Part 2: Energy accounting (circulation vs. aeration)
# ---------------------------------------------------------------------------

# Standard nominal pipe inside diameters (inches), schedule 40
SCH40_ID_IN = {
    "1": 1.049,
    "1.25": 1.380,
    "1.5": 1.610,
    "2": 2.067,
}

G_FT_S2 = 32.174
GAMMA_WATER_LBF_FT3 = 62.4  # lbf/ft^3
NU_WATER_FT2_S = 1.1e-5  # kinematic viscosity, ~60F water, ft^2/s
GAL_TO_FT3 = 0.133681


def swamee_jain_f(re: float, eps_ft: float, d_ft: float) -> float:
    if re < 2300:
        return 64.0 / max(re, 1e-6)  # laminar
    return 0.25 / (math.log10(eps_ft / (3.7 * d_ft) + 5.74 / re ** 0.9)) ** 2


def circulation_hydraulics(
    gpm: float,
    pipe_size_in: str,
    length_ft: float,
    n_turns: int,
    k_per_turn: float,
    roughness_ft: float,
):
    d_in = SCH40_ID_IN[pipe_size_in]
    d_ft = d_in / 12.0
    area_ft2 = math.pi / 4.0 * d_ft ** 2

    q_cfs = gpm * GAL_TO_FT3 / 60.0
    v_fps = q_cfs / area_ft2

    re = v_fps * d_ft / NU_WATER_FT2_S
    f = swamee_jain_f(re, roughness_ft, d_ft)

    hf_ft = f * (length_ft / d_ft) * v_fps ** 2 / (2 * G_FT_S2)
    hm_ft = n_turns * k_per_turn * v_fps ** 2 / (2 * G_FT_S2)
    h_total_ft = hf_ft + hm_ft

    p_hyd_ftlb_s = GAMMA_WATER_LBF_FT3 * q_cfs * h_total_ft
    p_hyd_w = p_hyd_ftlb_s * 1.355818

    return {
        "d_in": d_in,
        "v_fps": v_fps,
        "re": re,
        "f": f,
        "hf_ft": hf_ft,
        "hm_ft": hm_ft,
        "h_total_ft": h_total_ft,
        "p_hyd_w": p_hyd_w,
    }


def run_part2(args):
    print("=" * 78)
    print("PART 2: Energy accounting — circulation vs. aeration")
    print("=" * 78)

    hyd = circulation_hydraulics(
        args.loop_gpm,
        args.pipe_size,
        args.pipe_length_ft,
        args.n_turns,
        args.k_per_turn,
        args.roughness_ft,
    )

    print(
        f"Loop: {args.loop_gpm} GPM through {args.pipe_length_ft} ft of "
        f"{args.pipe_size}\" sch40 pipe (ID {hyd['d_in']:.3f} in) + "
        f"{args.n_turns} turns (K={args.k_per_turn} each)"
    )
    print(f"  Velocity: {hyd['v_fps']:.2f} ft/s, Reynolds: {hyd['re']:,.0f}, friction factor f={hyd['f']:.4f}")
    print(f"  Friction head loss (pipe run): {hyd['hf_ft']:.3f} ft")
    print(f"  Minor loss head (turns/fittings): {hyd['hm_ft']:.3f} ft")
    print(f"  Total head to overcome for circulation alone: {hyd['h_total_ft']:.3f} ft")
    print(f"  Hydraulic power needed: {hyd['p_hyd_w']:.2f} W (theoretical, ignoring available hardware)\n")

    p_elec_circ_theoretical_w = hyd["p_hyd_w"] / args.pump_efficiency
    # There is no potable-water-safe pump built to deliver a handful of
    # watts of hydraulic output -- the smallest real circulators (e.g. a
    # small bronze/stainless NSF-61 rated unit like a Taco 006 or Grundfos
    # UP15-10) draw on the order of tens of watts just to spin their motor,
    # regardless of how little head/flow they actually need to produce.
    # So the *real* circulation-only floor is whichever is bigger: the
    # theoretical hydraulic requirement, or the smallest practical pump
    # you could actually install.
    p_elec_circ_w = max(p_elec_circ_theoretical_w, args.min_pump_watts)
    kwh_day_circ = p_elec_circ_w * 24.0 / 1000.0

    if p_elec_circ_w > p_elec_circ_theoretical_w:
        print(
            f"Note: theoretical circulation draw ({p_elec_circ_theoretical_w:.2f} W) is "
            f"below the smallest practical potable-safe pump's floor "
            f"({args.min_pump_watts:.0f} W); using the {args.min_pump_watts:.0f} W floor "
            f"as the realistic circulation baseline.\n"
        )

    total_kwh_day = args.total_kwh_day
    kwh_day_aeration = max(0.0, total_kwh_day - kwh_day_circ)

    print(f"Assumed wire-to-water pump efficiency: {args.pump_efficiency:.0%}")
    print(f"  -> Electrical power for circulation alone: {p_elec_circ_w:.2f} W "
          f"({kwh_day_circ:.3f} kWh/day)")
    print(f"Measured total system draw: {total_kwh_day:.2f} kWh/day "
          f"({total_kwh_day * 1000 / 24:.1f} W average)")
    print(f"  -> Implied aeration surcharge (venturi vacuum + eductor shear + "
          f"losses): {kwh_day_aeration:.3f} kWh/day "
          f"({kwh_day_aeration / total_kwh_day:.1%} of total)\n")

    cost_kwh = args.cost_per_kwh
    circ_cost_day = kwh_day_circ * cost_kwh
    aeration_cost_day = kwh_day_aeration * cost_kwh
    total_cost_day = total_kwh_day * cost_kwh

    print(f"At ${cost_kwh:.2f}/kWh:")
    print(f"  Circulation-only cost:   ${circ_cost_day:.4f}/day  (${circ_cost_day*365:,.2f}/yr)")
    print(f"  Aeration surcharge cost: ${aeration_cost_day:.4f}/day  (${aeration_cost_day*365:,.2f}/yr)")
    print(f"  Total measured cost:     ${total_cost_day:.4f}/day  (${total_cost_day*365:,.2f}/yr)\n")

    if args.calcite_annual_cost is not None:
        print("Fair comparison vs. passive calcite neutralizer:")
        print(f"  Calcite media, ~{args.calcite_gpd:,.0f} gal/day annualized: "
              f"${args.calcite_annual_cost:,.2f}/yr")
        print(f"  Aeration surcharge (the part calcite would eliminate): "
              f"${aeration_cost_day*365:,.2f}/yr")
        print(f"  Circulation cost (you'd likely still want this for mixing, "
              f"even with calcite): ${circ_cost_day*365:,.2f}/yr")
        delta = aeration_cost_day * 365 - args.calcite_annual_cost
        if delta > 0:
            print(f"  -> Calcite saves ~${delta:,.2f}/yr relative to just the "
                  f"aeration-specific electricity (circulation cost is a wash, "
                  f"since you'd keep circulating either way).")
        else:
            print(f"  -> Aeration surcharge alone is already cheaper than calcite "
                  f"by ~${-delta:,.2f}/yr under these assumptions.")
        print()

    if not args.no_plots:
        try:
            import matplotlib
            matplotlib.use("Agg")
            import matplotlib.pyplot as plt
        except ImportError:
            print("[part2] matplotlib not installed; skipping plot (--no-plots to silence)")
            return

        fig, ax = plt.subplots(figsize=(6, 5.5))
        labels = ["Circulation\n(pipe+fittings)", "Aeration surcharge\n(venturi/eductor)"]
        values = [kwh_day_circ, kwh_day_aeration]
        colors = ["#4C72B0", "#C44E52"]
        ax.bar(labels, values, color=colors)
        for i, v in enumerate(values):
            ax.text(i, v + 0.1, f"{v:.2f} kWh/day", ha="center")
        ax.set_ylabel("kWh/day")
        ax.set_title(f"Energy breakdown of measured {total_kwh_day:.1f} kWh/day")
        plt.tight_layout()
        out = args.plot_prefix + "_energy_breakdown.png"
        plt.savefig(out, dpi=150)
        print(f"[part2] wrote {out}")


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def build_arg_parser():
    p = argparse.ArgumentParser(
        description="Back-of-the-envelope CSTR pH model + energy accounting "
        "for a raw-water aeration/circulation loop.",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )

    # Part 1 args
    g1 = p.add_argument_group("Part 1: chemistry / pH model")
    g1.add_argument("--alkalinity", type=float, default=12.0, help="mg/L as CaCO3")
    g1.add_argument("--raw-ph", type=float, default=5.9, help="raw water pH")
    g1.add_argument("--loop-gpm", type=float, default=10.0, help="continuous venturi/eductor loop flow, GPM")
    g1.add_argument("--flow-min", type=float, default=500, help="min raw gal/day for x-axis")
    g1.add_argument("--flow-max", type=float, default=3000, help="max raw gal/day for x-axis")
    g1.add_argument("--flow-step", type=float, default=250, help="gal/day step for table/plot")
    g1.add_argument("--temps", type=float, nargs="+", default=[35, 40, 45, 50, 55, 60, 65, 70],
                     help="tank temperatures (F) to draw as separate lines")
    g1.add_argument("--e0", type=float, default=None,
                     help="single-pass stripping efficiency at Tref (override auto-calibration)")
    g1.add_argument("--k", type=float, default=None,
                     help="efficiency temperature sensitivity, per degF (override auto-calibration)")
    g1.add_argument("--tref-f", type=float, default=55.0, help="reference temp (F) for E0")

    # Part 2 args
    g2 = p.add_argument_group("Part 2: energy accounting")
    g2.add_argument("--pipe-size", choices=list(SCH40_ID_IN.keys()), default="1.5", help="nominal pipe size, inches")
    g2.add_argument("--pipe-length-ft", type=float, default=100.0)
    g2.add_argument("--n-turns", type=int, default=12)
    g2.add_argument("--k-per-turn", type=float, default=0.5, help="minor loss coefficient per simplified turn")
    g2.add_argument("--roughness-ft", type=float, default=5e-6, help="pipe roughness, ft (PVC-ish default)")
    g2.add_argument("--pump-efficiency", type=float, default=0.35, help="assumed wire-to-water pump efficiency")
    g2.add_argument("--min-pump-watts", type=float, default=60.0,
                     help="smallest practical potable-safe circulator's electrical draw, W "
                     "(floor on the circulation baseline; no potable pump exists at a few watts)")
    g2.add_argument("--total-kwh-day", type=float, default=12.0, help="measured total system electricity use")
    g2.add_argument("--cost-per-kwh", type=float, default=0.25)
    g2.add_argument("--calcite-annual-cost", type=float, default=222.0,
                     help="annual calcite media cost to compare against (set to skip with a negative value)")
    g2.add_argument("--calcite-gpd", type=float, default=2740.0,
                     help="approx annualized daily gallons the calcite cost above assumes (1M gal/yr)")

    p.add_argument("--no-plots", action="store_true", help="skip matplotlib plots, print numbers only")
    p.add_argument("--plot-prefix", default="aeration_model", help="output file prefix for plots")
    p.add_argument("--part", choices=["1", "2", "both"], default="both")

    return p


def main(argv=None):
    args = build_arg_parser().parse_args(argv)
    if args.calcite_annual_cost is not None and args.calcite_annual_cost < 0:
        args.calcite_annual_cost = None

    if args.part in ("1", "both"):
        run_part1(args)
    if args.part in ("2", "both"):
        run_part2(args)


if __name__ == "__main__":
    sys.exit(main())
