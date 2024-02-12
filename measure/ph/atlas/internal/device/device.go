package device

import (
	"time"

	"golang.org/x/exp/io/i2c"
)

const (
	millis = time.Millisecond

	Retry = 5 * millis
	Short = 300 * millis
	Long  = 900 * millis
)

type I2C interface {
	Close() error
	Read([]byte) error
	Write(string) error
	Sleep(time.Duration)
}

var _ I2C = &stringDevice{}

func New(i2cPath string, devAddr int) (I2C, error) {
	opener := &i2c.Devfs{
		Dev: i2cPath,
	}
	dev, err := i2c.Open(opener, devAddr)
	if err != nil {
		return nil, err
	}
	return &stringDevice{dev}, nil
}

type stringDevice struct {
	device *i2c.Device
}

func (r *stringDevice) Write(s string) error {
	return r.device.Write([]byte(s))
}

func (r *stringDevice) Close() error {
	return r.device.Close()
}

func (r *stringDevice) Read(d []byte) error {
	return r.device.Read(d)
}

func (r *stringDevice) Sleep(d time.Duration) {
	time.Sleep(d)
}
