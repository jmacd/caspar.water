package openlcd

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

type Pair struct {
	Metric string `mapstructure:"metric"`
	Abbrev string `mapstructure:"abbrev"`
}

type Config struct {
	// e.g., /dev/i2c-0
	Device string `mapstructure:"device"`

	// e.g., 0x72
	I2CAddr uint8 `mapstructure:"i2c_addr"`

	Rows int    `mapstructure:"rows"`
	Cols int    `mapstructure:"cols"`
	Show []Pair `mapstructure:"show"`

	RunFor time.Duration `mapstructure:"run_for"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	if cfg.Device == "" {
		return fmt.Errorf("empty device name")
	}
	if cfg.Rows == 0 {
		return fmt.Errorf("rows can't be zero")
	}
	if cfg.Cols == 0 {
		return fmt.Errorf("cols can't be zero")
	}
	switch {
	case len(cfg.Show) == 0:
		return fmt.Errorf("empty metrics list")
	case len(cfg.Show) > cfg.Rows:
		return fmt.Errorf("too many metrics")
	}
	if cfg.RunFor < 0 {
		return fmt.Errorf("negative run_for")
	}
	return nil
}
