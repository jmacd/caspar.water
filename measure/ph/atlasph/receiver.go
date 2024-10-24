package atlasph

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jmacd/caspar.water/measure/ph/atlasph/internal/device"
	"github.com/jmacd/caspar.water/measure/ph/atlasph/internal/ezo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// phReceiver is the type that exposes Trace and Metrics reception.
type phReceiver struct {
	cfg          *Config
	settings     receiver.Settings
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	ph           *ezo.Ph
	nextConsumer consumer.Metrics
}

// newPhReceiver just creates the OpenTelemetry receiver services. It is the caller's
// responsibility to invoke the respective Start*Reception methods as well
// as the various Stop*Reception methods to end it.
func newPhReceiver(cfg *Config, set receiver.Settings, nextConsumer consumer.Metrics) (*phReceiver, error) {
	dev, err := device.New(cfg.Device, int(cfg.I2CAddr))
	if err != nil {
		return nil, err
	}
	r := &phReceiver{
		cfg:          cfg,
		settings:     set,
		nextConsumer: nextConsumer,
		ph:           ezo.New(dev),
	}
	if status, err := r.ph.Status(); err != nil {
		return r, err
	} else {
		set.TelemetrySettings.Logger.Info("ph status", zap.Float64("vcc", status.Vcc), zap.String("restart", status.Restart))
	}
	return r, nil
}

// Start runs.
func (r *phReceiver) Start(_ context.Context, host component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.wg.Add(1)
	go r.run(ctx)
	return nil
}

func (r *phReceiver) run(ctx context.Context) {
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

func (r *phReceiver) measure() {
	ts := pcommon.NewTimestampFromTime(time.Now())
	ph, err := r.ph.ReadPh(r.cfg.ReferenceTempC)
	if err != nil {
		r.settings.TelemetrySettings.Logger.Error("read ph device", zap.String("device", r.cfg.Device), zap.Error(err))
		return
	}

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("atlas_ph")

	// pH
	m := sm.Metrics().AppendEmpty()
	m.SetName(r.cfg.Prefix + "_ph")
	m.SetUnit("")
	m.SetEmptyGauge()
	pt := m.Gauge().DataPoints().AppendEmpty()
	pt.SetDoubleValue(ph)
	pt.SetTimestamp(ts)

	if err := r.nextConsumer.ConsumeMetrics(context.Background(), md); err != nil {
		r.settings.TelemetrySettings.Logger.Error("write metrics", zap.Error(err))
	}
}

// Shutdown stops.
func (r *phReceiver) Shutdown(ctx context.Context) error {
	if r.cancel == nil {
		return fmt.Errorf("not started")
	}
	r.cancel()
	r.wg.Wait()
	return nil
}
