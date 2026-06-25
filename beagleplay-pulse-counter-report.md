# Beagle Play pulse-counter: hardware & software recommendation

> **Status:** recommendation / design note
> **Date:** 2026-06-12
> **Scope note:** This is a Beagle Play (AM62x, Linux) energy-meter pulse-counter
> design. It does **not** use the PRU and is independent of the supruglue PRU
> firmware/framework — it belongs in its own project/repo. This file is left
> unstaged in supruglue only as a scratch artifact to be moved.

## Recommendation (TL;DR)

Count the Leviton meter's pulses with a **plain GPIO interrupt**, isolated by an
**opto-isolated digital-input front-end whose output is a single logic line**
(e.g. MikroElektronika **Opto Click**), driven by the site's **24 V** supply as
the contact-wetting loop:

```
Leviton KYZ          Opto Click (mikroBUS)         Beagle Play
  K (common) ──24V+──► IN+  ─┐
  Y (NO)     ─────────► (in loop)  [opto LED]      isolation barrier
                24V- ──► IN-  ─┘
                              opto output ──► mikroBUS INT ──► gpio1 line
                                                         └─► libgpiod edge count
```

- **Galvanic isolation:** yes (optocoupler).
- **Wetting current:** supplied by the 24 V opto-LED loop (good for a mechanical/relay contact).
- **Software:** `libgpiod` edge counting with kernel debounce — a small (~30-line)
  userspace counter, no SPI, no vendor register protocol.
- **Device-tree change:** none — the mikroBUS INT pin is already a usable `gpio1`
  line on the stock image.

## Background & requirements

- **Sensor:** Leviton Series 100 energy meter, pulse output.
  - **KYZ Form C** (SPDT) **isolated dry contact**.
  - Pulse value **5 Wh** ("on/off") — on a Form-C output this typically means
    **each transition = 5 Wh** (count both edges). **Verify against the meter
    label/datasheet before trusting the kWh math.**
  - Contact rating ~240 VAC / 30 VDC, 0.5 A; **min pulse width ~50 ms** ⇒ ≤ ~10 Hz.
- **Host:** Beagle Play (TI **AM625**, Cortex-A53, arm64), Debian 13.x IoT,
  kernel 6.18.x.
  - **I/O is 3.3 V and NOT 5 V tolerant.**
  - mikroBUS socket present (the natural place for a Click add-on).
- **Application:** totalize pulses → cumulative energy (kWh); a slow, mains-powered,
  always-on telemetry node.

## Key findings that shaped the decision

1. **PRU not needed.** The AM62x *does* have a PRU (pru0/pru1) and the **same
   eCAP/ePWM IP** as am335x (`ti,am3352-ecap`, `ti,am3352-ehrpwm`), plus eQEP. But
   for a slow dry-contact meter, none of that is required — a GPIO interrupt is
   sufficient. This decouples the project from supruglue entirely.
2. **"Poll vs eCAP" was a false comparison.** The recommended GPIO path is
   **interrupt-driven**, not polling. Both GPIO-IRQ and eCAP let the SoC idle
   between edges; eCAP actually adds a small constant draw (its time-base counter
   runs continuously), and neither lets a running Linux box sleep deeper. At
   ≤10 Hz on a ~2 W board, power difference is negligible. Power is not a useful
   axis for this choice.
3. **A dry contact sources no voltage.** Whatever the front-end, something must
   supply the loop voltage and wetting current. Using the site 24 V satisfies this.
4. **Isolation ≠ SPI.** The SPI complexity only appeared because the *industrial*
   8-channel part (DIGI IN Click / MAX22199) serializes its inputs. Isolation for a
   single channel is available as a **logic-line opto output → one GPIO**, with no
   SPI and no custom driver.
5. **Totalizing is a userspace job either way.** Linux has no built-in GPIO
   totalizer (the Counter subsystem needs eCAP/eQEP hardware), so a small counting
   process is unavoidable. The GPIO version is trivial; the SPI version required a
   register/CRC protocol implementation.

## Options considered

| Option | Isolation | Field supply | Host interface | Software | Verdict for one 5 Wh meter |
| --- | --- | --- | --- | --- | --- |
| Bare GPIO + 3.3 V pull-up + RC | none | board 3.3 V | 1 GPIO | libgpiod | Simplest; fine if isolation not required |
| **Opto Click (or discrete opto) → GPIO** | **yes** | **24 V loop** | **1 GPIO (mikroBUS INT)** | **libgpiod** | **Recommended** |
| DIGI IN Click (MAX22199) | yes | 24 V | SPI (`main_spi2`) + IRQ | bespoke SPI/CRC daemon | Robust but overkill: 8-ch, SPI, no mainline driver |
| eCAP2 capture | no (just a pin) | board 3.3 V | Counter subsys (`ti,am62-ecap-capture`) | /dev/counterN | Needs DT compatible change; for *rate*, not totalizing |
| eQEP counter | no | board 3.3 V | Counter subsys | sysfs `count` | HW totalizer, but no isolation; pin-routing dependent |

## Wiring detail (recommended option)

- **Opto Click** accepts **5–24 V** on screw terminals (IN+/IN−), optoisolates,
  and outputs **3.3 V logic on the mikroBUS INT pin** (do **not** feed 24 V to any
  Beagle pin — the opto handles translation).
- Loop: **24 V+ → K (common)**, **Y (NO) → IN+**, **IN− → 24 V−**. Contact closure
  energizes the opto LED (a few mA = wetting current) → INT toggles.
- Optional: also land **Z (NC)** on a second isolated input for a validated/redundant read.
- Optional RC/ESD hardening on the field side for long cable runs.

## Beagle Play platform facts (verified against the v6.18.x device tree)

- mikroBUS **SPI** = `main_spi2`, CS0, already `status = "okay"` (relevant only if
  the SPI/DIGI-IN path were chosen — it is **not**).
- mikroBUS **PWM** pin (B20) = `ECAP2_IN_APWM_OUT`; `&ecap2` is enabled by default
  as a **PWM output** (`ti,am3352-ecap`). Using eCAP as *capture* would require
  switching the node to `ti,am62-ecap-capture`.
- mikroBUS **GPIO/INT/AN** pins are muxed as **`gpio1`** lines
  (`GPIO1_9 / _10 / _12`, mode 7) out of the box — no DT change to use as a GPIO.

## Software approach (recommended option)

- Request the INT `gpio1` line via **libgpiod** with edge detection +
  `GPIO_V2_LINE_FLAG_EDGE_*` and a debounce period of a few ms.
- Block on edge events; per edge add 5 Wh (both edges if Form-C per-transition);
  persist the cumulative total (e.g. periodic flush to disk so it survives reboot).
- Validate wiring with `gpiomon` before writing code.
- Implementation can be a small Go program (libgpiod binding) — same toolchain
  family as other tooling, but it is its own project.

## What we trade away vs the industrial DIGI IN Click

Only the MAX22199 extras: per-channel programmable glitch filters, under/missing
24 V diagnostics, and 7 unused channels. For a single contact these don't justify
an SPI driver. The opto path still isolates and still wets the contact from 24 V.

## Open items / to verify before ordering & deploying

1. **Pulse definition:** confirm 5 Wh is **per transition** vs per open/close cycle
   (decides whether to count one edge or both).
2. **Exact INT line:** trace the Beagle Play mikroBUS INT pin to its precise
   `gpiochip`/line number for libgpiod.
3. **Opto Click input at 24 V:** confirm the board's LED resistor/rating gives an
   acceptable, reliable wetting current at 24 V (a few mA).
4. **Persistence/΅recovery:** decide how the cumulative count survives reboot/power
   loss without losing or double-counting pulses.
5. **Mechanical/EMC:** field-side RC/ESD protection for the cable run.

## References

- Leviton Series 100 pulse output (KYZ Form C, ~50 ms min pulse, 240 VAC/30 VDC, 0.5 A).
- MikroElektronika **Opto Click** (5–24 V opto input → logic output on mikroBUS INT).
- MikroElektronika **DIGI IN Click** (Analog Devices **MAX22199**, 8-ch IEC 61131-2,
  SPI) — considered, not chosen.
- Linux **libgpiod** (GPIO v2 edge events + debounce).
- BeagleBoard-DeviceTrees `v6.18.x`: `k3-am625-beagleplay.dts`, `k3-am62-main.dtsi`.
