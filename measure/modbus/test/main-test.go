package main

import (
	"log"

	"github.com/jmacd/caspar.water/measure/modbus"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment()
	dev, err := modbus.New("rtu:///dev/ttyUSB0",
		[]modbus.Attribute{
			{
				Field: modbus.Field{
					Name:  "serial_number",
					Base:  9002,
					Type:  "uint32",
					Range: "holding",
				},
			},
		},
		[]modbus.Metric{
			{
				Field: modbus.Field{
					Name:  "temperature",
					Base:  38,
					Type:  "float32",
					Range: "holding",
				},
				Unit: "C",
			},
			{
				Field: modbus.Field{
					Name:  "pressure",
					Base:  46,
					Type:  "float32",
					Range: "holding",
				},
				Unit: "psi",
			},
		},
		logger,
	)

	if err != nil {
		log.Fatal("open: %w", err)
	}

	comp, err := dev.Read()
	if err != nil {
		log.Fatal("read: %w", err)
	}

	for _, a := range comp.A {
		log.Println("A=", a)
	}
	for _, m := range comp.M {
		log.Println("M=", m)
	}
}
