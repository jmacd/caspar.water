package currentloop

import (
	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for OTLP receiver.
type Config struct {
}

var _ component.Config = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	return nil
}
