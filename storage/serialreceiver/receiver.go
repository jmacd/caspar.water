package serialreceiver

import (
	"bufio"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/goburrow/serial"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

const readTimeout = 5 * time.Second

// serialReceiver is the type that exposes Trace and Metrics reception.
type serialReceiver struct {
	cfg          *Config
	settings     receiver.CreateSettings
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	port         serial.Port
	nextConsumer consumer.Logs
}

// newSerialReceiver just creates the OpenTelemetry receiver services. It is the caller's
// responsibility to invoke the respective Start*Reception methods as well
// as the various Stop*Reception methods to end it.
func newSerialReceiver(cfg *Config, set receiver.CreateSettings, nextConsumer consumer.Logs) (*serialReceiver, error) {
	scfg := serial.Config{
		Address:  cfg.Device,
		BaudRate: cfg.Baud,
		DataBits: 8,
		StopBits: 1,
		Parity:   "E",
		Timeout:  readTimeout,
	}
	port, err := serial.Open(&scfg)
	if err != nil {
		return nil, err
	}
	r := &serialReceiver{
		cfg:          cfg,
		settings:     set,
		nextConsumer: nextConsumer,
		port:         port,
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

	rdr := bufio.NewReader(r.port)

	for {
		select {
		case <-ctx.Done():
			return
		}
		ld := plog.NewLogs()
		rm := ld.ResourceLogs().AppendEmpty()
		sm := rm.ScopeLogs().AppendEmpty()
		sm.Scope().SetName("serial")

		start := time.Now()

		for time.Since(start) < readTimeout {
			str, err := rdr.ReadString('\n')
			if len(str) != 0 {
				ts := pcommon.NewTimestampFromTime(time.Now())

				rec := sm.LogRecords().AppendEmpty()
				rec.SetTimestamp(ts)
				rec.Body().SetStr(str)
			}
			if err != nil {
				r.settings.TelemetrySettings.Logger.Error("read serial device",
					zap.String("device", r.cfg.Device), zap.Error(err))
				break
			}

		}
		if ld.LogRecordCount() == 0 {
			continue
		}
		if err := r.nextConsumer.ConsumeLogs(context.Background(), ld); err != nil {
			r.settings.TelemetrySettings.Logger.Error("write logs", zap.Error(err))
		}
	}
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
