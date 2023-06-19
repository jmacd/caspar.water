package modbus

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the modbus receiver.
type Config struct {
	URL      string        `mapstructure:"url"`
	Interval time.Duration `mapstructure:"interval"`
}

var _ component.Config = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	if cfg.URL == "" {
		return fmt.Errorf("empty URL")
	}
	if cfg.Interval < 50*time.Millisecond {
		return fmt.Errorf("interval is too short")
	}
	return nil
}
