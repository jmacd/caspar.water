package sparkplugreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
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

func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
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
	cfg config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	c := cfg.(*Config)
	err := c.validate()
	if err != nil {
		return nil, err
	}
	return New(params, *c, consumer)
}
