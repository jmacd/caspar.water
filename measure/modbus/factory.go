package modbus

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr = "modbus"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		DefaultConfig,
		receiver.WithMetrics(createMetrics, component.StabilityLevelAlpha),
	)
}

// DefaultConfig creates the default configuration for receiver.
func DefaultConfig() component.Config {
	return &Config{
		URL:      "rtu:///dev/ttyUSB0",
		Interval: time.Minute,
		Baud:     19200,
		DataBits: 8,
		StopBits: 1,
		Parity:   "even",
		Timeout:  time.Millisecond * 300,
	}
}

// createMetrics creates a metrics receiver based on provided config.
func createMetrics(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	oCfg := cfg.(*Config)
	return newModbusReceiver(oCfg, set, consumer)
}
