// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package influxdbexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// NewFactory creates a factory for InfluxDB exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		component.MustNewType("influxdb"),
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ClientConfig: confighttp.ClientConfig{
			Timeout: 30 * time.Second,
			Headers: map[string]configopaque.String{},
		},
		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      false,
			NumConsumers: 1,
			QueueSize:    0,
		},
		BackOffConfig: configretry.BackOffConfig{
			Enabled: true,
		},
	}
}

func createMetricsExporter(ctx context.Context, set exporter.CreateSettings, config component.Config) (exporter.Metrics, error) {
	cfg := config.(*Config)

	writer, err := newInfluxHTTPWriter(cfg, set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		writer.consumeMetrics,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithStart(writer.Start),
	)
}
