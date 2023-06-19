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

func (lcd *OpenLCD) Update(str string) error {
	for _, c := range []byte(str) {
		if err := lcd.write([]byte{c}); err != nil {
			return err
		}
	}
	for i := len(str); i < 20; i++ {
		if err := lcd.write([]byte{' '}); err != nil {
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
