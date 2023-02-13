// https://learn.adafruit.com/usb-plus-serial-backpack/command-reference
package matrixfruit

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

type matrixfruitExporter struct {
	display  *os.File
	config   *Config
	defs     []pmetric.Metric
	current  []interface{} // a point type
	name2idx map[string]int
}

func newMatrixfruitExporter(cfg *Config) (*matrixfruitExporter, error) {

	f, err := os.OpenFile(cfg.Device, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open device")
	}
	n2i := map[string]int{}
	cur := make([]interface{}, len(cfg.Metrics))
	defs := make([]pmetric.Metric, len(cfg.Metrics))
	for idx, mc := range cfg.Metrics {
		n2i[mc.Name] = idx
		defs[idx] = pmetric.NewMetric()
	}
	return &matrixfruitExporter{
		display:  f,
		config:   cfg,
		current:  cur,
		defs:     defs,
		name2idx: n2i,
	}, err
}

func (mfe *matrixfruitExporter) pushMetrics(_ context.Context, md pmetric.Metrics) error {
	for ri := 0; ri < md.ResourceMetrics().Len(); ri++ {
		rm := md.ResourceMetrics().At(ri)

		for si := 0; si < rm.ScopeMetrics().Len(); si++ {
			sm := rm.ScopeMetrics().At(si)

			for mi := 0; mi < sm.Metrics().Len(); mi++ {
				m := sm.Metrics().At(mi)

				idx, ok := mfe.name2idx[m.Name()]
				if !ok {
					continue
				}
				switch m.Type() {
				case pmetric.MetricTypeEmpty:
					mfe.current[idx] = nil

				case pmetric.MetricTypeGauge:
					dp := m.Gauge().DataPoints()
					mfe.current[idx] = dp.At(dp.Len() - 1)
					mfe.defs[idx].SetUnit(m.Unit())
					mfe.defs[idx].SetDescription(m.Description())

				case pmetric.MetricTypeSum,
					pmetric.MetricTypeHistogram,
					pmetric.MetricTypeExponentialHistogram,
					pmetric.MetricTypeSummary:
					panic("unimplemented")
				}
			}
		}
	}

	return mfe.export()
}

func (mfe *matrixfruitExporter) line(n int) string {
	value := mfe.current[n]

	vstr := "<unset>"
	if value != nil {
		switch t := value.(type) {
		case pmetric.NumberDataPoint:
			switch t.ValueType() {
			case pmetric.NumberDataPointValueTypeDouble:
				vstr = fmt.Sprint(t.DoubleValue())
			case pmetric.NumberDataPointValueTypeInt:
				vstr = fmt.Sprint(t.DoubleValue())
			case pmetric.NumberDataPointValueTypeEmpty:
			}
		default:
			panic("unhandled")
		}
	}
	ustr := "<undef>"
	if mfe.defs[n].Unit() != "" {
		ustr = mfe.defs[n].Unit()
	}

	line := fmt.Sprint(vstr, " ", ustr)
	return line
}

func (mfe *matrixfruitExporter) export() error {

	var send = []byte{
		// For a 2x16
		0xFE,
		0xD1,
		16,
		2,

		// set background
		0xFE,
		0xD0,
		0x33,
		0x55,
		0xFF,

		// clear screen
		0xFE,
		0x58,

		// go home
		0xFE,
		0x48,

		// autoscroll
		0xFE,
		0x51,

		// setpos
		0xFE,
		0x47,
		1, 1,
	}

	send = append(send, []byte(mfe.line(0))...)

	if len(mfe.current) > 1 {
		send = append(send,
			// setpos
			0xFE,
			0x47,
			1, 2,
		)

		send = append(send, []byte(mfe.line(1))...)
	}

	_, err := mfe.display.Write(send)
	return err
}
