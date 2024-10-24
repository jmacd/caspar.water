package modbus

import (
	"fmt"
	"strings"
	"time"

	"github.com/simonvetter/modbus"
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
	Kind  string `mapstructure:"kind"`
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

	Baud     uint          `mapstructure:"baud"`
	DataBits uint          `mapstructure:"data_bits"`
	StopBits uint          `mapstructure:"stop_bits"`
	Parity   string        `mapstructure:"parity"`
	Timeout  time.Duration `mapstructure:"timeout"`
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
	_, err := parityFromString(cfg.Parity)
	if err != nil {
		return err
	}
	return nil
}

func parityFromString(p string) (uint, error) {
	switch strings.ToLower(p) {
	case "even":
		return modbus.PARITY_EVEN, nil
	case "odd":
		return modbus.PARITY_ODD, nil
	case "none":
		return modbus.PARITY_NONE, nil
	}
	return 0, fmt.Errorf("invalid parity: %s", p)
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
	case "float32", "uint32", "bool", "uint16":
	default:
		return fmt.Errorf("unknown type: %q", f.Type)
	}
	return nil
}

func (m Metric) check() error {
	if err := m.Field.check(); err != nil {
		return err
	}
	switch m.Kind {
	case "counter", "gauge":
		return nil
	default:
		return fmt.Errorf("unknown kind: %q", m.Kind)
	}
}
