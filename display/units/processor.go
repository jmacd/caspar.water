package units

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type unitsProcessor struct {
	cfg *Config
}

func newUnitsProcessor(config *Config) *unitsProcessor {
	return &unitsProcessor{
		cfg: config,
	}
}

func (*unitsProcessor) Start(context.Context, component.Host) error {
	return nil
}

func (p *unitsProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	resourceMetricsSlice := md.ResourceMetrics()

	for i := 0; i < resourceMetricsSlice.Len(); i++ {
		rm := resourceMetricsSlice.At(i)
		ilms := rm.ScopeMetrics()
		for i := 0; i < ilms.Len(); i++ {
			ilm := ilms.At(i)
			metricSlice := ilm.Metrics()
			for j := 0; j < metricSlice.Len(); j++ {
				metric := metricSlice.At(j)
				var repl *Replace
				for _, r := range p.cfg.Replace {
					if r.Input != metric.Unit() {
						continue
					}
					repl = &r
					break
				}
				if repl == nil {
					continue
				}
				metric.SetUnit(repl.Output)
				dps := metric.Gauge().DataPoints()

				for i := 0; i < dps.Len(); i++ {
					pt := dps.At(i)
					pt.SetDoubleValue(pt.DoubleValue() * repl.Conversion)
				}
			}
		}
	}
	return md, nil
}

func (*unitsProcessor) Shutdown(context.Context) error {
	return nil
}
