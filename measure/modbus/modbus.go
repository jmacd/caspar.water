package modbus

import (
	"context"
	"fmt"
	"time"

	"github.com/simonvetter/modbus"
	"go.uber.org/zap"
)

type modbusClient struct {
	cfg     *Config
	client  *modbus.ModbusClient
	attrs   []Attribute
	metrics []Metric
	logger  *zap.Logger
}

type pair[T any] struct {
	field T
	value interface{}
}

// Measurements contains compensated measurement values.
type Measurements struct {
	A []pair[Attribute]
	M []pair[Metric]
}

func New(cfg *Config, attrs []Attribute, metrics []Metric, logger *zap.Logger) (*modbusClient, error) {
	var client *modbus.ModbusClient
	var err error

	parity, err := parityFromString(cfg.Parity)
	if err != nil {
		return nil, err
	}
	client, err = modbus.NewClient(&modbus.ClientConfiguration{
		URL:      cfg.URL,
		Speed:    cfg.Baud,
		DataBits: cfg.DataBits,
		Parity:   parity,
		StopBits: cfg.StopBits,
		Timeout:  cfg.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("new client: %w", err)
	}
	mc := &modbusClient{
		cfg:     cfg,
		client:  client,
		attrs:   attrs,
		metrics: metrics,
		logger:  logger,
	}

	err = client.Open()
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	return mc, nil
}

func (c *modbusClient) Close() error {
	return c.client.Close()
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

	wholeTimeout := c.cfg.Timeout * 2 * time.Duration(len(c.attrs)+len(c.metrics))
	ctx, cancel := context.WithTimeout(ctx, wholeTimeout)
	defer cancel()

	for _, attr := range c.attrs {
		for !isDone(ctx) {
			aval, err := c.read(attr.Field)

			if err != nil {
				time.Sleep(20 * time.Millisecond)
				if err != modbus.ErrRequestTimedOut {
					c.logger.Info("will retry", zap.Error(err))
				} else {
					c.logger.Debug("request timeout")
				}
				continue
			}
			m.A = append(m.A, pair[Attribute]{
				field: attr,
				value: aval,
			})
			break
		}
	}

	for _, metric := range c.metrics {
		for !isDone(ctx) {
			aval, err := c.read(metric.Field)

			if err != nil {
				time.Sleep(20 * time.Millisecond)
				if err != modbus.ErrRequestTimedOut {
					c.logger.Info("will retry", zap.Error(err))
				} else {
					c.logger.Debug("request timeout")
				}
				continue
			}
			m.M = append(m.M, pair[Metric]{
				field: metric,
				value: aval,
			})
			break
		}
	}

	return m, nil
}

func (c *modbusClient) read(f Field) (interface{}, error) {
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
		return c.client.ReadUint32(f.Base-1, rt)
	case "float32":
		return c.client.ReadFloat32(f.Base-1, rt)
	case "bool":
		switch f.Range {
		case "coil":
			return c.client.ReadCoil(f.Base - 1)
		case "discrete":
			return c.client.ReadDiscreteInput(f.Base - 1)
		}
	}
	return nil, fmt.Errorf("unknown type/range %q/%q", f.Type, f.Range)
}
