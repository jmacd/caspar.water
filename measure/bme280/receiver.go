package bme280

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

// bme280Receiver is the type that exposes Trace and Metrics reception.
type bme280Receiver struct {
	cfg          *Config
	settings     receiver.Settings
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	bme          *BME280
	nextConsumer consumer.Metrics
}

// newBme280Receiver just creates the OpenTelemetry receiver services. It is the caller's
// responsibility to invoke the respective Start*Reception methods as well
// as the various Stop*Reception methods to end it.
//
// Device-open errors are logged but do NOT fail receiver construction.
// The sensor can be missing, disconnected, or unpowered at startup
// (BME280 over I2C is physically optional hardware); the receiver
// keeps polling and tries to (re-)open the device on every tick.
// Once the sensor comes back, measurements resume automatically.
func newBme280Receiver(cfg *Config, set receiver.Settings, nextConsumer consumer.Metrics) (*bme280Receiver, error) {
	r := &bme280Receiver{
		cfg:          cfg,
		settings:     set,
		nextConsumer: nextConsumer,
	}
	if err := r.openDevice(); err != nil {
		set.TelemetrySettings.Logger.Warn("bme280 device unavailable at startup; will keep retrying",
			zap.String("device", cfg.Device),
			zap.Int("i2c_addr", int(cfg.I2CAddr)),
			zap.Error(err))
	}
	return r, nil
}

// openDevice attempts to (re-)initialize the underlying BME280 handle.
// Safe to call repeatedly; on success r.bme is non-nil, on failure r.bme
// is left nil and the caller should log/skip.
func (r *bme280Receiver) openDevice() error {
	bme, err := New(r.cfg.Device, int(r.cfg.I2CAddr), UltraHighAccuracy)
	if err != nil {
		r.bme = nil
		return err
	}
	r.bme = bme
	return nil
}

// Start runs.
func (r *bme280Receiver) Start(_ context.Context, host component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.wg.Add(1)
	go r.run(ctx)
	return nil
}

func (r *bme280Receiver) run(ctx context.Context) {
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

func (r *bme280Receiver) measure() {
	if r.bme == nil {
		if err := r.openDevice(); err != nil {
			r.settings.TelemetrySettings.Logger.Debug("bme280 still unavailable, skipping measurement",
				zap.String("device", r.cfg.Device),
				zap.Error(err))
			return
		}
		r.settings.TelemetrySettings.Logger.Info("bme280 device opened",
			zap.String("device", r.cfg.Device))
	}

	ts := pcommon.NewTimestampFromTime(time.Now())
	data, err := r.bme.Read()
	if err != nil {
		r.settings.TelemetrySettings.Logger.Error("read bme280 device", zap.String("device", r.cfg.Device), zap.Error(err))
		// Drop the handle so the next tick re-opens; the bus may have hot-unplugged.
		_ = r.bme.Close()
		r.bme = nil
		return
	}

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("bme280")

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
	m.SetEmptyGauge()
	pt = m.Gauge().DataPoints().AppendEmpty()
	pt.SetDoubleValue(data.H)
	pt.SetTimestamp(ts)

	if err := r.nextConsumer.ConsumeMetrics(context.Background(), md); err != nil {
		r.settings.TelemetrySettings.Logger.Error("write metrics", zap.Error(err))
	}
}

// Shutdown stops.
func (r *bme280Receiver) Shutdown(ctx context.Context) error {
	if r.cancel == nil {
		return fmt.Errorf("not started")
	}
	r.cancel()
	r.wg.Wait()
	return nil
}
