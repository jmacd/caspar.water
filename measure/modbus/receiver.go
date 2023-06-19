package modbus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type modbusReceiver struct {
	cfg          *Config
	settings     receiver.CreateSettings
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	bme          *MODBUS
	nextConsumer consumer.Metrics
}

func newModbusReceiver(cfg *Config, set receiver.CreateSettings, nextConsumer consumer.Metrics) (*modbusReceiver, error) {
	bme, err := New(cfg.Device, int(cfg.I2CAddr), UltraHighAccuracy)
	if err != nil {
		return nil, err
	}
	r := &modbusReceiver{
		cfg:          cfg,
		settings:     set,
		nextConsumer: nextConsumer,
		bme:          bme,
	}
	return r, nil
}

// Start runs.
func (r *modbusReceiver) Start(_ context.Context, host component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.wg.Add(1)
	go r.run(ctx)
	return nil
}

func (r *modbusReceiver) run(ctx context.Context) {
	defer r.wg.Done()
	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			break
		}
		ts := pcommon.NewTimestampFromTime(time.Now())
		data, err := r.bme.Read()
		if err != nil {
			r.settings.TelemetrySettings.Logger.Error("read modbus device", zap.String("device", r.cfg.Device), zap.Error(err))
			continue
		}

		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("modbus")

		// Temperature
		m := sm.Metrics().AppendEmpty()
		m.SetName(r.cfg.Prefix + "_temperature")
		m.SetUnit("C")
		m.SetEmptyGauge()
		pt := m.Gauge().DataPoints().AppendEmpty()
		pt.SetDoubleValue(data.T)
		pt.SetTimestamp(ts)

		// Pressure
		m = sm.Metrics().AppendEmpty()
		m.SetName(r.cfg.Prefix + "_pressure")
		m.SetUnit("Pa")
		m.SetEmptyGauge()
		pt = m.Gauge().DataPoints().AppendEmpty()
		pt.SetDoubleValue(data.P)
		pt.SetTimestamp(ts)

		// Humidity
		m = sm.Metrics().AppendEmpty()
		m.SetName(r.cfg.Prefix + "_humidity")
		m.SetUnit("rh%")
		m.SetEmptyGauge()
		pt = m.Gauge().DataPoints().AppendEmpty()
		pt.SetDoubleValue(data.H)
		pt.SetTimestamp(ts)

		if err := r.nextConsumer.ConsumeMetrics(context.Background(), md); err != nil {
			r.settings.TelemetrySettings.Logger.Error("write metrics", zap.Error(err))
		}
	}
}

// Shutdown stops.
func (r *modbusReceiver) Shutdown(ctx context.Context) error {
	if r.cancel == nil {
		return fmt.Errorf("not started")
	}
	r.cancel()
	r.wg.Wait()
	return nil
}
