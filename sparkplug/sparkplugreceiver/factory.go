package sparkplugreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr             = "sparkplug"
	defaultBindEndpoint = "localhost:1883"
	defaultTransport    = "tcp"
)

// NewFactory creates a factory for the sparkplug receiver.
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(createMetricsReceiver, component.StabilityLevelAlpha),
	)
}

func createDefaultConfig() receiver.CreateSettings {
	return &Config{
		Broker: BrokerConfig{
			NetAddr: confignet.NetAddr{
				Endpoint:  defaultBindEndpoint,
				Transport: defaultTransport,
			},
		},
	}
}

func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	cfg receiver.CreateSettings,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	c := cfg.(*Config)
	err := c.validate()
	if err != nil {
		return nil, err
	}
	return New(params, *c, consumer)
}
