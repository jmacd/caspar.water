package atlasph

import (
	"context"
	"time"

	"github.com/jmacd/caspar.water/measure/ph/atlasph/internal/ezo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr = "atlasph"
)

// NewFactory creates a new OTLP receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetrics, component.StabilityLevelAlpha),
	)
}

// createDefaultConfig creates the default configuration for receiver.
func createDefaultConfig() component.Config {
	return &Config{
		Device:         ezo.DefaultDevice,
		I2CAddr:        ezo.DefaultAddress,
		Prefix:         "atlasph",
		Interval:       time.Minute,
		ReferenceTempC: 15,
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
	return newPhReceiver(oCfg, set, consumer)
}
