package modbus

import (
	"context"
	"fmt"
	"time"

	"github.com/simonvetter/modbus"
	"go.uber.org/zap"
)

type modbusClient struct {
	cfg          *Config
	client       *modbus.ModbusClient
	clientConfig *modbus.ClientConfiguration
	attrs        []Attribute
	metrics      []Metric
	logger       *zap.Logger
}

type Pair[T any] struct {
	Field T
	Value interface{}
}

// Measurements contains compensated measurement values.
type Measurements struct {
	A []Pair[Attribute]
	M []Pair[Metric]
}

func New(cfg *Config, attrs []Attribute, metrics []Metric, logger *zap.Logger) (*modbusClient, error) {
	parity, err := parityFromString(cfg.Parity)
	if err != nil {
		return nil, err
	}

	clientConfig := &modbus.ClientConfiguration{
		URL:      cfg.URL,
		Speed:    cfg.Baud,
		DataBits: cfg.DataBits,
		Parity:   parity,
		StopBits: cfg.StopBits,
		Timeout:  cfg.Timeout,
	}

	mc := &modbusClient{
		cfg:          cfg,
		clientConfig: clientConfig,
		attrs:        attrs,
		metrics:      metrics,
		logger:       logger,
	}

	// If not reconnecting per-read, open a persistent connection
	if !cfg.Reconnect {
		client, err := modbus.NewClient(clientConfig)
		if err != nil {
			return nil, fmt.Errorf("new client: %w", err)
		}
		if err = client.Open(); err != nil {
			return nil, fmt.Errorf("open: %w", err)
		}
		mc.client = client
	}

	return mc, nil
}

func (c *modbusClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func (c *modbusClient) Read(ctx context.Context) (Measurements, error) {
	var m Measurements

	// Calculate timeout based on read delay if set, otherwise use legacy 5s
	delayPerRead := c.cfg.ReadDelay
	if delayPerRead == 0 {
		delayPerRead = 5 * time.Second
	}
	wholeTimeout := (c.cfg.Timeout + delayPerRead) * 2 * time.Duration(len(c.attrs)+len(c.metrics))
	ctx, cancel := context.WithTimeout(ctx, wholeTimeout)
	defer cancel()

	firstRead := true
	for _, attr := range c.attrs {
		for !isDone(ctx) {
			aval, err := c.readField(ctx, attr.Field, &firstRead)

			if err != nil {
				time.Sleep(20 * time.Millisecond)
				if err != modbus.ErrRequestTimedOut {
					c.logger.Info("will retry", zap.Error(err))
				} else {
					c.logger.Debug("request timeout")
				}
				continue
			}
			m.A = append(m.A, Pair[Attribute]{
				Field: attr,
				Value: aval,
			})
			break
		}
	}

	for _, metric := range c.metrics {
		for !isDone(ctx) {
			aval, err := c.readField(ctx, metric.Field, &firstRead)

			if err != nil {
				time.Sleep(20 * time.Millisecond)
				if err != modbus.ErrRequestTimedOut {
					c.logger.Info("will retry", zap.Error(err))
				} else {
					c.logger.Debug("request timeout")
				}
				continue
			}
			m.M = append(m.M, Pair[Metric]{
				Field: metric,
				Value: aval,
			})
			break
		}
	}

	return m, nil
}

// readField reads a single field, handling reconnect and delay as configured
func (c *modbusClient) readField(ctx context.Context, f Field, firstRead *bool) (interface{}, error) {
	// Apply read delay (skip for first read)
	if !*firstRead && c.cfg.ReadDelay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.cfg.ReadDelay):
		}
	} else if !*firstRead {
		// Legacy behavior: 5s delay if no read_delay configured
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	*firstRead = false

	// If reconnect mode, create fresh connection for this read
	client := c.client
	if c.cfg.Reconnect {
		var err error
		client, err = modbus.NewClient(c.clientConfig)
		if err != nil {
			return nil, fmt.Errorf("new client: %w", err)
		}
		if err = client.Open(); err != nil {
			return nil, fmt.Errorf("open: %w", err)
		}
		defer client.Close()
	}

	return c.read(client, f)
}

func (c *modbusClient) read(client *modbus.ModbusClient, f Field) (interface{}, error) {
	var rt modbus.RegType
	switch f.Range {
	case "coil":
		rt = modbus.INPUT_REGISTER
	case "discrete":
		rt = modbus.HOLDING_REGISTER
	case "input":
		rt = modbus.INPUT_REGISTER
	case "holding":
		rt = modbus.HOLDING_REGISTER
	}

	switch f.Type {
	case "uint32":
		return client.ReadUint32(f.Base-1, rt)
	case "uint16":
		return client.ReadRegister(f.Base-1, rt)
	case "float32":
		return client.ReadFloat32(f.Base-1, rt)
	case "bool":
		switch f.Range {
		case "coil":
			return client.ReadCoil(f.Base - 1)
		case "discrete":
			return client.ReadDiscreteInput(f.Base - 1)
		}
	}
	return nil, fmt.Errorf("unknown type/range %q/%q", f.Type, f.Range)
}
