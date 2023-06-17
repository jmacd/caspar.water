package openlcd

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
)

type Config struct {
	// e.g., /dev/i2c-0
	Device string `mapstructure:"device"`

	// e.g., 0x72
	I2CAddr uint8 `mapstructure:"i2c_addr"`

	Metrics []string `mapstructure:"metrics"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	if cfg.Device == "" {
		return fmt.Errorf("empty device name")
	}
	if len(cfg.Metrics) == 0 {
		return fmt.Errorf("empty metrics list")
	}
	return nil
}
