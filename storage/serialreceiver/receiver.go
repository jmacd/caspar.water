package serialreceiver

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
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
	mode := &serial.Mode{
		BaudRate: cfg.Baud,
		DataBits: 8,
		StopBits: serial.OneStopBit,
		Parity:   serial.NoParity,
	}
	port, err := serial.Open(cfg.Device, mode)
	if err != nil {
		return nil, err
	}
	if err := port.SetReadTimeout(readTimeout); err != nil {
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
	if err := r.port.Break(100 * time.Millisecond); err != nil {
		return err
	}
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
		default:
			data, err := r.read(rdr)
			if err != nil {
				r.settings.TelemetrySettings.Logger.Error(
					"serial read",
					zap.String("device", r.cfg.Device),
					zap.Error(err),
				)
			}

			if data.LogRecordCount() == 0 {
				continue
			}

			if err := r.nextConsumer.ConsumeLogs(ctx, data); err != nil {
				r.settings.TelemetrySettings.Logger.Error(
					"serial consume",
					zap.Error(err),
				)
			}
		}
	}
}

func (r *serialReceiver) read(rdr *bufio.Reader) (plog.Logs, error) {
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
			rec.Body().SetStr(strings.TrimRight(str, "\r\n"))
		}
		if err != nil {
			return ld, err
		}
	}
	return ld, nil
}

// Shutdown stops.
func (r *serialReceiver) Shutdown(ctx context.Context) error {
	if r.cancel == nil {
		return fmt.Errorf("not started")
	}
	r.cancel()
	_ = r.port.Close()
	r.wg.Wait()
	return nil
}
