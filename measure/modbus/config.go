package modbus

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

type Field struct {
	Name  string `mapstructure:"name"`
	Base  uint16 `mapstructure:"base"`
	Type  string `mapstructure:"type"`
	Range string `mapstructure:"range"`
}

type Metric struct {
	Field `mapstructure:",squash"`
	Unit  string `mapstructure:"unit"`
}

type Attribute struct {
	Field `mapstructure:",squash"`
}

// Config defines configuration for the modbus receiver.
type Config struct {
	// e.g. "rtu:///dev/ttyUSB0"
	URL        string        `mapstructure:"url"`
	Interval   time.Duration `mapstructure:"interval"`
	Prefix     string        `mapstructure:"prefix"`
	Metrics    []Metric      `mapstructure:"metrics"`
	Attributes []Attribute   `mapstructure:"attributes"`
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
	if cfg.Prefix == "" {
		return fmt.Errorf("prefix is empty")
	}
	for _, attr := range cfg.Attributes {
		if err := attr.check(); err != nil {
			return err
		}
	}
	for _, metric := range cfg.Metrics {
		if err := metric.check(); err != nil {
			return err
		}
	}
	return nil
}

func (f Field) check() error {
	if f.Name == "" {
		return fmt.Errorf("name is empty")
	}
	if f.Base < 1 || f.Base > 9999 {
		return fmt.Errorf("0 < base < 10000")
	}
	switch f.Range {
	case "coil", "discrete", "input", "holding":
	default:
		return fmt.Errorf("unknown range: %q", f.Range)
	}
	switch f.Type {
	case "float32", "uint32", "bool":
	default:
		return fmt.Errorf("unknown type: %q", f.Type)
	}
	return nil
}
