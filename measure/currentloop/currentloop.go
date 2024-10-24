package currentloop

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// loopReceiver is the type that exposes Trace and Metrics reception.
type loopReceiver struct {
	cfg          *Config
	settings     receiver.Settings
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	nextConsumer consumer.Metrics
}

// newLoopReceiver just creates the OpenTelemetry receiver services. It is the caller's
// responsibility to invoke the respective Start*Reception methods as well
// as the various Stop*Reception methods to end it.
func newLoopReceiver(cfg *Config, set receiver.Settings, nextConsumer consumer.Metrics) (*loopReceiver, error) {
	r := &loopReceiver{
		cfg:          cfg,
		settings:     set,
		nextConsumer: nextConsumer,
	}
	return r, nil
}

// Start runs.
func (r *loopReceiver) Start(_ context.Context, host component.Host) error {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.wg.Add(1)
	go r.run(ctx)
	return nil
}

func unitScale[N uint64 | float64](x, min, max N) float64 {
	return float64(x-min) / float64(max-min)
}

func (r *loopReceiver) run(ctx context.Context) {
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
		data, err := os.ReadFile(r.cfg.Device)
		if err != nil {
			r.settings.TelemetrySettings.Logger.Error("read currentloop device", zap.String("device", r.cfg.Device), zap.Error(err))
			continue
		}

		rawValue, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 32)
		if err != nil {
			r.settings.TelemetrySettings.Logger.Error("parse device data", zap.String("device", r.cfg.Device), zap.String("value", string(data)), zap.Error(err))
			continue
		}

		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName(r.cfg.Name)
		m.SetUnit(r.cfg.Unit)
		m.SetEmptyGauge()

		volts := float64(rawValue) * r.cfg.Multiply / r.cfg.Divide
		current := volts / r.cfg.Ohms
		scaled := unitScale(current, minCurrent, maxCurrent)
		shifted := scaled*(r.cfg.Max-r.cfg.Min) + r.cfg.Min

		pt := m.Gauge().DataPoints().AppendEmpty()
		pt.SetDoubleValue(shifted)
		pt.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

		if err := r.nextConsumer.ConsumeMetrics(context.Background(), md); err != nil {
			r.settings.TelemetrySettings.Logger.Error("write metrics", zap.Error(err))
		}
	}
}

// Shutdown stops.
func (r *loopReceiver) Shutdown(ctx context.Context) error {
	if r.cancel == nil {
		return fmt.Errorf("not started")
	}
	r.cancel()
	r.wg.Wait()
	return nil
}
