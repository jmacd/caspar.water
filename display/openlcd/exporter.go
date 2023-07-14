package openlcd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type indexAndAbbrev struct {
	index  int
	abbrev string
}

type openLCDExporter struct {
	lock     sync.Mutex
	display  *os.File
	config   *Config
	defs     []pmetric.Metric
	current  []interface{} // a point type
	name2iaa map[string]indexAndAbbrev
	ablen    int
	olcd     *OpenLCD
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
		defs[idx].SetName(mc.Abbrev)
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
	}
	_ = exp.olcd.Clear()
	_ = exp.olcd.On()
	go exp.export()
	return exp, err
}

func (e *openLCDExporter) pushMetrics(_ context.Context, md pmetric.Metrics) error {
	e.lock.Lock()
	defer e.lock.Unlock()

	// TODO:
	// 1. Update only changed bytes

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
	return nil
}

func (e *openLCDExporter) line(n, pos int, now time.Time) string {
	value := e.current[n]
	stale := false
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
			}
			if now.Sub(t.Timestamp().AsTime()) > e.config.Staleness {
				stale = true
				vstr = "<stale>"
			}
		default:
			panic("unhandled")
		}
	}
	ustr := ""
	if !stale && e.defs[n].Unit() != "" {
		switch e.defs[n].Unit() {
		case "C":
			ustr = "\xDFC"
		default:
			ustr = e.defs[n].Unit()
		}
	}

	line := fmt.Sprint(kstr, vstr, ustr)
	if pos == 0 {
		const hhmm = "15:04"
		for len(line) < e.config.Cols-len(hhmm) {
			line += " "
		}
		line += now.Local().Format(hhmm)
	}
	for len(line) < e.config.Cols {
		line += " "
	}
	return line
}

func (e *openLCDExporter) export() {
	seq := 0
	start := time.Now()

	for e.config.RunFor == 0 || time.Since(start) < e.config.RunFor {
		e.draw(seq)
		seq++
		time.Sleep(e.config.Refresh)
	}

	_ = e.olcd.Off()
}

func (e *openLCDExporter) draw(seq int) {
	e.lock.Lock()
	defer e.lock.Unlock()

	e.olcd.Home()
	now := time.Now()

	for x := 0; x < e.config.Rows; x++ {
		if len(e.current) > x {
			if len(e.current) <= e.config.Rows {
				e.olcd.Update(e.line(x, x, now))
			} else {
				e.olcd.Update(e.line((x+seq)%len(e.current), x, now))
			}
		} else {
			e.olcd.Update("")
		}
	}
}
