package serialreceiver

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the serial receiver.
type Config struct {
	// e.g., "/dev/ttyUSB0"
	Device string `mapstructure:"device"`

	Baud int `mapstructure:"baud"`
}

var _ component.Config = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Device == "" {
		return fmt.Errorf("empty device name")
	}
	if cfg.Baud < 50 || cfg.Baud > 4000000 {
		return fmt.Errorf("baud rate invalid: %d", cfg.Baud)
	}

	return nil
}
