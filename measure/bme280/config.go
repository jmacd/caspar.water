package bme280

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the bme280 receiver.
type Config struct {
	// e.g., "/dev/i2c-5"
	Device string `mapstructure:"device"`

	// e.g., 0x77
	I2CAddr uint8 `mapstructure:"i2c_addr"`

	// e.g., nameofsensor
	Prefix string `mapstructure:"prefix"`

	// measurement interval
	Interval time.Duration `mapstructure:"interval"`
}

var _ component.Config = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Device == "" {
		return fmt.Errorf("empty device name")
	}
	if cfg.Prefix == "" {
		return fmt.Errorf("empty prefix name")
	}
	if cfg.I2CAddr > 128 {
		return fmt.Errorf("i2c address out of range")
	}
	if cfg.Interval < 50*time.Millisecond {
		return fmt.Errorf("interval is too short")
	}

	return nil
}
