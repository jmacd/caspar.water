package serialreceiver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

// serialReceiver is the type that exposes Trace and Metrics reception.
type serialReceiver struct {
	cfg          *Config
	settings     receiver.CreateSettings
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	serial       *Serial
	nextConsumer consumer.Metrics
}

// newSerialReceiver just creates the OpenTelemetry receiver services. It is the caller's
// responsibility to invoke the respective Start*Reception methods as well
// as the various Stop*Reception methods to end it.
func newSerialReceiver(cfg *Config, set receiver.CreateSettings, nextConsumer consumer.Metrics) (*serialReceiver, error) {
	serial, err := New(cfg.Device)
	if err != nil {
		return nil, err
	}
	r := &serialReceiver{
		cfg:          cfg,
		settings:     set,
		nextConsumer: nextConsumer,
		serial:       serial,
	}
	return r, nil
}

// Start runs.
func (r *serialReceiver) Start(_ context.Context, host component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.wg.Add(1)
	go r.run(ctx)
	return nil
}

func (r *serialReceiver) run(ctx context.Context) {
	defer r.wg.Done()

	// Send an initial masurement immediately.
	r.measure()

	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			break
		}
		r.measure()
	}
}

func (r *serialReceiver) measure() {
	// ts := pcommon.NewTimestampFromTime(time.Now())
	// data, err := r.bme.Read()
	// if err != nil {
	// 	r.settings.TelemetrySettings.Logger.Error("read serial device", zap.String("device", r.cfg.Device), zap.Error(err))
	// 	return
	// }

	// md := pmetric.NewMetrics()
	// rm := md.ResourceMetrics().AppendEmpty()
	// sm := rm.ScopeMetrics().AppendEmpty()
	// sm.Scope().SetName("serial")

	// // Temperature
	// m := sm.Metrics().AppendEmpty()
	// m.SetName(r.cfg.Prefix + "_temperature")
	// m.SetUnit("C")
	// m.SetEmptyGauge()
	// pt := m.Gauge().DataPoints().AppendEmpty()
	// pt.SetDoubleValue(data.T)
	// pt.SetTimestamp(ts)

	// // Pressure
	// m = sm.Metrics().AppendEmpty()
	// m.SetName(r.cfg.Prefix + "_pressure")
	// m.SetUnit("Pa")
	// m.SetEmptyGauge()
	// pt = m.Gauge().DataPoints().AppendEmpty()
	// pt.SetDoubleValue(data.P)
	// pt.SetTimestamp(ts)

	// // Humidity
	// m = sm.Metrics().AppendEmpty()
	// m.SetName(r.cfg.Prefix + "_humidity")
	// m.SetEmptyGauge()
	// pt = m.Gauge().DataPoints().AppendEmpty()
	// pt.SetDoubleValue(data.H)
	// pt.SetTimestamp(ts)

	// if err := r.nextConsumer.ConsumeMetrics(context.Background(), md); err != nil {
	// 	r.settings.TelemetrySettings.Logger.Error("write metrics", zap.Error(err))
	// }
}

// Shutdown stops.
func (r *serialReceiver) Shutdown(ctx context.Context) error {
	if r.cancel == nil {
		return fmt.Errorf("not started")
	}
	r.cancel()
	r.wg.Wait()
	return nil
}
