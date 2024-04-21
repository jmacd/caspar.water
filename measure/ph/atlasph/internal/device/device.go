package device

import (
	"bytes"
	"time"

	"golang.org/x/exp/io/i2c"
)

const (
	millis = time.Millisecond

	Short = 500 * millis  // longer than stated
	Long  = 1100 * millis // longer than stated

	StatusOK byte = 1
)

type I2CStringer interface {
	Close() error
	WriteSleepRead(cmd string, delay time.Duration) (byte, string, error)
}

type I2C interface {
	Close() error
	Read([]byte) error
	Write([]byte) error
	Sleep(time.Duration)
}

var _ I2C = &realDevice{}

func New(i2cPath string, devAddr int) (I2CStringer, error) {
	opener := &i2c.Devfs{
		Dev: i2cPath,
	}
	dev, err := i2c.Open(opener, devAddr)
	if err != nil {
		return nil, err
	}
	return &writeSleepReader{
		device: &realDevice{
			device: dev,
		},
	}, nil
}

type realDevice struct {
	device *i2c.Device
}

func (r *realDevice) Write(d []byte) error {
	return r.device.Write(d)
}

func (r *realDevice) Close() error {
	return r.device.Close()
}

func (r *realDevice) Read(d []byte) error {
	return r.device.Read(d)
}

func (r *realDevice) Sleep(d time.Duration) {
	time.Sleep(d)
}

type writeSleepReader struct {
	device I2C
}

func (r *writeSleepReader) Close() error {
	return r.device.Close()
}

func (r *writeSleepReader) WriteSleepRead(cmd string, delay time.Duration) (byte, string, error) {
	var buf [64]byte
	if err := r.device.Write([]byte(cmd)); err != nil {
		return 0, "", err
	}

	r.device.Sleep(delay)

	if err := r.device.Read(buf[:]); err != nil {
		return 0, "", err
	}
	nz, _, _ := bytes.Cut(buf[:], []byte{0})
	return nz[0], string(nz[1:]), nil
}
