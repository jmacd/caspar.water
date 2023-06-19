package openlcd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type indexAndAbbrev struct {
	index  int
	abbrev string
}

type openLCDExporter struct {
	display  *os.File
	config   *Config
	defs     []pmetric.Metric
	current  []interface{} // a point type
	name2iaa map[string]indexAndAbbrev
	ablen    int
	olcd     *OpenLCD
	ch       chan struct{}
}

func newOpenLCDExporter(cfg *Config, set exporter.CreateSettings) (*openLCDExporter, error) {
	f, err := os.OpenFile(cfg.Device, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open device: %s: %w", cfg.Device, err)
	}
	n2iaa := map[string]indexAndAbbrev{}
	cur := make([]interface{}, len(cfg.Show))
	defs := make([]pmetric.Metric, len(cfg.Show))
	ablen := 0
	for idx, mc := range cfg.Show {
		n2iaa[mc.Metric] = indexAndAbbrev{
			index:  idx,
			abbrev: mc.Abbrev,
		}
		if len(mc.Abbrev) > ablen {
			ablen = len(mc.Abbrev)
		}
		defs[idx] = pmetric.NewMetric()
	}

	olcd, err := New(cfg.Device, int(cfg.I2CAddr))
	if err != nil {
		return nil, err
	}

	exp := &openLCDExporter{
		display:  f,
		config:   cfg,
		current:  cur,
		defs:     defs,
		name2iaa: n2iaa,
		ablen:    ablen + 1,
		olcd:     olcd,
		ch:       make(chan struct{}, 1),
	}
	go exp.export()
	return exp, err
}

func (e *openLCDExporter) pushMetrics(_ context.Context, md pmetric.Metrics) error {
	for ri := 0; ri < md.ResourceMetrics().Len(); ri++ {
		rm := md.ResourceMetrics().At(ri)

		for si := 0; si < rm.ScopeMetrics().Len(); si++ {
			sm := rm.ScopeMetrics().At(si)

			for mi := 0; mi < sm.Metrics().Len(); mi++ {
				m := sm.Metrics().At(mi)

				iaa, ok := e.name2iaa[m.Name()]
				if !ok {
					continue
				}
				switch m.Type() {
				case pmetric.MetricTypeEmpty:
					e.current[iaa.index] = nil

				case pmetric.MetricTypeGauge:
					dp := m.Gauge().DataPoints()
					e.current[iaa.index] = dp.At(dp.Len() - 1)
					e.defs[iaa.index].SetName(iaa.abbrev)
					e.defs[iaa.index].SetUnit(m.Unit())
					e.defs[iaa.index].SetDescription(m.Description())

				case pmetric.MetricTypeSum,
					pmetric.MetricTypeHistogram,
					pmetric.MetricTypeExponentialHistogram,
					pmetric.MetricTypeSummary:
					panic("unimplemented")
				}

			}
		}
	}

	e.ch <- struct{}{}
	return nil
}

func (e *openLCDExporter) line(n int) string {
	value := e.current[n]

	kstr := e.defs[n].Name() + strings.Repeat(" ", e.ablen-len(e.defs[n].Name()))

	vstr := "<unset>"
	if value != nil {
		switch t := value.(type) {
		case pmetric.NumberDataPoint:
			switch t.ValueType() {
			case pmetric.NumberDataPointValueTypeDouble:
				vstr = fmt.Sprintf("%.2f", t.DoubleValue())
			case pmetric.NumberDataPointValueTypeInt:
				vstr = fmt.Sprint(t.IntValue())
			case pmetric.NumberDataPointValueTypeEmpty:
			}
		default:
			panic("unhandled")
		}
	}
	ustr := ""
	if e.defs[n].Unit() != "" {
		switch e.defs[n].Unit() {
		case "C":
			ustr = "\xDFC"
		default:
			ustr = e.defs[n].Unit()
		}
	}

	line := fmt.Sprint(kstr, vstr, ustr)
	return line
}

func (e *openLCDExporter) export() error {
	for {
		<-e.ch

		e.olcd.Clear()

		for i := 0; i < 4; i++ {
			if len(e.current) > i {
				e.olcd.Update(e.line(i))
			} else {
				e.olcd.Update("")
			}
		}
	}
}
