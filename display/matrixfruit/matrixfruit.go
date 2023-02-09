// https://learn.adafruit.com/usb-plus-serial-backpack/command-reference
package matrixfruit

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

type matrixfruitExporter struct {
	display *os.File
}

func newMatrixfruitExporter(cfg *Config) (*matrixfruitExporter, error) {
	f, err := os.OpenFile(cfg.Device, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open device")
	}
	return &matrixfruitExporter{
		display: f,
	}, err
}

func (m *matrixfruitExporter) pushMetrics(_ context.Context, md pmetric.Metrics) error {
	return m.export()
}

func (m *matrixfruitExporter) export() error {
	_, err := m.display.Write([]byte{
		// For a 2x16
		0xFE,
		0xD1,
		16,
		2,

		// set background
		0xFE,
		0xD0,
		0x33,
		0x55,
		0xFF,

		// clear screen
		0xFE,
		0x58,

		// go home
		0xFE,
		0x48,

		// autoscroll
		0xFE,
		0x51,

		// setpos
		0xFE,
		0x47,
		1, 1,

		'h',
		'e',
		'l',
		'l',
		'o',
		'5',
		'6',
		'7',
		'8',
		'9',
		'0',
		'1',
		'2',
		'3',
		'4',
		'5',

		// setpos
		0xFE,
		0x47,
		1, 2,

		'm',
		'o',
		'r',
		'e',
		'4',
		'5',
		'6',
		'7',
		'8',
		'9',
		'0',
		'1',
		'2',
		'3',
		'4',
	})
	return err
}
