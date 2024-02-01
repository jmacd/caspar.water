// https://files.atlas-scientific.com/pH_EZO_Datasheet.pdf
package atlas

import (
	"fmt"
	"time"

	"golang.org/x/exp/io/i2c"
)

type Ph struct {
	device *i2c.Device
}

type Measurements struct {
	Ph float64
}

func New(i2cPath string, devAddr int) (*Ph, error) {
	opener := &i2c.Devfs{
		Dev: i2cPath,
	}
	device, err := i2c.Open(opener, devAddr)
	if err != nil {
		return nil, err
	}
	ph := &Ph{
		device: device,
	}
	if err = device.Write([]byte("i")); err != nil {
		return nil, err
	}

	ph.wait()

	var info [11]byte
	if err := ph.device.Read(info[:]); err != nil {
		return nil, err
	}
	fmt.Println("read", info)
	return ph, nil
}

func (ph *Ph) Close() error {
	return ph.device.Close()
}

func (ph *Ph) Read() (Measurements, error) {
	return Measurements{}, nil
}

func (ph *Ph) wait() error {
	for n := 0; n < 30; n++ {
		var status [12]byte
		if err := ph.device.Read(status[:]); err != nil {
			return err
		}
		switch status[0] {
		case 255:
			// No data
			return fmt.Errorf("No data to read")
		case 254:
			// Processing
			continue
		case 2:
			// Syntax
			return fmt.Errorf("Command syntax error")
		case 1:
			// OK
			fmt.Println("I HAVE", string(status[1:]))
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}
