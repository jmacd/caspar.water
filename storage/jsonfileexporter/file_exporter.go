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
	"io"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"gopkg.in/natefinch/lumberjack.v2"
)

var metricsMarshaler = pmetric.JSONMarshaler{}
var logsMarshaler = plog.JSONMarshaler{}

// fileExporter is the implementation of file exporter that writes telemetry data to a file
// in Protobuf-JSON format.
type fileExporter struct {
	mutex  sync.Mutex
	logger *lumberjack.Logger
}

func (e *fileExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *fileExporter) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	buf, err := metricsMarshaler.MarshalMetrics(md)
	if err != nil {
		return err
	}
	return exportMessageAsLine(e, buf)
}

func (e *fileExporter) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	buf, err := logsMarshaler.MarshalLogs(ld)
	if err != nil {
		return err
	}
	return exportMessageAsLine(e, buf)
}

func exportMessageAsLine(e *fileExporter, buf []byte) error {
	// Ensure only one write operation happens at a time.
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if _, err := e.logger.Write(buf); err != nil {
		return err
	}
	if _, err := io.WriteString(e.logger, "\n"); err != nil {
		return err
	}
	return nil
}

func (e *fileExporter) Start(context.Context, component.Host) error {
	return nil
}

// Shutdown stops the exporter and is invoked during shutdown.
func (e *fileExporter) Shutdown(context.Context) error {
	return e.logger.Close()
}
