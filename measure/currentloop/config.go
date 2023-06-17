package currentloop

import (
	"fmt"
	"math"
	"time"

	"go.opentelemetry.io/collector/component"
)

const (
	minCurrent = 0.004 // 4mA
	maxCurrent = 0.020 // 20mA
)

// Config defines configuration for the currentloop receiver.
type Config struct {
	// e.g. /sys/bus/iio/devices/iio:device0/in_voltage0_raw

	Device   string        `mapstructure:"device"`
	Interval time.Duration `mapstructure:"interval"`

	Name string `mapstructure:"name"`
	Unit string `mapstructure:"unit"`

	Multiply float64 `mapstructure:"multiply"` // Max volts
	Divide   float64 `mapstructure:"divide"`   // Max scaled reading
	Ohms     float64 `mapstructure:"omhs"`     // Resistor

	Min float64 `mapstructure:"min"`
	Max float64 `mapstructure:"max"`
}

var _ component.Config = (*Config)(nil)

func validFloat(x float64) bool {
	return !math.IsInf(x, 0) && !math.IsInf(x, 0)
}

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Device == "" {
		return fmt.Errorf("empty device name")
	}
	if cfg.Name == "" {
		return fmt.Errorf("empty metric name")
	}
	if !validFloat(cfg.Min) {
		return fmt.Errorf("invalid min")
	}
	if !validFloat(cfg.Max) {
		return fmt.Errorf("invalid max")
	}
	if !validFloat(cfg.Multiply) {
		return fmt.Errorf("invalid multiply")
	}
	if !validFloat(cfg.Divide) {
		return fmt.Errorf("invalid divide")
	}
	if !validFloat(cfg.Ohms) {
		return fmt.Errorf("invalid ohms")
	}
	if cfg.Min >= cfg.Max {
		return fmt.Errorf("min >= max")
	}
	if cfg.Interval < 50*time.Millisecond {
		return fmt.Errorf("interval is too short")
	}

	return nil
}
