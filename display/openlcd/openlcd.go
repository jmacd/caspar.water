package openlcd

import (
	"fmt"
	"time"

	"golang.org/x/exp/io/i2c"
)

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
		if err := lcd.device.Write([]byte{c}); err != nil {
			fmt.Println("Error:", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}
