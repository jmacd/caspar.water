# Setting up the Atlas Scientific pH probe

The EZO pH probe receiver arrives, out of the box, in UART mode and we
need it to be in i2c mode for the arrangement in this repository.

The data sheet indicates a procedure for manually changing from UART
to i2c mode, by shorting the `Tx` and `Pgnd` pins during power-up,
while `Rx` is disconnected.  The unit must be disconnected from the
isolator board and requires a jumpers and possibly a breadboard.

Another way to setup the new EZO pH probe receiver is to connect a USB
to TTL serial cable, of the sort used for serial debugging.  My
adapter has four connections, which can be attached to the EZO unit as
follows:


| USB-to-TTL pin color (name) | EZO pH probe pin name |
|-----------------------------|-----------------------|
| Red (Vcc)                   | Vcc                   |
| Black  (Gnd)                | Gnd                   |
| Yellow (Rx)                 | Tx                    |
| Green (Tx)                  | Rx                    |

Make these connections and plug the USB device into any computer.  On
a Linux machine, locate the serial device (e.g., `/dev/ttyUSB0`), then
execute:

```
echo i2c,99 > /dev/ttyUSB0
```

The EZO unit LED will change from green (flashing cyan, one a 1s
interval, for each continuous measurement) to solid blue.  Unplug the
EZO unit from the USB-to-TTL device and connect it to an i2c bus.

The EZO unit will power up in i2C mode.
