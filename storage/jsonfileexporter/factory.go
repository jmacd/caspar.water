// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fileexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	// The value of "type" key in configuration.
	typeStr = "jsonfile"
)

// NewFactory creates a factory for OTLP exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, component.StabilityLevelAlpha),
		exporter.WithLogs(createLogsExporter, component.StabilityLevelAlpha),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Path: "",
	}
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Metrics, error) {
	fe := &fileExporter{
		logger: &lumberjack.Logger{
			Filename: cfg.(*Config).Path,
		},
	}
	return exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		fe.ConsumeMetrics,
		exporterhelper.WithStart(fe.Start),
		exporterhelper.WithShutdown(fe.Shutdown),
	)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Logs, error) {
	// TODO: Danger!
	// This shoud be using a shared component mechanism to ensure that
	// multiple pipelines do not clobber each other.
	// TODO: replace with https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/fileexporter
	fe := &fileExporter{
		logger: &lumberjack.Logger{
			Filename: cfg.(*Config).Path,
		},
	}
	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		fe.ConsumeLogs,
		exporterhelper.WithStart(fe.Start),
		exporterhelper.WithShutdown(fe.Shutdown),
	)
}
