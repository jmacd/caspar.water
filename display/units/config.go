package units

import "fmt"

type Replace struct {
	Input      string  `mapstructure:"input"`
	Output     string  `mapstructure:"output"`
	Conversion float64 `mapstructure:"conversion"`
}

type Config struct {
	Replace []Replace `mapstructure:"replace"`
}

func (cfg *Config) Validate() error {
	for _, r := range cfg.Replace {
		if r.Input == "" {
			return fmt.Errorf("input unit is empty")
		}
		if r.Output == "" {
			return fmt.Errorf("output unit is empty")
		}
		if r.Conversion <= 0 {
			return fmt.Errorf("conversion is <= 0")
		}
	}

	return nil
}
