package sparkplugreceiver

import (
	"fmt"

	"go.opentelemetry.io/collector/config/confignet"
)

// Config defines configuration for OTel MQTT Sparkplug receiver.
type Config struct {
	Broker BrokerConfig `mapstructure:"broker"`
}

type BrokerConfig struct {
	NetAddr    confignet.NetAddr `mapstructure:",squash"`
	SelfHosted bool              `mapstructure:"self_hosted"`
	HostID     string            `mapstructure:"host_id"`
}

func (c *Config) validate() error {
	if c.Broker.SelfHosted && c.Broker.HostID == "" {
		return fmt.Errorf("hosted primary application host ID not defined")
	}
	return nil
}
