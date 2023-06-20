package modbus

import (
	"fmt"
	"time"

	"github.com/simonvetter/modbus"
	"go.uber.org/zap"
)

type modbusClient struct {
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

func New(url string, attrs []Attribute, metrics []Metric, logger *zap.Logger) (*modbusClient, error) {
	var client *modbus.ModbusClient
	var err error

	client, err = modbus.NewClient(&modbus.ClientConfiguration{
		URL:      url,
		Speed:    19200,              // default
		DataBits: 8,                  // default, optional
		Parity:   modbus.PARITY_EVEN, // default, optional
		StopBits: 1,                  // default if no parity, optional
		Timeout:  300 * time.Millisecond,
	})
	if err != nil {
		return nil, fmt.Errorf("new client: %w", err)
	}
	mc := &modbusClient{
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

func (c *modbusClient) Read() (Measurements, error) {
	var m Measurements

	for _, attr := range c.attrs {
		for {
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
		for {
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
