package currentloop

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
)

// loopReceiver is the type that exposes Trace and Metrics reception.
type loopReceiver struct {
	cfg      *Config
	settings receiver.CreateSettings
}

// newLoopReceiver just creates the OpenTelemetry receiver services. It is the caller's
// responsibility to invoke the respective Start*Reception methods as well
// as the various Stop*Reception methods to end it.
func newLoopReceiver(cfg *Config, set receiver.CreateSettings) (*loopReceiver, error) {
	r := &loopReceiver{
		cfg:      cfg,
		settings: set,
	}
	return r, nil
}

// Start runs.
func (r *loopReceiver) Start(_ context.Context, host component.Host) error {
	return nil
}

// Shutdown stops.
func (r *loopReceiver) Shutdown(ctx context.Context) error {
	return nil
}
