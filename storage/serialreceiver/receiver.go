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
	logger       *zap.Logger
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	nextConsumer consumer.Logs
}

// newSerialReceiver just creates the OpenTelemetry receiver services. It is the caller's
// responsibility to invoke the respective Start*Reception methods as well
// as the various Stop*Reception methods to end it.
func newSerialReceiver(cfg *Config, set receiver.Settings, nextConsumer consumer.Logs) (*serialReceiver, error) {
	return &serialReceiver{
		cfg:          cfg,
		logger:       set.Logger.With(zap.String("device", cfg.Device)),
		nextConsumer: nextConsumer,
	}, nil
}

// Start runs.
func (r *serialReceiver) Start(_ context.Context, host component.Host) error {
	// Note: do not use start context, it is canceled when start completes.
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.wg.Add(1)
	go r.run(ctx)
	return nil
}

func (r *serialReceiver) run(ctx context.Context) {
	defer r.wg.Done()

	// Loop to re-open serial port
	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(time.Second)
		}
		mode := &serial.Mode{
			BaudRate: r.cfg.Baud,
			DataBits: 8,
			StopBits: serial.OneStopBit,
			Parity:   serial.NoParity,
		}
		port, err := serial.Open(r.cfg.Device, mode)
		if err != nil {
			r.logger.Error("serial open", zap.Error(err))
			continue
		}
		r.runOpened(ctx, port, bufio.NewReader(port))
	}
}

func (r *serialReceiver) runOpened(ctx context.Context, port serial.Port, rdr *bufio.Reader) {
	defer port.Close()
	if err := port.SetReadTimeout(readTimeout); err != nil {
		r.logger.Error("serial setup", zap.Error(err))
		return
	}

	if err := port.Break(100 * time.Millisecond); err != nil {
		r.logger.Error("serial break", zap.Error(err))
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
			data, err := r.read(rdr)
			if err != nil {
				r.logger.Error(
					"serial read",
					zap.Error(err),
				)
			}

			if data.LogRecordCount() == 0 {
				continue
			}

			if err := r.nextConsumer.ConsumeLogs(ctx, data); err != nil {
				r.logger.Error(
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
	r.wg.Wait()
	return nil
}
