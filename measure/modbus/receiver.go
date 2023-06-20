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
	client       *modbusClient
	nextConsumer consumer.Metrics
}

func newModbusReceiver(cfg *Config, set receiver.CreateSettings, nextConsumer consumer.Metrics) (*modbusReceiver, error) {
	client, err := New(cfg.URL, cfg.Attributes, cfg.Metrics, set.Logger)
	if err != nil {
		return nil, err
	}
	r := &modbusReceiver{
		cfg:          cfg,
		settings:     set,
		nextConsumer: nextConsumer,
		client:       client,
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
		data, err := r.client.Read()
		if err != nil {
			r.settings.TelemetrySettings.Logger.Error("read modbus device", zap.String("device", r.cfg.URL), zap.Error(err))
			continue
		}

		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("modbus")

		attrs := sm.Scope().Attributes()

		for _, ma := range data.A {
			switch t := ma.value.(type) {
			case uint32:
				attrs.PutInt(r.cfg.Prefix+"_"+ma.field.Name, int64(t))
			case float32:
				attrs.PutDouble(r.cfg.Prefix+"_"+ma.field.Name, float64(t))
			case string:
				attrs.PutStr(r.cfg.Prefix+"_"+ma.field.Name, t)
			default:
				r.settings.TelemetrySettings.Logger.Error("unhandled attribute type")
			}
		}

		for _, ma := range data.M {
			m := sm.Metrics().AppendEmpty()
			m.SetName(r.cfg.Prefix + "_" + ma.field.Name)
			m.SetUnit(ma.field.Unit)
			m.SetEmptyGauge()
			pt := m.Gauge().DataPoints().AppendEmpty()
			pt.SetTimestamp(ts)

			switch t := ma.value.(type) {
			case uint32:
				pt.SetIntValue(int64(t))
			case float32:
				pt.SetDoubleValue(float64(t))
			default:
				r.settings.TelemetrySettings.Logger.Error("unhandled metric type")
			}
		}

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
