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
	Write([]byte) error
	Sleep(time.Duration)
}

var _ I2C = &realSleeper{}

func New(i2cPath string, devAddr int) (I2C, error) {
	opener := &i2c.Devfs{
		Dev: i2cPath,
	}
	dev, err := i2c.Open(opener, devAddr)
	if err != nil {
		return nil, err
	}
	return &realSleeper{dev}, nil
}

type realSleeper struct {
	*i2c.Device
}

func (*realSleeper) Sleep(d time.Duration) {
	time.Sleep(d)
}
