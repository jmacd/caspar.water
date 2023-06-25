// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package influxdbexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter"

import (
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for the InfluxDB exporter.
type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	exporterhelper.QueueSettings  `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings  `mapstructure:"retry_on_failure"`

	// Org is the InfluxDB organization name of the destination bucket.
	Org string `mapstructure:"org"`
	// Bucket is the InfluxDB bucket name that telemetry will be written to.
	Bucket string `mapstructure:"bucket"`
	// Token is used to identify InfluxDB permissions within the organization.
	Token configopaque.String `mapstructure:"token"`
}

func (cfg *Config) Validate() error {
	return nil
}
