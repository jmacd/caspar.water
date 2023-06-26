// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package influxdbexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"

	"github.com/influxdata/line-protocol/v2/lineprotocol"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type influxHTTPWriter struct {
	httpClient *http.Client

	httpClientSettings confighttp.HTTPClientSettings
	telemetry          component.TelemetrySettings
	writeURL           string
}

func newInfluxHTTPWriter(config *Config, telemetrySettings component.TelemetrySettings) (*influxHTTPWriter, error) {
	writeURL, err := composeWriteURL(config)
	if err != nil {
		return nil, err
	}

	return &influxHTTPWriter{
		httpClientSettings: config.HTTPClientSettings,
		telemetry:          telemetrySettings,
		writeURL:           writeURL,
	}, nil
}

func composeWriteURL(config *Config) (string, error) {
	writeURL, err := url.Parse(config.HTTPClientSettings.Endpoint)
	if err != nil {
		return "", err
	}
	if writeURL.Path == "" || writeURL.Path == "/" {
		writeURL, err = writeURL.Parse("api/v2/write")
		if err != nil {
			return "", err
		}
	}
	org := config.Org
	bucket := config.Bucket
	token := config.Token

	if org == "" {
		org = os.Getenv("INFLUX_ORG")
	}
	if bucket == "" {
		bucket = os.Getenv("INFLUX_BUCKET")
	}
	if token == "" {
		token = configopaque.String(os.Getenv("INFLUX_TOKEN"))
	}

	queryValues := writeURL.Query()
	queryValues.Set("precision", "ns")
	queryValues.Set("org", org)
	queryValues.Set("bucket", bucket)

	if token != "" {
		if config.HTTPClientSettings.Headers == nil {
			config.HTTPClientSettings.Headers = map[string]configopaque.String{}
		}
		config.HTTPClientSettings.Headers["Authorization"] = "Token " + token
	}

	writeURL.RawQuery = queryValues.Encode()

	return writeURL.String(), nil
}

// Start implements component.StartFunc
func (w *influxHTTPWriter) Start(_ context.Context, host component.Host) error {
	httpClient, err := w.httpClientSettings.ToClient(host, w.telemetry)
	if err != nil {
		return err
	}
	w.httpClient = httpClient

	if err = w.ping(); err != nil {
		w.telemetry.Logger.Warn("influxdb ping failed", zap.Error(err))
	}
	return nil
}

type lineKey struct {
	set attribute.Set
	ts  pcommon.Timestamp
}

type lineValue struct {
	metric string
	value  lineprotocol.Value
}

func mapToKVs(m pcommon.Map) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	m.Range(func(k string, v pcommon.Value) bool {
		attrs = append(attrs, attribute.String(k, v.AsString()))
		return true
	})
	return attrs
}

func pointValue(pt pmetric.NumberDataPoint) lineprotocol.Value {
	switch pt.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		v, _ := lineprotocol.FloatValue(pt.DoubleValue())
		return v
	case pmetric.NumberDataPointValueTypeInt:
		return lineprotocol.IntValue(pt.IntValue())
	default:
		v, _ := lineprotocol.StringValue("undefined")
		return v
	}
}

func joinAttrs(lists ...[]attribute.KeyValue) attribute.Set {
	var flat []attribute.KeyValue
	for _, l := range lists {
		flat = append(flat, l...)
	}
	return attribute.NewSet(flat...)
}

func (w *influxHTTPWriter) ping() error {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.writeURL, bytes.NewReader(nil))
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	return w.doHTTP(req, w.writeURL)
}

func (w *influxHTTPWriter) doHTTP(req *http.Request, path string) error {
	if res, err := w.httpClient.Do(req); err != nil {
		return err
	} else if body, err := io.ReadAll(res.Body); err != nil {
		return err
	} else if err = res.Body.Close(); err != nil {
		return err
	} else {
		switch res.StatusCode / 100 {
		case 2: // Success
			break
		case 5: // Retryable error
			return fmt.Errorf("influxdb %s returned %q %q", path, res.Status, string(body))
		default: // Terminal error
			return consumererror.NewPermanent(fmt.Errorf("influx %s returned %q %q", path, res.Status, string(body)))
		}
	}

	return nil
}

func nameOf(m pmetric.Metric) string {
	name := m.Name()
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		name += "_gauge"
	case pmetric.MetricTypeSum:
		if m.Sum().IsMonotonic() {
			name += "_counter"
		} else {
			name += "_updowncounter"
		}
	}

	if m.Unit() != "" {
		name += "_"
		name += m.Unit()
	}
	return name
}

func (w *influxHTTPWriter) consumeMetrics(ctx context.Context, ld pmetric.Metrics) error {
	var enc lineprotocol.Encoder
	enc.SetPrecision(lineprotocol.Nanosecond)
	enc.SetLax(false)

	for ri := 0; ri < ld.ResourceMetrics().Len(); ri++ {
		rm := ld.ResourceMetrics().At(ri)
		rattrs := mapToKVs(rm.Resource().Attributes())

		for si := 0; si < rm.ScopeMetrics().Len(); si++ {
			sm := rm.ScopeMetrics().At(si)
			sattrs := mapToKVs(sm.Scope().Attributes())

			allPoints := map[lineKey][]lineValue{}

			for mi := 0; mi < sm.Metrics().Len(); mi++ {
				m := sm.Metrics().At(mi)

				eachPoint := func(pts pmetric.NumberDataPointSlice) {
					for pi := 0; pi < pts.Len(); pi++ {
						pt := pts.At(pi)
						lkey := lineKey{
							set: joinAttrs(rattrs, sattrs, mapToKVs(pt.Attributes())),
							ts:  pt.Timestamp(),
						}
						allPoints[lkey] = append(allPoints[lkey], lineValue{
							metric: nameOf(m),
							value:  pointValue(pt),
						})
					}
				}

				switch m.Type() {
				case pmetric.MetricTypeGauge:
					eachPoint(m.Gauge().DataPoints())

				case pmetric.MetricTypeSum:
					if m.Sum().AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
						return consumererror.NewPermanent(fmt.Errorf("delta temporality not supported"))
					}
					eachPoint(m.Sum().DataPoints())
				default:
					return consumererror.NewPermanent(fmt.Errorf("unsupported metric type"))
				}
			}

			for key, fields := range allPoints {
				enc.StartLine(sm.Scope().Name())

				for iter := key.set.Iter(); iter.Next(); {
					_, kv := iter.IndexedAttribute()
					enc.AddTag(string(kv.Key), kv.Value.AsString())
				}

				sort.Slice(fields, func(i, j int) bool {
					return fields[i].metric < fields[j].metric
				})

				for _, field := range fields {
					enc.AddField(field.metric, field.value)
				}

				enc.EndLine(key.ts.AsTime())

				if err := enc.Err(); err != nil {
					return consumererror.NewPermanent(fmt.Errorf("failed to encode point: %w", err))
				}
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.writeURL, bytes.NewReader(enc.Bytes()))
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	return w.doHTTP(req, w.writeURL)
}
