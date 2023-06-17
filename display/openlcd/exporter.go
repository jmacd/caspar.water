package openlcd

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type openLCDExporter struct {
	display  *os.File
	config   *Config
	defs     []pmetric.Metric
	current  []interface{} // a point type
	name2idx map[string]int
	olcd     *OpenLCD
}

func newOpenLCDExporter(cfg *Config, set exporter.CreateSettings) (*openLCDExporter, error) {
	f, err := os.OpenFile(cfg.Device, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open device: %s: %w", cfg.Device, err)
	}
	n2i := map[string]int{}
	cur := make([]interface{}, len(cfg.Metrics))
	defs := make([]pmetric.Metric, len(cfg.Metrics))
	for idx, mc := range cfg.Metrics {
		n2i[mc] = idx
		defs[idx] = pmetric.NewMetric()
	}

	olcd, err := New(cfg.Device, int(cfg.I2CAddr))
	if err != nil {
		return nil, err
	}

	return &openLCDExporter{
		display:  f,
		config:   cfg,
		current:  cur,
		defs:     defs,
		name2idx: n2i,
		olcd:     olcd,
	}, err
}

func (e *openLCDExporter) pushMetrics(_ context.Context, md pmetric.Metrics) error {
	for ri := 0; ri < md.ResourceMetrics().Len(); ri++ {
		rm := md.ResourceMetrics().At(ri)

		for si := 0; si < rm.ScopeMetrics().Len(); si++ {
			sm := rm.ScopeMetrics().At(si)

			for mi := 0; mi < sm.Metrics().Len(); mi++ {
				m := sm.Metrics().At(mi)

				idx, ok := e.name2idx[m.Name()]
				if !ok {
					continue
				}
				switch m.Type() {
				case pmetric.MetricTypeEmpty:
					e.current[idx] = nil

				case pmetric.MetricTypeGauge:
					dp := m.Gauge().DataPoints()
					e.current[idx] = dp.At(dp.Len() - 1)
					e.defs[idx].SetUnit(m.Unit())
					e.defs[idx].SetDescription(m.Description())

				case pmetric.MetricTypeSum,
					pmetric.MetricTypeHistogram,
					pmetric.MetricTypeExponentialHistogram,
					pmetric.MetricTypeSummary:
					panic("unimplemented")
				}
			}
		}
	}

	return e.export()
}

func (e *openLCDExporter) line(n int) string {
	value := e.current[n]

	vstr := "<unset>"
	if value != nil {
		switch t := value.(type) {
		case pmetric.NumberDataPoint:
			switch t.ValueType() {
			case pmetric.NumberDataPointValueTypeDouble:
				vstr = fmt.Sprintf("%f", t.DoubleValue())
			case pmetric.NumberDataPointValueTypeInt:
				vstr = fmt.Sprint(t.IntValue())
			case pmetric.NumberDataPointValueTypeEmpty:
			}
		default:
			panic("unhandled")
		}
	}
	ustr := "<undef>"
	if e.defs[n].Unit() != "" {
		ustr = e.defs[n].Unit()
	}

	line := fmt.Sprint(vstr, " ", ustr)
	return line
}

func (e *openLCDExporter) export() error {
	var send []byte
	send = append(send, []byte(e.line(0))...)

	if len(e.current) > 1 {
		send = append(send, '\n')
		send = append(send, []byte(e.line(1))...)
	}

	if len(e.current) > 2 {
		send = append(send, '\n')
		send = append(send, []byte(e.line(2))...)
	}

	if len(e.current) > 3 {
		send = append(send, '\n')
		send = append(send, []byte(e.line(3))...)
	}

	return e.olcd.Update(string(send))
}
