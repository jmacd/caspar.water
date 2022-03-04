package otlp

import (
	otlpcommon "go.opentelemetry.io/proto/otlp/common/v1"
)

type (
	SparkplugID struct {
		GroupID    string
		EdgeNodeID string
		DeviceID   string
	}

	DeviceMap map[SparkplugID]Device

	Device struct {
		AliasMap  AliasMap
		MetricMap MetricMap
	}

	AliasMap map[string]uint64

	MetricMap map[uint64]*Metric

	Metric struct {
		Name           string
		StartTimestamp uint64
		Timestamp      uint64
		Description    string
		Value          otlpcommon.AnyValue
	}
)

func (dm DeviceMap) Define(id SparkplugID, name string, alias, ts uint64, desc string) *Metric {
	dev, ok := dm[id]

	if !ok {
		dev = Device{
			AliasMap:  AliasMap{},
			MetricMap: MetricMap{},
		}
		dm[id] = dev
	}

	dev.AliasMap[name] = alias

	existMetric, ok := dev.MetricMap[alias]

	if !ok {
		existMetric = &Metric{
			Name:           name,
			StartTimestamp: ts,
			Description:    desc,
		}
		dev.MetricMap[alias] = existMetric
	}
	return existMetric
}

func (dm DeviceMap) Lookup(id SparkplugID, alias uint64) *Metric {
	dev, ok := dm[id]

	if !ok {
		return nil
	}

	return dev.MetricMap[alias]
}
