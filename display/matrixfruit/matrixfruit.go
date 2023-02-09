// https://learn.adafruit.com/usb-plus-serial-backpack/command-reference
package matrixfruit

import (
	"os"

	"log"
)

func Main() {
	f, err := os.OpenFile("/dev/ttyACM0", os.O_RDWR, 0)
	if err != nil {
		log.Println("open", err)
	}
	_, err = f.Write([]byte{
		// For a 2x16
		0xFE,
		0xD1,
		16,
		2,

		// set background
		0xFE,
		0xD0,
		0x33,
		0x55,
		0xFF,

		// clear screen
		0xFE,
		0x58,

		// go home
		0xFE,
		0x48,

		// autoscroll
		0xFE,
		0x51,

		// setpos
		0xFE,
		0x47,
		1, 1,

		'h',
		'e',
		'l',
		'l',
		'o',
		'5',
		'6',
		'7',
		'8',
		'9',
		'0',
		'1',
		'2',
		'3',
		'4',
		'5',

		// setpos
		0xFE,
		0x47,
		1, 2,

		'm',
		'o',
		'r',
		'e',
		'4',
		'5',
		'6',
		'7',
		'8',
		'9',
		'0',
		'1',
		'2',
		'3',
		'4',
	})
	if err != nil {
		log.Println("write", err)
	}

}
