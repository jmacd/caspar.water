package openlcd

import (
	"time"

	"golang.org/x/exp/io/i2c"
)

var rowOffsets = []byte{
	0x00, 0x40, 0x14, 0x54,
}

type OpenLCD struct {
	device *i2c.Device
}

func New(i2cPath string, devAddr int) (*OpenLCD, error) {
	opener := &i2c.Devfs{
		Dev: i2cPath,
	}
	device, err := i2c.Open(opener, devAddr)
	if err != nil {
		return nil, err
	}

	return &OpenLCD{
		device: device,
	}, nil
}

func (lcd *OpenLCD) On() error {
	defer time.Sleep(1 * time.Millisecond)
	return lcd.device.Write([]byte{
		// SETTING_COMMAND, SET_RGB_COMMAND, R, G, B
		0x7C, 0x2B, 0xff, 0xff, 0xff,

		// SPECIAL_COMMAND, DISPLAYCONTROL|LCD_DISPLAYON
		254, 0x8 | 0x4,
	})
}

func (lcd *OpenLCD) Off() error {
	defer time.Sleep(1 * time.Millisecond)
	return lcd.device.Write([]byte{
		// SPECIAL_COMMAND, DISPLAYCONTROL|LCD_DISPLAYOFF
		254, 0x8 | 0x0,

		// SETTING_COMMAND, SET_RGB_COMMAND, R, G, B
		0x7C, 0x2B, 0, 0, 0,
	})
}

func (lcd *OpenLCD) Update(str string) error {
	for _, c := range []byte(str) {
		if err := lcd.write([]byte{c}); err != nil {
			return err
		}
	}
	return nil
}

func (lcd *OpenLCD) write(d []byte) error {
	defer time.Sleep(1 * time.Millisecond)
	return lcd.device.Write(d)
}

func (lcd *OpenLCD) Clear() error {
	defer time.Sleep(1 * time.Millisecond)
	return lcd.device.Write([]byte{0x7c, 0x2d})
}
